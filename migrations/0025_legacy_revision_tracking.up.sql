CREATE TABLE IF NOT EXISTS legacy_region_sequences (
    doc_type TEXT NOT NULL,
    region TEXT NOT NULL,
    update_seq BIGINT NOT NULL,
    PRIMARY KEY (doc_type, region)
);

CREATE TABLE IF NOT EXISTS legacy_doc_revisions (
    doc_type TEXT NOT NULL,
    region TEXT NOT NULL,
    doc_id TEXT NOT NULL,
    generation INTEGER NOT NULL,
    hash_suffix CHAR(8) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (doc_type, region, doc_id)
);

CREATE SEQUENCE IF NOT EXISTS legacy_revision_seq;
