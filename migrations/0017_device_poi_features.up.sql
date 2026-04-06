-- migrations/0017_device_poi_features.up.sql

CREATE TABLE IF NOT EXISTS device_poi_features (
    host_name TEXT PRIMARY KEY REFERENCES devices(host_name) ON DELETE CASCADE,
    city TEXT,
    region_code TEXT,
    college_count_1km INT NOT NULL DEFAULT 0,
    college_count_2km INT NOT NULL DEFAULT 0,
    dorm_count_1km INT NOT NULL DEFAULT 0,
    hostel_count_1km INT NOT NULL DEFAULT 0,
    nearest_college_distance_km DOUBLE PRECISION,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')
);

CREATE INDEX IF NOT EXISTS idx_device_poi_features_city_region
    ON device_poi_features (city, region_code);
