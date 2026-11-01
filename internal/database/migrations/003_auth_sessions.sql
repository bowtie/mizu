-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS auth_sessions (
    token_hash TEXT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT (datetime('now')),
    expires_at TIMESTAMP NOT NULL
);
-- +goose StatementEnd

CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires ON auth_sessions(expires_at);

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_auth_sessions_expires;
DROP TABLE IF EXISTS auth_sessions;
-- +goose StatementEnd
