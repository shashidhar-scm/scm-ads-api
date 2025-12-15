-- migrations/0004_campaign_cities.up.sql
-- +goose Up
-- SQL in this section is executed when the migration is applied

ALTER TABLE campaigns
ADD COLUMN IF NOT EXISTS cities TEXT[] NOT NULL DEFAULT '{}';
