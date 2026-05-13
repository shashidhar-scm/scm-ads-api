CREATE TABLE IF NOT EXISTS place_exchange_tokens (
    doc_id TEXT PRIMARY KEY,
    city TEXT NOT NULL,
    token TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS place_exchange_tokens_city_idx
    ON place_exchange_tokens (LOWER(city));
