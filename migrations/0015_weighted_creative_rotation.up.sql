-- migrations/0015_weighted_creative_rotation.up.sql
-- +goose Up
-- SQL in this section is executed when the migration is applied

-- Add per-creative weight for weighted rotation.
ALTER TABLE creatives
    ADD COLUMN IF NOT EXISTS play_weight INT NOT NULL DEFAULT 100;

-- Replace simple last-id rotation state with daily per-creative counters (UTC day).
DROP TABLE IF EXISTS creative_rotation_state;

CREATE TABLE IF NOT EXISTS creative_rotation_state_daily (
    device TEXT NOT NULL,
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    day DATE NOT NULL,
    creative_id UUID NOT NULL REFERENCES creatives(id) ON DELETE CASCADE,
    served_count INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC'),
    PRIMARY KEY (device, campaign_id, day, creative_id)
);

CREATE INDEX IF NOT EXISTS idx_creative_rotation_state_daily_lookup
    ON creative_rotation_state_daily (device, campaign_id, day);
