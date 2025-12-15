-- migrations/0002_auth_schema.down.sql
-- +goose Down
-- SQL in this section is executed when the migration is rolled back

DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS users;
