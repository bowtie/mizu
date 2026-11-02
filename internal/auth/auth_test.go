package auth

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"mizu/internal/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return NewManager(db)
}

func TestIsConfiguredFalseByDefault(t *testing.T) {
	mgr := newTestManager(t)
	configured, err := mgr.IsConfigured(context.Background())
	require.NoError(t, err)
	assert.False(t, configured)
}

func TestSetPasswordAndLogin(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	require.NoError(t, mgr.SetPassword(ctx, "correct horse battery"))

	configured, err := mgr.IsConfigured(ctx)
	require.NoError(t, err)
	assert.True(t, configured)

	token, expiresAt, err := mgr.Login(ctx, "correct horse battery")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.True(t, expiresAt.After(time.Now()))

	expiry, err := mgr.Validate(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, expiresAt.Unix(), expiry.Unix())
}

func TestLoginWrongPassword(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	require.NoError(t, mgr.SetPassword(ctx, "hunter2hunter2"))

	_, _, err := mgr.Login(ctx, "wrong-password")
	assert.ErrorIs(t, err, ErrInvalidPassword)

	// Login against an unconfigured instance must also fail.
	empty := newTestManager(t)
	_, _, err = empty.Login(ctx, "anything")
	assert.ErrorIs(t, err, ErrInvalidPassword)
}

func TestSetupOnlyWorksOnce(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	_, _, err := mgr.Setup(ctx, "first-password")
	require.NoError(t, err)

	_, _, err = mgr.Setup(ctx, "second-password")
	assert.ErrorIs(t, err, ErrAlreadyConfigured)
}

func TestPasswordTooShort(t *testing.T) {
	mgr := newTestManager(t)
	err := mgr.SetPassword(context.Background(), "short")
	assert.ErrorIs(t, err, ErrPasswordTooShort)
}

// bcrypt only reads 72 bytes; longer passwords get a clear error rather
// than bcrypt's internal one. Exactly 72 bytes is still accepted.
func TestPasswordByteLimit(t *testing.T) {
	_, err := HashPassword(strings.Repeat("a", MaxPasswordBytes+1))
	assert.ErrorIs(t, err, ErrPasswordTooLong)
	// Multi-byte characters count by bytes: 37 × "é" (2 bytes each) = 74.
	_, err = HashPassword(strings.Repeat("é", 37))
	assert.ErrorIs(t, err, ErrPasswordTooLong)
	_, err = HashPassword(strings.Repeat("a", MaxPasswordBytes))
	assert.NoError(t, err)
}

func TestLogoutInvalidatesSession(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	require.NoError(t, mgr.SetPassword(ctx, "long-enough-password"))
	token, _, err := mgr.Login(ctx, "long-enough-password")
	require.NoError(t, err)

	require.NoError(t, mgr.Logout(ctx, token))
	_, err = mgr.Validate(ctx, token)
	assert.ErrorIs(t, err, ErrInvalidSession)

	// Logout is idempotent.
	assert.NoError(t, mgr.Logout(ctx, token))
	assert.NoError(t, mgr.Logout(ctx, "never-existed"))
}

func TestValidateRejectsGarbageTokens(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	for _, token := range []string{"", "   ", "not-a-token"} {
		_, err := mgr.Validate(ctx, token)
		assert.ErrorIs(t, err, ErrInvalidSession, "token %q", token)
	}
}

func TestChangePasswordRevokesSessions(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	require.NoError(t, mgr.SetPassword(ctx, "old-password-1"))
	oldToken, _, err := mgr.Login(ctx, "old-password-1")
	require.NoError(t, err)

	// Wrong current password is rejected and sessions survive.
	err = mgr.ChangePassword(ctx, "wrong-current", "new-password-2")
	assert.ErrorIs(t, err, ErrInvalidPassword)
	_, err = mgr.Validate(ctx, oldToken)
	require.NoError(t, err)

	require.NoError(t, mgr.ChangePassword(ctx, "old-password-1", "new-password-2"))

	// Old session revoked, new password accepted.
	_, err = mgr.Validate(ctx, oldToken)
	assert.ErrorIs(t, err, ErrInvalidSession)

	newToken, _, err := mgr.Login(ctx, "new-password-2")
	require.NoError(t, err)
	_, err = mgr.Validate(ctx, newToken)
	require.NoError(t, err)
}

func TestExpiredSessionRejectedAndSwept(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	require.NoError(t, mgr.SetPassword(ctx, "long-enough-password"))
	token, _, err := mgr.Login(ctx, "long-enough-password")
	require.NoError(t, err)

	// Force the stored expiry into the past.
	db := mgr.store.(*database.DB)
	require.NoError(t, db.DeleteAllSessions(ctx))
	require.NoError(t, db.CreateSession(ctx, TokenHash(token), time.Now().Add(-time.Minute)))

	// Sweeping removes exactly the expired row...
	n, err := mgr.SweepExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// ...and validation rejects the now-unknown token.
	_, err = mgr.Validate(ctx, token)
	assert.ErrorIs(t, err, ErrInvalidSession)
}

func TestTokenHashIsDeterministic(t *testing.T) {
	assert.Equal(t, TokenHash("abc"), TokenHash("abc"))
	assert.NotEqual(t, TokenHash("abc"), TokenHash("abd"))
}

func TestConcurrentSetupHasOneWinner(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	const racers = 8
	errs := make(chan error, racers)
	var start sync.WaitGroup
	start.Add(1)
	for i := 0; i < racers; i++ {
		go func(i int) {
			start.Wait()
			_, _, err := mgr.Setup(ctx, fmt.Sprintf("password-%d-long", i))
			errs <- err
		}(i)
	}
	start.Done()

	wins := 0
	for i := 0; i < racers; i++ {
		err := <-errs
		if err == nil {
			wins++
		} else {
			assert.ErrorIs(t, err, ErrAlreadyConfigured)
		}
	}
	assert.Equal(t, 1, wins, "exactly one first-run setup may succeed")
}

func TestValidateDoesNotWriteForBogusTokens(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, mgr.SetPassword(ctx, "long-enough-password"))

	db := mgr.store.(*database.DB)
	require.NoError(t, db.CreateSession(ctx, TokenHash("stale"), time.Now().Add(-time.Minute)))

	_, err := mgr.Validate(ctx, "bogus")
	assert.ErrorIs(t, err, ErrInvalidSession)

	// The expired row is still there: only SweepExpired removes it.
	n, err := mgr.SweepExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestVerifyCachedFollowsPasswordChanges(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, mgr.SetPassword(ctx, "first-password"))

	require.NoError(t, mgr.VerifyCached(ctx, "first-password"))
	require.NoError(t, mgr.VerifyCached(ctx, "first-password"), "cached hit")
	assert.ErrorIs(t, mgr.VerifyCached(ctx, "wrong-password"), ErrInvalidPassword)

	require.NoError(t, mgr.ChangePassword(ctx, "first-password", "second-password"))
	assert.ErrorIs(t, mgr.VerifyCached(ctx, "first-password"), ErrInvalidPassword, "old password not served from cache")
	require.NoError(t, mgr.VerifyCached(ctx, "second-password"))
}
