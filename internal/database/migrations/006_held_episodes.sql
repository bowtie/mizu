-- +goose Up
-- +goose StatementBegin
-- Episodes Mizu unmonitored because AnimeFillerList hadn't classified them
-- yet, so they can be monitored again if it never does.
CREATE TABLE held_episodes (
    mapping_id INTEGER NOT NULL REFERENCES show_mappings(id) ON DELETE CASCADE,
    sonarr_episode_id INTEGER NOT NULL,
    held_at DATETIME NOT NULL,
    PRIMARY KEY (mapping_id, sonarr_episode_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE held_episodes;
-- +goose StatementEnd
