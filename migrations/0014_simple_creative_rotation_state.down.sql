-- migrations/0014_simple_creative_rotation_state.down.sql
-- +goose Down
-- SQL in this section is executed when the migration is rolled back

DROP TABLE IF EXISTS creative_rotation_state;

-- Note: legacy rotation_groups schema is not recreated on down migration.
