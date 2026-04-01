# Contributing to AgentLens

Thank you for your interest in contributing! This guide covers how to get started.

## Development Setup

**Prerequisites:** Go 1.26.1, Bun 1.3+, SQLite

```bash
git clone https://github.com/PawelHaracz/agentlens
cd agentlens

# Build backend
go build ./...

# Run tests
go test ./...

# Build frontend
cd web && bun install && bun run build && cd ..

# Run locally
go run ./cmd/agentlens
```

The server starts on <http://localhost:8080>.

## Project Structure

```text
cmd/agentlens/      # Main entrypoint
internal/
  api/              # HTTP handlers, router, middleware
  config/           # Configuration loading
  discovery/        # Agent discovery (K8s, static, A2A, MCP)
  health/           # Health checker
  model/            # Core data types
  server/           # HTTP server lifecycle
  store/            # SQLite persistence
web/                # React frontend (TypeScript + Vite + Tailwind)
deploy/helm/        # Helm chart
examples/           # Docker Compose + mock agents
docs/               # API documentation
```

## Making Changes

1. Fork the repository and create a feature branch.
2. Write tests for new functionality.
3. Run `go test ./...` to ensure tests pass.
4. Run `go vet ./...` and `go build ./...`.
5. If changing the frontend, run `cd web && bun run build`.
6. Open a pull request with a clear description of the change.

## Code Style

- Go: follow standard Go conventions (`gofmt`, `go vet`).
- TypeScript: strict mode, no unused variables.
- Keep functions small and focused.
- Comment public APIs.

## Reporting Issues

Please open a GitHub issue with:

- A clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Go/Bun version and OS

## Running with PostgreSQL

To run AgentLens locally with PostgreSQL instead of SQLite:

```bash
cd examples
docker compose -f docker-compose.postgres.yaml up
```

This starts AgentLens with a PostgreSQL 16 database, along with mock A2A and MCP agents for testing. The PostgreSQL data is persisted in a Docker volume (`pgdata`).

To reset the database:

```bash
docker compose -f docker-compose.postgres.yaml down -v
docker compose -f docker-compose.postgres.yaml up
```

## Adding a New Migration

Database migrations are implemented in Go under `internal/db` and are applied automatically on startup through the migrator (see the `AllMigrations()` function).

To add a new migration:

1. Inspect the existing migrations in `internal/db` (and their registration in `AllMigrations()`) to determine the next sequence/order number.
2. Add a new migration in `internal/db` (following the existing patterns), including `Up` logic.
3. Ensure the migration is safe to run multiple times or is otherwise handled idempotently by the migrator.
4. Make sure the migration works for both SQLite and PostgreSQL (e.g., by branching on the database driver or using dialect-agnostic SQL as done in existing migrations).
5. Register the new migration in `AllMigrations()` so it is included when the application starts.
6. Test by running the application against both database backends.

## Adding a New Permission

Permissions follow the `resource:action` format (e.g., `catalog:read`, `users:write`).

To add a new permission:

1. Define the permission constant in the auth/RBAC code (e.g., `myresource:read`).
2. Add it to the appropriate default roles in the bootstrap migration or seed logic.
3. Reference the permission in the middleware/handler where access control is enforced.
4. Update the documentation:
   - `docs/auth.md` — add the permission to the roles/permissions tables
   - `docs/api.md` — note which endpoints require the new permission
   - `README.md` — update the Roles & Permissions table if needed
5. Write tests to verify the permission is enforced correctly.

## License

By contributing, you agree that your contributions will be licensed under the same terms as the project, as specified in the LICENSE file (currently the Business Source License 1.1).
