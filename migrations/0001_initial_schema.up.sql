-- migrations/0001_initial_schema.up.sql
-- +goose Up
-- SQL in this section is executed when the migration is applied

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Advertisers table
CREATE TABLE advertisers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC')
);

-- Devices table
CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    location VARCHAR(255),
    status VARCHAR(50) DEFAULT 'active',
    last_seen TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC'),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC')
);

-- Campaigns table
CREATE TABLE campaigns (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'draft',
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE NOT NULL,
    budget DECIMAL(10, 2) NOT NULL,
    spent DECIMAL(10, 2) DEFAULT 0,
    impressions INTEGER DEFAULT 0,
    clicks INTEGER DEFAULT 0,
    ctr DECIMAL(5, 2) DEFAULT 0,
    advertiser_id UUID NOT NULL REFERENCES advertisers(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC')
);

-- Creatives table
CREATE TABLE creatives (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- 'image' or 'video'
    url TEXT NOT NULL,
    file_path TEXT NOT NULL,
    size BIGINT NOT NULL,
    uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC')
);

-- Creative assignments table
CREATE TABLE creative_assignments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    selected_days TEXT[] NOT NULL, -- Array of days: ['monday', 'tuesday', ...]
    time_slots TEXT[] NOT NULL,    -- Array of time slots: ['08:00-10:00', ...]
    devices TEXT[],                -- Array of device names: ['Mall Display 1', ...]
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    creative_id UUID NOT NULL REFERENCES creatives(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'UTC')
);

-- Indexes
CREATE INDEX idx_campaigns_advertiser_id ON campaigns(advertiser_id);
CREATE INDEX idx_creative_assignments_campaign_id ON creative_assignments(campaign_id);
CREATE INDEX idx_creative_assignments_creative_id ON creative_assignments(creative_id);