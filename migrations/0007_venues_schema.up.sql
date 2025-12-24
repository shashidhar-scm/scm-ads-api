-- Create venues table for grouping devices
CREATE TABLE venues (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create venue_devices junction table for many-to-many relationship
CREATE TABLE venue_devices (
    venue_id INTEGER NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
    device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    added_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (venue_id, device_id)
);

-- Create indexes for faster lookups
CREATE INDEX idx_venues_name ON venues(name);
CREATE INDEX idx_venue_devices_venue_id ON venue_devices(venue_id);
CREATE INDEX idx_venue_devices_device_id ON venue_devices(device_id);

-- Create trigger to update updated_at timestamp for venues
CREATE OR REPLACE FUNCTION update_venues_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER venues_updated_at_trigger
    BEFORE UPDATE ON venues
    FOR EACH ROW
    EXECUTE FUNCTION update_venues_updated_at();
