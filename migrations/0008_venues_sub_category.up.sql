-- Add sub_category column to venues for tagging venue types
ALTER TABLE venues
ADD COLUMN IF NOT EXISTS sub_category TEXT[] NOT NULL DEFAULT ARRAY[]::text[];
