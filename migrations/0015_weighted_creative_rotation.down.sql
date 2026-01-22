-- migrations/0015_weighted_creative_rotation.down.sql
-- +goose Down
-- SQL in this section is executed when the migration is rolled back

DROP TABLE IF EXISTS creative_rotation_state_daily;

-- Note: we don't recreate the previous creative_rotation_state table on rollback.

ALTER TABLE creatives
    DROP COLUMN IF EXISTS play_weight;
