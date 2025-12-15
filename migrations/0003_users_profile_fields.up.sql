-- migrations/0003_users_profile_fields.up.sql
-- +goose Up
-- SQL in this section is executed when the migration is applied

ALTER TABLE users ADD COLUMN name VARCHAR(255);
ALTER TABLE users ADD COLUMN user_name VARCHAR(255);
ALTER TABLE users ADD COLUMN phone_number VARCHAR(50);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_user_name_unique ON users (LOWER(user_name));
