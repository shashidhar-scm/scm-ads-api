-- Drop venues table and related objects
DROP TRIGGER IF EXISTS venues_updated_at_trigger ON venues;
DROP FUNCTION IF EXISTS update_venues_updated_at();
DROP INDEX IF EXISTS idx_venue_devices_device_id;
DROP INDEX IF EXISTS idx_venue_devices_venue_id;
DROP INDEX IF EXISTS idx_venues_name;
DROP TABLE IF EXISTS venue_devices;
DROP TABLE IF EXISTS venues;
