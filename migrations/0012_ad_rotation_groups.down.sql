-- migrations/0012_ad_rotation_groups.down.sql
-- +goose Down
-- SQL in this section is executed when the migration is rolled back

DROP TABLE IF EXISTS creative_rotation_state;

ALTER TABLE creatives
    DROP COLUMN IF EXISTS rotation_group_id;

DROP TABLE IF EXISTS rotation_groups;
