# AgentLens API Documentation

Base URL: `http://<host>:8080`

All API responses use JSON (`Content-Type: application/json`).

---

## Health Check

### `GET /healthz`

Returns server health status.

**Response 200:**
```json
{"status": "ok"}
```

---

## Agents

### `GET /api/v1/agents`

List all registered agents with optional filtering.

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `q` | string | Full-text search on name, description |
| `protocol` | string | Filter by protocol: `a2a`, `mcp`, `a2ui` |
| `status` | string | Filter by status: `healthy`, `degraded`, `down`, `unknown` |
| `source` | string | Filter by source: `k8s`, `config`, `push`, `upstream` |
| `team` | string | Filter by team name |
| `tags` | string | Comma-separated tag filter |
| `limit` | int | Maximum results to return (default: no limit) |
| `offset` | int | Pagination offset |

**Response 200:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "my-agent",
    "description": "Handles customer support",
    "protocol": "a2a",
    "endpoint": "http://my-agent.default.svc:8080",
    "version": "1.0.0",
    "status": "healthy",
    "source": "k8s",
    "namespace": "default",
    "team": "platform",
    "tags": ["nlp", "support"],
    "skills": [
      {
        "name": "answer_question",
        "description": "Answers user questions",
        "input_modes": ["text"],
        "output_modes": ["text"]
      }
    ],
    "last_seen": "2024-01-15T10:30:00Z",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### `POST /api/v1/agents`

Register an agent via push.

**Request Body:**
```json
{
  "name": "my-agent",
  "description": "Does amazing things",
  "protocol": "a2a",
  "endpoint": "http://my-agent.internal:8080",
  "version": "1.2.3",
  "team": "platform",
  "tags": ["nlp"],
  "skills": [
    {
      "name": "chat",
      "description": "Chat with the agent",
      "input_modes": ["text"],
      "output_modes": ["text"]
    }
  ]
}
```

**Response 201:** Returns the created agent object with generated `id`.

**Response 400:** Invalid request body.

---

### `GET /api/v1/agents/{id}`

Get a specific agent by ID.

**Path Parameters:**
- `id` — Agent UUID

**Response 200:** Agent object.

**Response 404:**
```json
{"error": "agent not found"}
```

---

### `DELETE /api/v1/agents/{id}`

Delete an agent from the catalog.

**Response 204:** No content.

**Response 404:** Agent not found.

---

### `GET /api/v1/agents/{id}/card`

Get the raw agent card JSON (A2A or MCP card fetched from the agent).

**Response 200:** Raw JSON card (content varies by protocol).

**Response 404:** Agent or card not found.

---

## Skills

### `GET /api/v1/skills`

Search agents by skill name.

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `q` | string | Skill name search query |

**Response 200:** Array of Agent objects that have matching skills.

---

## Stats

### `GET /api/v1/stats`

Get aggregate statistics about the catalog.

**Response 200:**
```json
{
  "total": 42,
  "by_status": {
    "healthy": 35,
    "degraded": 4,
    "down": 2,
    "unknown": 1
  },
  "by_source": {
    "k8s": 30,
    "config": 8,
    "push": 4
  }
}
```

---

## Error Responses

All errors return JSON with an `error` field:

```json
{"error": "descriptive error message"}
```

| Status | Meaning |
|---|---|
| `400` | Bad request / invalid body |
| `404` | Resource not found |
| `500` | Internal server error |

---

## Protocols

| Value | Description |
|---|---|
| `a2a` | Agent-to-Agent protocol — card at `/.well-known/agent-card.json` |
| `mcp` | Model Context Protocol — card at `/.well-known/mcp/server.json` |
| `a2ui` | Agent-to-UI protocol |

## Source Types

| Value | Description |
|---|---|
| `k8s` | Discovered via Kubernetes pod annotations |
| `config` | Registered via static config file |
| `push` | Self-registered via `POST /api/v1/agents` |
| `upstream` | Crawled from an upstream registry |
