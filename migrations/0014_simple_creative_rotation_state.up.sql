-- migrations/0014_simple_creative_rotation_state.up.sql
-- +goose Up
-- SQL in this section is executed when the migration is applied

-- Remove legacy rotation-group schema and replace with per-campaign rotation state.

DROP TABLE IF EXISTS creative_rotation_state;

ALTER TABLE creatives
    DROP COLUMN IF EXISTS rotation_group_id;

DROP TABLE IF EXISTS rotation_groups;

CREATE TABLE IF NOT EXISTS creative_rotation_state (
    device TEXT NOT NULL,
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    last_creative_id UUID,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC'),
    PRIMARY KEY (device, campaign_id)
);
