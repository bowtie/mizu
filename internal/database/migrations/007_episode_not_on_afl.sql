-- +goose Up
-- +goose StatementBegin
-- Set for an episode AnimeFillerList doesn't count (a banned one, say), so
-- the show page labels it by its Sonarr season and episode instead of a
-- number that would clash with its neighbour's.
ALTER TABLE episode_info ADD COLUMN not_on_afl INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE episode_info DROP COLUMN not_on_afl;
-- +goose StatementEnd
