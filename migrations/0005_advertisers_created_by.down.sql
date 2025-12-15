-- migrations/0005_advertisers_created_by.down.sql
-- +goose Down
-- SQL in this section is executed when the migration is rolled back

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'advertisers_created_by_fkey'
    ) THEN
        ALTER TABLE advertisers DROP CONSTRAINT advertisers_created_by_fkey;
    END IF;

    ALTER TABLE advertisers
    DROP COLUMN IF EXISTS created_by;
END $$;
