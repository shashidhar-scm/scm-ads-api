-- migrations/0005_advertisers_created_by.up.sql
-- +goose Up
-- SQL in this section is executed when the migration is applied

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM advertisers LIMIT 1) THEN
        RAISE EXCEPTION 'Cannot add NOT NULL advertisers.created_by when advertisers already has rows. Please backfill created_by first.';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'advertisers'
          AND column_name = 'created_by'
    ) THEN
        ALTER TABLE advertisers ADD COLUMN created_by UUID NOT NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'advertisers_created_by_fkey'
    ) THEN
        ALTER TABLE advertisers
        ADD CONSTRAINT advertisers_created_by_fkey
        FOREIGN KEY (created_by)
        REFERENCES users(id)
        ON DELETE RESTRICT;
    END IF;
END $$;
