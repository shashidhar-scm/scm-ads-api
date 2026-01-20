-- migrations/0012_ad_rotation_groups.up.sql
-- +goose Up
-- SQL in this section is executed when the migration is applied

CREATE TABLE IF NOT EXISTS rotation_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    selected_days TEXT[] NOT NULL DEFAULT '{}',
    time_slots TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC'),
    UNIQUE (campaign_id, name)
);

ALTER TABLE creatives
    ADD COLUMN IF NOT EXISTS rotation_group_id UUID REFERENCES rotation_groups(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_creatives_rotation_group_id ON creatives(rotation_group_id);

CREATE TABLE IF NOT EXISTS creative_rotation_state (
    device TEXT NOT NULL,
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    rotation_group_id UUID NOT NULL REFERENCES rotation_groups(id) ON DELETE CASCADE,
    last_creative_id UUID,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC'),
    PRIMARY KEY (device, campaign_id, rotation_group_id)
);
