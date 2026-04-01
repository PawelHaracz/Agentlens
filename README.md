# AgentLens

**Real-time AI agent catalog for Kubernetes — discover, track, and inspect A2A, MCP, and A2UI agents across your cluster.**

AgentLens automatically discovers AI agents running in Kubernetes (via Service annotations), polls static endpoints, and accepts push registrations. It exposes a REST API and a web dashboard for browsing the catalog, filtering by protocol/status, and inspecting agent cards and skills.

---

## Quickstart

### Docker Compose

```bash
git clone https://github.com/PawelHaracz/agentlens
cd agentlens/examples
docker compose up
```

Open http://localhost:8080 in your browser.

### Helm (Kubernetes)

```bash
helm install agentlens ./deploy/helm/agentlens \
  --namespace agentlens --create-namespace
```

AgentLens will start watching Services across all namespaces for agent annotations.

---

## Kubernetes Annotations

Annotate your Services to register agents automatically:

| Annotation | Required | Description |
|---|---|---|
| `agentlens.io/type` | ✓ | One of `a2a`, `mcp`, `a2ui` |
| `agentlens.io/card-path` | | Custom card path (defaults: `/.well-known/agent-card.json` for A2A, `/.well-known/mcp/server.json` for MCP) |
| `agentlens.io/team` | | Owning team label |
| `agentlens.io/tags` | | Comma-separated categories |

**Example:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-agent
  annotations:
    agentlens.io/type: "a2a"
    agentlens.io/team: "platform"
    agentlens.io/tags: "nlp,support"
spec:
  selector:
    app: my-agent
  ports:
    - port: 8080
```

---

## Static Config Example

```yaml
# agentlens.yaml
port: 8080
data_dir: ./data
log_level: info
poll_interval: 5m

sources:
  - name: my-a2a-agent
    type: a2a
    url: http://agent.internal:8080
  - name: my-mcp-server
    type: mcp
    url: http://mcp.internal:9000

health_check:
  enabled: true
  interval: 30s
  timeout: 5s
  concurrency: 10
```

Run with:

```bash
agentlens --config agentlens.yaml
```

---

## Agent Card Validation

Before registering an A2A agent card, validate it using the validation endpoint:

```bash
curl -X POST http://localhost:8080/api/v1/catalog/validate \
  -H "Content-Type: application/json" \
  -d @agent-card.json
```

The validation endpoint:
- **Auto-detects** A2A specification version (v0.3 vs v1.0)
- **Returns** structured errors and warnings if validation fails
- **Returns** a preview of the agent details if valid
- **Requires authentication** — `catalog:write` permission needed

Example response for a valid card:

```json
{
  "valid": true,
  "spec_version": "1.0",
  "errors": [],
  "warnings": [],
  "preview": {
    "display_name": "Example Chat Agent",
    "description": "A sample agent demonstrating A2A v1.0 features",
    "protocol": "a2a",
    "spec_version": "1.0",
    "skills_count": 0,
    "extensions_count": 1,
    "security_schemes": ["oauth2"],
    "interfaces": ["https://api.example.com/v1"]
  }
}
```

---

## Push Registration

Register a catalog entry via HTTP POST:

```bash
curl -X POST http://localhost:8080/api/v1/catalog \
  -H "Content-Type: application/json" \
  -d '{
    "display_name": "my-agent",
    "description": "Does amazing things",
    "protocol": "a2a",
    "endpoint": "http://my-agent.internal:8080",
    "version": "1.2.3",
    "provider": {"organization": "Acme Corp", "team": "platform"},
    "categories": ["nlp", "demo"]
  }'
```

---

## API Reference

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check |
| `GET` | `/api/v1/catalog` | List catalog entries (`?protocol=`, `?status=`, `?q=`, `?team=`, `?categories=`, `?limit=`, `?offset=`) |
| `POST` | `/api/v1/catalog/validate` | Validate A2A agent card (dry-run, does not persist) |
| `POST` | `/api/v1/catalog/register` | Register an A2A agent from a raw agent card JSON |
| `POST` | `/api/v1/catalog` | Push-register a catalog entry |
| `GET` | `/api/v1/catalog/{id}` | Get entry by ID |
| `DELETE` | `/api/v1/catalog/{id}` | Delete entry |
| `GET` | `/api/v1/catalog/{id}/card` | Get raw protocol card JSON |
| `GET` | `/api/v1/skills?q=` | Search entries by skill name |
| `GET` | `/api/v1/stats` | Aggregate stats |
| `POST` | `/api/v1/auth/login` | Login and obtain JWT token |
| `POST` | `/api/v1/auth/logout` | Logout (invalidate token) |
| `POST` | `/api/v1/auth/refresh` | Refresh JWT token |
| `GET` | `/api/v1/auth/me` | Get current user info |
| `PUT` | `/api/v1/auth/password` | Change current user password |
| `GET` | `/api/v1/users` | List users |
| `POST` | `/api/v1/users` | Create user |
| `GET` | `/api/v1/users/{id}` | Get user by ID |
| `PUT` | `/api/v1/users/{id}` | Update user |
| `DELETE` | `/api/v1/users/{id}` | Delete user |
| `GET` | `/api/v1/roles` | List roles |
| `POST` | `/api/v1/roles` | Create role |
| `PUT` | `/api/v1/roles/{id}` | Update role |
| `DELETE` | `/api/v1/roles/{id}` | Delete role |
| `GET` | `/api/v1/settings` | List settings |
| `GET` | `/api/v1/settings/{category}` | List settings in a category |
| `PUT` | `/api/v1/settings` | Bulk update settings |

See [docs/api.md](docs/api.md) for full API documentation.

---

## Architecture

AgentLens uses a **microkernel plugin architecture**:

- **Core kernel** — manages store, config, logger, and plugin lifecycle
- **Parser plugins** — A2A and MCP card parsers (extensible)
- **Source plugins** — static config, Kubernetes discovery (extensible)
- **Enterprise plugins** — SSO, RBAC, audit, PostgreSQL (license-gated)

The domain model follows the **Product Archetype Pattern** where each discovered agent/server is a `CatalogEntry` wrapping a `ProductType` (protocol).

---

## Database

AgentLens supports two database backends:

- **SQLite** (default) — zero-config, file-based, ideal for single-instance deployments
- **PostgreSQL** — recommended for production, multi-instance, and high-availability setups

Configure via `database.dialect` in the config file or `AGENTLENS_DB_DIALECT` env var.

### SQLite (default)
```yaml
database:
  dialect: sqlite
  sqlite:
    path: ./data/agentlens.db
```

### PostgreSQL
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

See [docs/database.md](docs/database.md) for full database documentation.

---

## Authentication

AgentLens includes built-in authentication with JWT tokens and role-based access control.

### First Run
On first startup, AgentLens creates an `admin` user with a randomly generated password printed to stdout:

```
============================================
  INITIAL ADMIN CREDENTIALS
  Username: admin
  Password: <generated>
  CHANGE THIS PASSWORD IMMEDIATELY
============================================
```

### Login
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "<your-password>"}'
```

The response includes a JWT token. Use it in subsequent requests:
```bash
curl http://localhost:8080/api/v1/catalog \
  -H "Authorization: Bearer <token>"
```

See [docs/auth.md](docs/auth.md) for full authentication documentation.

---

## Roles & Permissions

Three default roles are created on first run:

| Role | Permissions |
|------|------------|
| **admin** | Full access: catalog, users, roles, settings (read/write/delete) |
| **editor** | catalog:read/write, users:read, roles:read, settings:read |
| **viewer** | catalog:read, users:read, roles:read, settings:read |

All permissions follow the `resource:action` format (e.g., `catalog:read`, `users:write`).

---

## Configuration Reference

| Environment Variable | Default | Description |
|---|---|---|
| `AGENTLENS_PORT` | `8080` | HTTP server port |
| `AGENTLENS_DATA_DIR` | `./data` | Data directory for SQLite |
| `AGENTLENS_LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `AGENTLENS_POLL_INTERVAL` | `5m` | Discovery poll interval |
| `AGENTLENS_DB_DIALECT` | `sqlite` | Database backend (sqlite/postgres) |
| `AGENTLENS_DB_SQLITE_PATH` | `./data/agentlens.db` | SQLite database file path |
| `AGENTLENS_DB_POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `AGENTLENS_DB_POSTGRES_PORT` | `5432` | PostgreSQL port |
| `AGENTLENS_DB_POSTGRES_USER` | `agentlens` | PostgreSQL user |
| `AGENTLENS_DB_POSTGRES_PASSWORD` | | PostgreSQL password |
| `AGENTLENS_DB_POSTGRES_DBNAME` | `agentlens` | PostgreSQL database name |
| `AGENTLENS_DB_POSTGRES_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `AGENTLENS_JWT_SECRET` | (auto-generated) | JWT signing secret |
| `AGENTLENS_SESSION_DURATION` | `24h` | JWT token expiration |
| `AGENTLENS_KUBERNETES_ENABLED` | `false` | Enable Kubernetes discovery |
| `AGENTLENS_HEALTH_CHECK_ENABLED` | `true` | Enable health checking |
| `AGENTLENS_HEALTH_CHECK_INTERVAL` | `30s` | Health check interval |
| `AGENTLENS_HEALTH_CHECK_TIMEOUT` | `5s` | Health check timeout |
| `AGENTLENS_HEALTH_CHECK_CONCURRENCY` | `10` | Health check parallelism |

---

## Documentation

| Document | Description |
| -------- | ----------- |
| [API Reference](docs/api.md) | Full REST API documentation — endpoints, request/response schemas, error codes |
| [Architecture](docs/architecture.md) | Microkernel design, plugin system, data flow diagrams |
| [Authentication](docs/auth.md) | JWT auth, RBAC, roles & permissions, account lockout |
| [Database](docs/database.md) | SQLite & PostgreSQL setup, migrations, dialect differences |
| [Settings](docs/settings.md) | Application settings, categories, configuration store |
| [Developer Guide](docs/developer-guide.md) | Build, test, lint, write plugins, frontend development |
| [DevOps Guide](docs/devops-guide.md) | Docker, Helm, CI/CD, release pipeline, versioning, troubleshooting |
| [End-User Guide](docs/end-user-guide.md) | UI walkthrough with screenshots — catalog, agents, filtering |
| [User Guide](docs/user-guide.md) | Configuration, deployment modes, Kubernetes annotations |
| [Contributing](CONTRIBUTING.md) | How to contribute, project structure, code style |

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

AgentLens is licensed under the [Business Source License 1.1](LICENSE).

**What this means in plain language:**

- ✅ You can use AgentLens freely for any purpose, including production and internal business use
- ✅ You can modify, fork, and create derivative works
- ✅ You can redistribute the source code
- ❌ You may **not** offer AgentLens as a commercial hosted or managed service competing with AgentLens commercial offerings
- 🔄 Each release automatically converts to **Apache License 2.0** after four years

For commercial hosting/managed service licensing, contact the Licensor.
