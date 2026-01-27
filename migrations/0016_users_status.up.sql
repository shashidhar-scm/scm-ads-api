-- +goose Up

ALTER TABLE users
    ADD COLUMN status TEXT NOT NULL DEFAULT 'pending';

ALTER TABLE users
    ADD CONSTRAINT users_status_check CHECK (status IN ('pending', 'active'));

-- Existing users should remain able to login.
UPDATE users SET status = 'active' WHERE status = 'pending';
