-- +goose Up
-- +goose StatementBegin
ALTER TABLE show_mappings ADD COLUMN artwork_poster_url TEXT NOT NULL DEFAULT '';
ALTER TABLE show_mappings ADD COLUMN artwork_banner_url TEXT NOT NULL DEFAULT '';
ALTER TABLE show_mappings ADD COLUMN artwork_logo_url TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE show_mappings DROP COLUMN artwork_poster_url;
ALTER TABLE show_mappings DROP COLUMN artwork_banner_url;
ALTER TABLE show_mappings DROP COLUMN artwork_logo_url;
-- +goose StatementEnd
