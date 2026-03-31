# Database Configuration

AgentLens supports two database backends: **SQLite** (default) and **PostgreSQL**.

---

## SQLite (Default)

SQLite is the default database backend — zero configuration required. It stores data in a single file, making it ideal for development, single-instance deployments, and small teams.

### Configuration

```yaml
database:
  dialect: sqlite
  sqlite:
    path: ./data/agentlens.db
```

Or via environment variables:

```bash
export AGENTLENS_DB_DIALECT=sqlite
export AGENTLENS_DB_SQLITE_PATH=./data/agentlens.db
```

### Notes

- The database file is created automatically on first run.
- SQLite uses WAL (Write-Ahead Logging) mode for better concurrent read performance.
- Suitable for single-instance deployments; do **not** use SQLite with multiple AgentLens instances sharing the same file.

---

## PostgreSQL

PostgreSQL is recommended for production deployments, multi-instance setups, and high-availability configurations.

### Configuration

```yaml
database:
  dialect: postgres
  postgres:
    host: localhost
    port: 5432
    user: agentlens
    password: secret
    dbname: agentlens
    sslmode: prefer
```

Or via environment variables:

```bash
export AGENTLENS_DB_DIALECT=postgres
export AGENTLENS_DB_POSTGRES_HOST=localhost
export AGENTLENS_DB_POSTGRES_PORT=5432
export AGENTLENS_DB_POSTGRES_USER=agentlens
export AGENTLENS_DB_POSTGRES_PASSWORD=secret
export AGENTLENS_DB_POSTGRES_DBNAME=agentlens
export AGENTLENS_DB_POSTGRES_SSLMODE=prefer
```

### SSL Modes

| Mode | Description |
|---|---|
| `disable` | No SSL (development only) |
| `require` | SSL required, no certificate verification |
| `verify-ca` | SSL required, verify server certificate authority |
| `verify-full` | SSL required, verify CA and hostname |
| `prefer` | Use SSL if available, fall back to unencrypted |

### Docker Compose

A ready-to-use Docker Compose file with PostgreSQL is available:

```bash
cd examples
docker compose -f docker-compose.postgres.yaml up
```

See [`examples/docker-compose.postgres.yaml`](../examples/docker-compose.postgres.yaml).

---

## Migrations

AgentLens runs database migrations automatically on startup. Migrations are embedded in the binary and applied in order.

- Migrations are idempotent — safe to run multiple times.
- On first run, all tables are created (catalog entries, users, roles, permissions, settings).
- Subsequent runs apply any new migrations added in newer versions.
- Migration state is tracked in a `schema_migrations` table.

### Adding a New Migration

1. Add a new `Migration` entry in the Go code under `internal/db` (alongside the existing migrations).
2. Assign it the next migration version (keeping versions strictly increasing) and implement the migration logic in the `Migration` definition.
3. Ensure it is included in `AllMigrations()` so the `Migrator` can apply it; it will then be automatically picked up and run on the next startup.

---

## Backup

### SQLite

Back up the SQLite database by copying the database file:

```bash
cp ./data/agentlens.db ./data/agentlens.db.backup
```

For a consistent backup while the server is running, use the SQLite `.backup` command:

```bash
sqlite3 ./data/agentlens.db ".backup ./data/agentlens.db.backup"
```

### PostgreSQL

Use standard PostgreSQL backup tools:

```bash
# Full dump
pg_dump -U agentlens -h localhost agentlens > backup.sql

# Restore
psql -U agentlens -h localhost agentlens < backup.sql
```

For production, consider using `pg_dump` with `--format=custom` for compressed, flexible restores:

```bash
pg_dump -U agentlens -h localhost -Fc agentlens > backup.dump
pg_restore -U agentlens -h localhost -d agentlens backup.dump
```
