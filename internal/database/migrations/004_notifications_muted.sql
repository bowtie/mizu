-- +goose Up
-- +goose StatementBegin
ALTER TABLE show_mappings ADD COLUMN notifications_muted INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE show_mappings DROP COLUMN notifications_muted;
-- +goose StatementEnd
