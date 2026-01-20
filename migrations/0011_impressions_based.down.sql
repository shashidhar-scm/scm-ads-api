-- migrations/0011_impressions_based.down.sql

ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS impressions INTEGER DEFAULT 0;

ALTER TABLE creatives
    DROP COLUMN IF EXISTS impressions_served,
    DROP COLUMN IF EXISTS impression_count;

ALTER TABLE campaigns
    DROP COLUMN IF EXISTS impressions_based;
