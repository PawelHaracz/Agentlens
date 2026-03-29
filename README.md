# AgentLens

**Real-time AI agent catalog for Kubernetes — discover, track, and inspect A2A, MCP, and A2UI agents across your cluster.**

AgentLens automatically discovers AI agents running in Kubernetes (via pod annotations), polls static endpoints, and accepts push registrations. It exposes a REST API and a web dashboard for browsing the catalog, filtering by protocol/status, and inspecting agent cards and skills.

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

AgentLens will start watching pods across all namespaces for agent annotations.

---

## Kubernetes Annotations

Annotate your pods to register agents automatically:

| Annotation | Required | Description |
|---|---|---|
| `agentlens/enabled` | ✓ | Set to `"true"` to register the pod as an agent |
| `agentlens/protocol` | ✓ | One of `a2a`, `mcp`, `a2ui` |
| `agentlens/name` | | Human-readable name (defaults to pod name) |
| `agentlens/description` | | Short description of the agent |
| `agentlens/endpoint` | | Override endpoint URL (defaults to `http://<podIP>:<port>`) |
| `agentlens/port` | | Container port to use for the endpoint |
| `agentlens/team` | | Owning team label |
| `agentlens/tags` | | Comma-separated tags |

**Example:**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-agent
  annotations:
    agentlens/enabled: "true"
    agentlens/protocol: "a2a"
    agentlens/name: "My AI Agent"
    agentlens/description: "Handles customer support queries"
    agentlens/team: "platform"
    agentlens/tags: "nlp,support"
spec:
  containers:
    - name: agent
      image: my-org/my-agent:latest
      ports:
        - containerPort: 8080
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

Register an agent via HTTP POST:

```bash
curl -X POST http://localhost:8080/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-agent",
    "description": "Does amazing things",
    "protocol": "a2a",
    "endpoint": "http://my-agent.internal:8080",
    "version": "1.2.3",
    "team": "platform",
    "tags": ["nlp", "demo"]
  }'
```

---

## API Reference

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check |
| `GET` | `/api/v1/agents` | List agents (supports `?protocol=`, `?status=`, `?q=`, `?team=`, `?tags=`, `?limit=`, `?offset=`) |
| `POST` | `/api/v1/agents` | Register an agent (push) |
| `GET` | `/api/v1/agents/{id}` | Get agent by ID |
| `DELETE` | `/api/v1/agents/{id}` | Delete agent |
| `GET` | `/api/v1/agents/{id}/card` | Get raw agent card JSON |
| `GET` | `/api/v1/skills?q=` | Search agents by skill name |
| `GET` | `/api/v1/stats` | Aggregate stats |

See [docs/api.md](docs/api.md) for full API documentation.

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
