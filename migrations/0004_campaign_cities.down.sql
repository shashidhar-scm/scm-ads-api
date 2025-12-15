-- migrations/0004_campaign_cities.down.sql
-- +goose Down
-- SQL in this section is executed when the migration is rolled back

ALTER TABLE campaigns
DROP COLUMN IF EXISTS cities;
