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
| `POST` | `/api/v1/catalog` | Push-register a catalog entry |
| `GET` | `/api/v1/catalog/{id}` | Get entry by ID |
| `DELETE` | `/api/v1/catalog/{id}` | Delete entry |
| `GET` | `/api/v1/catalog/{id}/card` | Get raw protocol card JSON |
| `GET` | `/api/v1/skills?q=` | Search entries by skill name |
| `GET` | `/api/v1/stats` | Aggregate stats |

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
