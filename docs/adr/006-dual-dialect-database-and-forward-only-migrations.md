# ADR-006: Dual-Dialect Database and Forward-Only Migrations

Date: 2026-04-11
Status: Accepted
Related: [ADR-003](003-microkernel-plugin-architecture.md) (enterprise Postgres gates via plugin)

## Context

AgentLens must support two deployment profiles: community (single-node, zero-config) and enterprise (managed database, team-scale). The database layer must handle both without conditional compilation or separate binaries. Additionally, schema evolution must be safe, auditable, and work identically across both dialects — ruling out tools that generate dialect-specific SQL externally.

Key forces:

1. **Zero-config community experience** — SQLite requires no server, no credentials, no setup.
2. **Enterprise expectations** — PostgreSQL for durability, concurrent access, backup tooling.
3. **Dialect divergence** — SQLite and Postgres differ in DDL syntax (`INSERT OR IGNORE` vs `ON CONFLICT DO NOTHING`), column introspection (`PRAGMA table_info` vs `information_schema`), and feature support (SQLite < 3.35 lacks `DROP COLUMN`).
4. **Data seeding in migrations** — system roles, default settings, and bootstrap data must be created atomically with schema changes.

## Decision

### Dual-dialect via GORM with explicit branching

Use GORM as the database abstraction layer. A `db.Dialect` type (`"sqlite"` or `"postgres"`) selects the driver at startup. GORM handles most SQL differences transparently. Where it cannot, migrations and queries branch on `db.Dialect()` or `db.Name()`.

Configuration: `database.dialect` config key or `AGENTLENS_DB_DIALECT` env var. SQLite path defaults to `./data/agentlens.db`. Postgres DSN assembled from individual config fields (host, port, dbname, user, password, sslmode).

### SQLite production defaults

WAL mode, foreign keys enabled, 5-second busy timeout. Test mode: in-memory with shared cache, pinned to 1 connection to avoid SQLite concurrency issues.

### Forward-only versioned migrations in Go

Migrations are Go functions, not SQL files. Each migration is a `Migration` struct with `Version int` and `Up func(*gorm.DB) error`. A `schema_migrations` table tracks applied versions. Each migration runs in a transaction. New migrations append to `AllMigrations()`.

There is no `Down` function. Rollback is handled operationally (restore from backup), not programmatically.

### Migrations as Go code

Writing migrations in Go (not SQL files) allows:

- Dialect branching within a single migration (`INSERT OR IGNORE` vs `ON CONFLICT DO NOTHING`)
- Data seeding in the same transaction as schema changes (roles in migration 003, settings in 004) via GORM's `FirstOrCreate`
- Column introspection that branches by dialect (`PRAGMA table_info` for SQLite, `information_schema.columns` for Postgres), with table name allowlisting to prevent SQL injection

### GORM logger silent

Query logging disabled. Deliberate choice — SQL noise adds no value in production and masks application logs.

## Consequences

### Positive

- Community users get a working database with zero configuration.
- Enterprise users get PostgreSQL with the same binary — no rebuild, no separate artifact.
- Go-coded migrations enable dialect-aware logic and data seeding in one atomic transaction.
- Version tracking in `schema_migrations` makes applied state inspectable and migration idempotency verifiable.

### Negative / Trade-offs

- **No rollback** — a failed migration on production requires manual intervention or restore from backup. Forward-only is simpler but less forgiving.
- **Manual dialect testing** — developers must verify migrations against both SQLite and Postgres. No automated cross-dialect matrix yet.
- **Old SQLite limitations** — SQLite < 3.35 cannot `DROP COLUMN`. Some migrations silently leave dead columns rather than fail.
- **GORM abstraction leaks** — advanced queries (column introspection, upserts with conflict clauses) require raw SQL with dialect branches. The ORM does not eliminate dialect awareness, it only reduces it.
- **No connection pooling configuration** — pool settings are not exposed via config. Sufficient at current scale, will need attention for high-concurrency enterprise deployments.

### Neutral

- MySQL/MariaDB are explicitly unsupported. Two dialects is the maintenance ceiling.
- GORM's `AutoMigrate` is not used — it lacks versioning, cannot seed data, and handles complex schema changes poorly. All schema changes go through the versioned migration system.

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| **Raw SQL without ORM** | Requires writing every query twice (SQLite + Postgres). GORM eliminates most dialect differences for CRUD operations |
| **SQL file migrations (goose, golang-migrate)** | Cannot branch by dialect within a single migration. Cannot seed data in the same transaction without raw SQL escape hatches |
| **GORM AutoMigrate only** | No version tracking, no data seeding, cannot handle column renames or complex schema changes. Fine for prototyping, insufficient for production |
| **Down migrations (reversible)** | Adds complexity with low payoff — schema rollbacks rarely work cleanly when data has been written to new columns. Backup/restore is more reliable |
| **MySQL / MariaDB support** | Three dialects triples the testing surface. No demand from target users. Two is enough |
| **Read replicas / multiple connections** | Single-instance sufficient at current scale. Premature optimization that adds connection management complexity |
