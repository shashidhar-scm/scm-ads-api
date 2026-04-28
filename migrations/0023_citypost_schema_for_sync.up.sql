-- Create citypost schema if it doesn't exist
CREATE SCHEMA IF NOT EXISTS citypost;

-- Create posters table for synced data
CREATE TABLE IF NOT EXISTS citypost.posters (
    mongo_id VARCHAR(255) PRIMARY KEY,
    data JSONB NOT NULL,
    region VARCHAR(100),
    city VARCHAR(100),
    status VARCHAR(50),
    title TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create ad_posters table for synced data
CREATE TABLE IF NOT EXISTS citypost.ad_posters (
    external_id VARCHAR(255) PRIMARY KEY,
    data JSONB NOT NULL,
    status VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_posters_region ON citypost.posters(region);
CREATE INDEX IF NOT EXISTS idx_posters_city ON citypost.posters(city);
CREATE INDEX IF NOT EXISTS idx_posters_status ON citypost.posters(status);
CREATE INDEX IF NOT EXISTS idx_adposters_status ON citypost.ad_posters(status);
CREATE INDEX IF NOT EXISTS idx_posters_data_region ON citypost.posters USING gin((data->'region'));
CREATE INDEX IF NOT EXISTS idx_adposters_data_region ON citypost.ad_posters USING gin((data->'region'));
