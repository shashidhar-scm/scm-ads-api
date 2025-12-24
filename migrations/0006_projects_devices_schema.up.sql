-- Create projects table
CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY,
    owner JSONB NOT NULL,
    languages JSONB NOT NULL DEFAULT '[]',
    name TEXT NOT NULL UNIQUE,
    company TEXT,
    description TEXT,
    max_devices INTEGER NOT NULL DEFAULT 0,
    profile_img TEXT,
    header BOOLEAN NOT NULL DEFAULT FALSE,
    sub_type TEXT,
    production BOOLEAN NOT NULL DEFAULT FALSE,
    city_poster_frequency INTEGER NOT NULL DEFAULT 0,
    ad_poster_frequency INTEGER NOT NULL DEFAULT 0,
    city_poster_play_time INTEGER NOT NULL DEFAULT 0,
    loop_length INTEGER NOT NULL DEFAULT 0,
    smallbiz_support BOOLEAN NOT NULL DEFAULT FALSE,
    proxy TEXT,
    address TEXT,
    latitude TEXT,
    longitude TEXT,
    is_transit BOOLEAN NOT NULL DEFAULT FALSE,
    scm_health BOOLEAN NOT NULL DEFAULT FALSE,
    priority INTEGER NOT NULL DEFAULT 0,
    replicas INTEGER NOT NULL DEFAULT 0,
    region JSONB NOT NULL DEFAULT '[]',
    status TEXT,
    role TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create devices table
CREATE TABLE devices (
    id INTEGER PRIMARY KEY,
    device_type JSONB NOT NULL,
    region JSONB NOT NULL,
    name TEXT NOT NULL,
    host_name TEXT NOT NULL UNIQUE,
    description TEXT,
    change BOOLEAN NOT NULL DEFAULT FALSE,
    last_synced_at TIMESTAMP WITH TIME ZONE,
    sync_status TEXT,
    project INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    device_config JSONB NOT NULL DEFAULT '{}',
    rtty_data BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);
CREATE INDEX IF NOT EXISTS idx_projects_production ON projects(production);
CREATE INDEX IF NOT EXISTS idx_devices_host_name ON devices(host_name);
CREATE INDEX IF NOT EXISTS idx_devices_project ON devices(project);
CREATE INDEX IF NOT EXISTS idx_devices_sync_status ON devices(sync_status);

-- Trigger to auto-update updated_at on projects
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_projects_updated_at BEFORE UPDATE
    ON projects FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_devices_updated_at BEFORE UPDATE
    ON devices FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
