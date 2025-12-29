-- Remove sub_category column from venues
ALTER TABLE venues
DROP COLUMN IF EXISTS sub_category;
