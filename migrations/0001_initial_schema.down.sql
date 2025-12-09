-- +goose Down
-- SQL in this section is executed when the migration is rolled back

DROP TABLE IF EXISTS creative_assignments;
DROP TABLE IF EXISTS creatives;
DROP TABLE IF EXISTS campaigns;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS advertisers;
DROP EXTENSION IF EXISTS "uuid-ossp";