package store

// schema is the SQL DDL for the agents table.
const schema = `
CREATE TABLE IF NOT EXISTS agents (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    protocol    TEXT NOT NULL,
    endpoint    TEXT NOT NULL,
    version     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'unknown',
    source      TEXT NOT NULL,
    namespace   TEXT NOT NULL DEFAULT '',
    team        TEXT NOT NULL DEFAULT '',
    tags        TEXT NOT NULL DEFAULT '[]',
    skills      TEXT NOT NULL DEFAULT '[]',
    raw_card    TEXT,
    last_seen   DATETIME NOT NULL,
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agents_protocol ON agents(protocol);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_source ON agents(source);
CREATE INDEX IF NOT EXISTS idx_agents_endpoint ON agents(endpoint);
CREATE INDEX IF NOT EXISTS idx_agents_team ON agents(team);
`
