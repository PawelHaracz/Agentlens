# User Guide

This guide walks you through installing, configuring, and using AgentLens — a service discovery platform for AI agents supporting A2A, MCP, and A2UI protocols.

---

## Table of Contents

- [Installation](#installation)
- [Configuration](#configuration)
- [Running AgentLens](#running-agentlens)
- [Web Dashboard](#web-dashboard)
- [Registering Agents](#registering-agents)
- [Kubernetes Discovery](#kubernetes-discovery)
- [Health Checks](#health-checks)
- [API Usage](#api-usage)

---

## Installation

### Docker (Recommended)

```bash
docker pull ghcr.io/pawelharacz/agentlens:latest
docker run -p 8080:8080 ghcr.io/pawelharacz/agentlens:latest
```

### Docker Compose (with mock agents)

```bash
git clone https://github.com/PawelHaracz/agentlens
cd agentlens/examples
docker compose up
```

This starts AgentLens along with mock A2A and MCP agents for demonstration.

### Helm (Kubernetes)

```bash
helm install agentlens ./deploy/helm/agentlens \
  --namespace agentlens --create-namespace
```

### Build from Source

Prerequisites: Go 1.23+, Node.js 20+

```bash
git clone https://github.com/PawelHaracz/agentlens
cd agentlens
make deps
make web-install
make web-build
make build
./bin/agentlens
```

---

## Configuration

AgentLens is configured via a YAML file and/or environment variables. Environment variables are prefixed with `AGENTLENS_` and override file settings.

### Configuration File

Create a file named `agentlens.yaml`:

```yaml
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

kubernetes:
  enabled: false
  namespaces:
    - default
    - agents

health_check:
  enabled: true
  interval: 30s
  timeout: 5s
  concurrency: 10
```

Run with the config file:

```bash
agentlens --config agentlens.yaml
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENTLENS_PORT` | `8080` | HTTP server port |
| `AGENTLENS_DATA_DIR` | `./data` | Directory for SQLite database |
| `AGENTLENS_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `AGENTLENS_LICENSE_KEY` | — | Enterprise license key (optional) |
| `AGENTLENS_POLL_INTERVAL` | `5m` | How often to poll discovery sources |
| `AGENTLENS_KUBERNETES_ENABLED` | `false` | Enable Kubernetes service discovery |
| `AGENTLENS_HEALTH_CHECK_ENABLED` | `true` | Enable periodic health checks |
| `AGENTLENS_HEALTH_CHECK_INTERVAL` | `30s` | Time between health checks |
| `AGENTLENS_HEALTH_CHECK_TIMEOUT` | `5s` | Timeout per health check request |
| `AGENTLENS_HEALTH_CHECK_CONCURRENCY` | `10` | Max parallel health checks |

### CLI Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to configuration file |
| `--port` | HTTP port (overrides config and env) |

---

## Running AgentLens

### Minimal (no config)

```bash
agentlens
```

Starts on port 8080 with health checks enabled and an empty catalog. Agents can be registered via the push API.

### With Static Sources

```bash
agentlens --config agentlens.yaml
```

AgentLens polls the configured sources at the specified interval and populates the catalog.

### With Kubernetes Discovery

Set `AGENTLENS_KUBERNETES_ENABLED=true` or configure in YAML:

```yaml
kubernetes:
  enabled: true
  namespaces:
    - default
    - agents
```

AgentLens will watch Kubernetes Services for agent annotations and discover agents automatically.

---

## Web Dashboard

Open `http://localhost:8080` in your browser to access the dashboard.

### Features

- **Catalog view** — browse all discovered agents and MCP servers in a table
- **Protocol badges** — color-coded badges for A2A, MCP, A2UI protocols
- **Status indicators** — health status (healthy/degraded/down/unknown)
- **Search** — full-text search across agent names and descriptions
- **Filters** — filter by protocol, status, source type, or team
- **Detail view** — click an entry to see skills, metadata, provider info, and the raw protocol card JSON
- **Stats bar** — aggregate counts by status and discovery source

---

## Registering Agents

### Automatic Discovery (Static Config)

Add entries under `sources:` in your config file:

```yaml
sources:
  - name: support-agent
    type: a2a
    url: http://support-agent.internal:8080
  - name: code-tools
    type: mcp
    url: http://mcp-tools.internal:9000
```

AgentLens will fetch the agent card from the well-known path:
- A2A: `{url}/.well-known/agent-card.json`
- MCP: `{url}/.well-known/mcp/server.json`

### Automatic Discovery (Kubernetes)

Annotate your Kubernetes Services:

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

| Annotation | Required | Description |
|------------|----------|-------------|
| `agentlens.io/type` | Yes | Protocol: `a2a`, `mcp`, `a2ui` |
| `agentlens.io/card-path` | No | Custom card path (defaults to well-known paths) |
| `agentlens.io/team` | No | Owning team label |
| `agentlens.io/tags` | No | Comma-separated categories |

### Push Registration (HTTP API)

Register an agent directly via POST:

```bash
curl -X POST http://localhost:8080/api/v1/catalog \
  -H "Content-Type: application/json" \
  -d '{
    "display_name": "my-agent",
    "description": "Handles customer support",
    "protocol": "a2a",
    "endpoint": "http://my-agent.internal:8080",
    "version": "1.0.0",
    "provider": {
      "organization": "Acme Corp",
      "team": "platform"
    },
    "categories": ["nlp", "support"],
    "skills": [
      {
        "name": "answer_question",
        "description": "Answers user questions",
        "input_modes": ["text"],
        "output_modes": ["text"]
      }
    ]
  }'
```

---

## Kubernetes Discovery

When running in Kubernetes with discovery enabled, AgentLens:

1. Lists Services in the configured namespaces
2. Looks for `agentlens.io/type` annotations
3. Constructs the card URL from the Service endpoint + card path
4. Fetches and parses the card using the appropriate parser plugin
5. Upserts the entry into the catalog (keyed by endpoint)

### RBAC Requirements

The Helm chart creates a ClusterRole with permissions to list/watch Services across namespaces. If deploying manually, ensure the ServiceAccount has:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
rules:
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["get", "list", "watch"]
```

---

## Health Checks

When enabled, AgentLens periodically pings each catalog entry's endpoint and updates its status:

| Status | Meaning |
|--------|---------|
| `healthy` | Endpoint responded successfully |
| `degraded` | Endpoint responded with errors or slow response |
| `down` | Endpoint unreachable or returned server error |
| `unknown` | Not yet checked |

### Configuration

```yaml
health_check:
  enabled: true
  interval: 30s    # Time between check cycles
  timeout: 5s      # Per-endpoint timeout
  concurrency: 10  # Max parallel checks
```

---

## API Usage

### List All Entries

```bash
curl http://localhost:8080/api/v1/catalog
```

### Filter by Protocol

```bash
curl "http://localhost:8080/api/v1/catalog?protocol=a2a"
```

### Search by Name

```bash
curl "http://localhost:8080/api/v1/catalog?q=support"
```

### Search by Skill

```bash
curl "http://localhost:8080/api/v1/skills?q=answer"
```

### Get Entry Details

```bash
curl http://localhost:8080/api/v1/catalog/{id}
```

### Get Raw Protocol Card

```bash
curl http://localhost:8080/api/v1/catalog/{id}/card
```

### Get Statistics

```bash
curl http://localhost:8080/api/v1/stats
```

### Delete an Entry

```bash
curl -X DELETE http://localhost:8080/api/v1/catalog/{id}
```

See [api.md](api.md) for the complete API reference.

---

## Troubleshooting

### Agent not appearing in catalog

1. Check the agent's card endpoint is reachable from AgentLens
2. Verify the card JSON matches the expected format (A2A requires `url` field; MCP requires `remotes` array)
3. Check AgentLens logs for parsing errors: `docker logs agentlens`
4. Ensure the `poll_interval` has elapsed since the source was added

### Health check showing "down"

1. Verify the agent's endpoint is accessible from AgentLens
2. Check if the agent requires authentication (not supported in health checks by default)
3. Increase `health_check.timeout` if the agent is slow to respond

### Kubernetes agents not discovered

1. Verify `kubernetes.enabled: true` in config
2. Check the Service has `agentlens.io/type` annotation
3. Ensure the ServiceAccount has ClusterRole permissions to list Services
4. Verify the configured namespaces match where Services are deployed
