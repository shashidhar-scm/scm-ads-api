-- migrations/0011_impressions_based.up.sql

ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS impressions_based BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE creatives
    ADD COLUMN IF NOT EXISTS impression_count BIGINT,
    ADD COLUMN IF NOT EXISTS impressions_served BIGINT NOT NULL DEFAULT 0;

ALTER TABLE campaigns
    DROP COLUMN IF EXISTS impressions;
