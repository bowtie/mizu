// Package auth implements single-password authentication for Mizu:
// a bcrypt-hashed admin password in app_settings plus DB-backed
// session tokens delivered via an HttpOnly cookie.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// SettingPasswordHash is the app_settings key holding the bcrypt hash.
	SettingPasswordHash = "auth_password_hash"

	// SessionCookieName is the HTTP cookie carrying the session token.
	SessionCookieName = "mizu_session"

	// SessionTTL is how long a session stays valid after login.
	SessionTTL = 30 * 24 * time.Hour

	// MinPasswordLength balances usability against online-guessing risk
	// for a LAN-exposed dashboard.
	MinPasswordLength = 8

	// MaxPasswordBytes is bcrypt's input limit. Longer passwords are
	// rejected up front with a clear message instead of bcrypt's own error.
	MaxPasswordBytes = 72

	// dummyHash is compared against when no password is configured so the
	// login path takes roughly the same time whether or not setup has
	// happened, avoiding a trivially detectable setup-state oracle.
	dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
)

var (
	ErrAlreadyConfigured = errors.New("auth: password already configured")
	ErrInvalidPassword   = errors.New("incorrect password")
	ErrPasswordTooShort  = fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	ErrPasswordTooLong   = fmt.Errorf("password must be at most %d characters (fewer with accented letters or emoji)", MaxPasswordBytes)
	ErrInvalidSession    = errors.New("invalid session")
)

// Store persists the password hash (via app_settings) and sessions.
type Store interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	// SetSettingIfEmpty writes only when the key is unset or empty and
	// reports whether it did, atomically.
	SetSettingIfEmpty(ctx context.Context, key, value string) (bool, error)
	CreateSession(ctx context.Context, tokenHash string, expiresAt time.Time) error
	GetSessionExpiry(ctx context.Context, tokenHash string) (time.Time, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error)
	DeleteAllSessions(ctx context.Context) error
}

// Manager wires password + session operations to a Store. The configured
// state is cached atomically after the first lookup so the request-path
// middleware doesn't hit SQLite on every call; the only code paths that
// flip it (SetPassword / ChangePassword / ClearPassword) update the cache
// themselves.
type Manager struct {
	store      Store
	configured atomic.Bool
	checked    atomic.Bool

	// verified caches the last successful VerifyCached check (a SHA-256
	// over the stored hash and the password) so a metrics scraper sending
	// the password every few seconds doesn't pay bcrypt each time. Keying
	// on the stored hash means a password change invalidates it.
	verifiedMu    sync.Mutex
	verifiedKey   [32]byte
	verifiedUntil time.Time
}

// verifyCacheTTL bounds how long a VerifyCached success is reused.
const verifyCacheTTL = 10 * time.Minute

func NewManager(store Store) *Manager {
	return &Manager{store: store}
}

// HashPassword bcrypt-hashes a plaintext password for storage.
func HashPassword(password string) (string, error) {
	if len(password) < MinPasswordLength {
		return "", ErrPasswordTooShort
	}
	if len(password) > MaxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("auth: hashing password: %w", err)
	}
	return string(hash), nil
}

// IsConfigured reports whether a password has been set. Auth is disabled
// until it is — matching the *arr first-run behavior.
func (m *Manager) IsConfigured(ctx context.Context) (bool, error) {
	if m.checked.Load() {
		return m.configured.Load(), nil
	}
	hash, err := m.store.GetSetting(ctx, SettingPasswordHash)
	if err != nil {
		return false, err
	}
	configured := strings.TrimSpace(hash) != ""
	m.configured.Store(configured)
	m.checked.Store(true)
	return configured, nil
}

// SetPassword stores the initial password hash. It fails if a password
// is already configured; use ChangePassword for rotation. The write is
// conditional on no hash being stored, so two concurrent first-run setups
// can't both win.
func (m *Manager) SetPassword(ctx context.Context, password string) error {
	configured, err := m.IsConfigured(ctx)
	if err != nil {
		return err
	}
	if configured {
		return ErrAlreadyConfigured
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	stored, err := m.store.SetSettingIfEmpty(ctx, SettingPasswordHash, hash)
	if err != nil {
		return err
	}
	m.configured.Store(true)
	m.checked.Store(true)
	if !stored {
		return ErrAlreadyConfigured
	}
	return nil
}

// ChangePassword rotates the password after verifying the current one.
// All existing sessions are revoked so stolen cookies don't survive a
// credential change.
func (m *Manager) ChangePassword(ctx context.Context, current, next string) error {
	if err := m.verify(ctx, current); err != nil {
		return err
	}
	if err := m.storeHash(ctx, next); err != nil {
		return err
	}
	return m.store.DeleteAllSessions(ctx)
}

// ClearPassword removes the password and revokes every session, for
// switching sign-in off. The caller is responsible for authorizing it.
func (m *Manager) ClearPassword(ctx context.Context) error {
	if err := m.store.SetSetting(ctx, SettingPasswordHash, ""); err != nil {
		return err
	}
	m.configured.Store(false)
	m.checked.Store(true)
	return m.store.DeleteAllSessions(ctx)
}

// Verify checks a plaintext password against the stored hash.
func (m *Manager) Verify(ctx context.Context, password string) error {
	return m.verify(ctx, password)
}

// VerifyCached is Verify with a short-lived cache of the last success,
// for callers that present the password on every request (metrics
// scrapers). Failures are never cached and always cost a bcrypt compare.
func (m *Manager) VerifyCached(ctx context.Context, password string) error {
	hash, err := m.store.GetSetting(ctx, SettingPasswordHash)
	if err != nil {
		return err
	}
	key := sha256.Sum256([]byte(hash + "\x00" + password))
	m.verifiedMu.Lock()
	hit := subtle.ConstantTimeCompare(key[:], m.verifiedKey[:]) == 1 && time.Now().Before(m.verifiedUntil)
	m.verifiedMu.Unlock()
	if hit && strings.TrimSpace(hash) != "" {
		return nil
	}
	if err := m.verify(ctx, password); err != nil {
		return err
	}
	m.verifiedMu.Lock()
	m.verifiedKey, m.verifiedUntil = key, time.Now().Add(verifyCacheTTL)
	m.verifiedMu.Unlock()
	return nil
}

func (m *Manager) verify(ctx context.Context, password string) error {
	hash, err := m.store.GetSetting(ctx, SettingPasswordHash)
	if err != nil {
		return err
	}
	if strings.TrimSpace(hash) == "" {
		hash = dummyHash // equalize timing during unconfigured state
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return ErrInvalidPassword
	}
	return nil
}

func (m *Manager) storeHash(ctx context.Context, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	if err := m.store.SetSetting(ctx, SettingPasswordHash, hash); err != nil {
		return err
	}
	m.configured.Store(true)
	m.checked.Store(true)
	return nil
}

// Login verifies the password and returns a new opaque session token.
// The token is only ever present in this return value and the cookie;
// the database stores its SHA-256 so a DB leak can't hijack sessions.
func (m *Manager) Login(ctx context.Context, password string) (string, time.Time, error) {
	if err := m.verify(ctx, password); err != nil {
		return "", time.Time{}, err
	}
	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(SessionTTL)
	if err := m.store.CreateSession(ctx, TokenHash(token), expiresAt); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// Setup creates the initial password AND logs the caller in, returning
// a session token. It fails if a password already exists.
func (m *Manager) Setup(ctx context.Context, password string) (string, time.Time, error) {
	configured, err := m.IsConfigured(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	if configured {
		return "", time.Time{}, ErrAlreadyConfigured
	}
	if err := m.SetPassword(ctx, password); err != nil {
		return "", time.Time{}, err
	}
	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(SessionTTL)
	if err := m.store.CreateSession(ctx, TokenHash(token), expiresAt); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// Validate resolves a session token to its expiry, rejecting unknown or
// expired sessions. It never writes: a request carrying a bogus cookie
// must not cost a database write. Expired rows are removed by
// SweepExpired, which the server runs periodically.
func (m *Manager) Validate(ctx context.Context, token string) (time.Time, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return time.Time{}, ErrInvalidSession
	}
	expiry, err := m.store.GetSessionExpiry(ctx, TokenHash(token))
	if err != nil || time.Now().After(expiry) {
		return time.Time{}, ErrInvalidSession
	}
	return expiry, nil
}

// Logout revokes a single session. Unknown tokens are ignored so logout
// is idempotent.
func (m *Manager) Logout(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	return m.store.DeleteSession(ctx, TokenHash(token))
}

// SweepExpired removes expired sessions and reports how many were
// dropped. Intended for periodic or opportunistic cleanup.
func (m *Manager) SweepExpired(ctx context.Context) (int64, error) {
	return m.store.DeleteExpiredSessions(ctx, time.Now())
}

// TokenHash derives the storage key for a session token.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: generating session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
