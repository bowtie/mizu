-- +goose Up
-- +goose StatementBegin
ALTER TABLE show_mappings ADD COLUMN artwork_custom_slots TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE show_mappings DROP COLUMN artwork_custom_slots;
-- +goose StatementEnd
