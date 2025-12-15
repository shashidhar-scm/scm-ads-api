-- migrations/0003_users_profile_fields.down.sql
-- +goose Down
-- SQL in this section is executed when the migration is rolled back

DROP INDEX IF EXISTS idx_users_user_name_unique;

ALTER TABLE users DROP COLUMN IF EXISTS phone_number;
ALTER TABLE users DROP COLUMN IF EXISTS user_name;
ALTER TABLE users DROP COLUMN IF EXISTS name;
