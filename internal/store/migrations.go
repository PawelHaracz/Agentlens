package store

// schema is the SQL DDL for the catalog_entries table.
const schema = `
CREATE TABLE IF NOT EXISTS catalog_entries (
    id               TEXT PRIMARY KEY,
    display_name     TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    protocol         TEXT NOT NULL,
    endpoint         TEXT NOT NULL UNIQUE,
    version          TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'unknown',
    source           TEXT NOT NULL,
    provider         TEXT NOT NULL DEFAULT '{}',
    categories       TEXT NOT NULL DEFAULT '[]',
    skills           TEXT NOT NULL DEFAULT '[]',
    validity_from    DATETIME,
    validity_to      DATETIME,
    validity_last_seen DATETIME NOT NULL,
    raw_card         TEXT,
    metadata         TEXT NOT NULL DEFAULT '{}',
    created_at       DATETIME NOT NULL,
    updated_at       DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_catalog_entries_protocol ON catalog_entries(protocol);
CREATE INDEX IF NOT EXISTS idx_catalog_entries_status ON catalog_entries(status);
CREATE INDEX IF NOT EXISTS idx_catalog_entries_source ON catalog_entries(source);
CREATE INDEX IF NOT EXISTS idx_catalog_entries_provider ON catalog_entries(provider);
`
