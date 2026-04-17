# Backend: Dual-Dialect Database

AgentLens supports SQLite (default) and PostgreSQL (enterprise). All database code must be dialect-aware.

### Branch on `db.Dialect()` — Never Assume SQLite-Only Syntax

Use `db.Dialect()` to branch dialect-specific SQL. SQLite uses TEXT for JSON and DATETIME for timestamps; PostgreSQL uses JSONB and TIMESTAMPTZ. Sources: CLAUDE.md, AGENTS.md.

### No SQLite-Specific DDL Types in Migrations or GORM Tags

Migrations and GORM `type:` tags must be dialect-aware. Types like BLOB, DATETIME, BOOLEAN work on SQLite but fail on PostgreSQL (BYTEA, TIMESTAMPTZ). Either use `tx.AutoMigrate` with migration-local structs so GORM picks the dialect type, or branch DDL per dialect in `migrations.go`. `ALTER TABLE DROP COLUMN IF EXISTS` is not accepted by SQLite. Source: PR reviews (#16, #1).

### Forward-Only Idempotent Migrations

Migrations are versioned Go functions appended to `AllMigrations()` in `internal/db/migrations.go`. They must be idempotent (safe to run multiple times). There is no `Down` function — rollback is handled operationally (backup/restore). Migration versions must strictly increase. Sources: CLAUDE.md, ADR-006, CONTRIBUTING.md.

### Migrations Must Work for Both Dialects

New migrations must pass on both SQLite and PostgreSQL. Branch on the database driver or use dialect-agnostic SQL; verify against both backends locally. Sources: CONTRIBUTING.md, ADR-006.

### Discovery Upsert by Endpoint

Discovery upserts catalog entries keyed by the `endpoint` column (UNIQUE constraint). `AgentKey = SHA256(protocol + endpoint)`. Sources: CLAUDE.md, ADR-008.

### Parameterized Queries Only (GORM)

All SQL queries go through GORM methods (`First`, `Where`, `Model`, `Updates`) with `?` placeholders. No raw string interpolation into SQL. Example: `tx.First(&user, "id = ?", id)`. Sources: code-patterns, CLAUDE.md, docs/devops-guide.md.
