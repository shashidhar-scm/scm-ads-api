-- Drop triggers
DROP TRIGGER IF EXISTS update_projects_updated_at ON projects;
DROP TRIGGER IF EXISTS update_devices_updated_at ON devices;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_devices_sync_status;
DROP INDEX IF EXISTS idx_devices_project;
DROP INDEX IF EXISTS idx_devices_host_name;
DROP INDEX IF EXISTS idx_projects_production;
DROP INDEX IF EXISTS idx_projects_name;

-- Drop tables
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS projects;
