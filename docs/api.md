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

## Catalog

### `GET /api/v1/catalog`

List all catalog entries with optional filtering.

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `q` | string | Full-text search on display name, description |
| `protocol` | string | Filter by protocol: `a2a`, `mcp`, `a2ui` |
| `status` | string | Filter by status: `healthy`, `degraded`, `down`, `unknown` |
| `source` | string | Filter by source: `k8s`, `config`, `push`, `upstream` |
| `team` | string | Filter by provider team name |
| `categories` | string | Comma-separated category filter |
| `limit` | int | Maximum results to return (default: no limit) |
| `offset` | int | Pagination offset |

**Response 200:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "display_name": "my-agent",
    "description": "Handles customer support",
    "protocol": "a2a",
    "endpoint": "http://my-agent.default.svc:8080",
    "version": "1.0.0",
    "status": "healthy",
    "source": "k8s",
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
    ],
    "validity": {
      "last_seen": "2024-01-15T10:30:00Z"
    },
    "metadata": {
      "kubernetes.namespace": "default"
    },
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### `POST /api/v1/catalog`

Register a catalog entry via push.

**Request Body:**
```json
{
  "display_name": "my-agent",
  "description": "Does amazing things",
  "protocol": "a2a",
  "endpoint": "http://my-agent.internal:8080",
  "version": "1.2.3",
  "provider": {
    "organization": "Acme Corp",
    "team": "platform"
  },
  "categories": ["nlp"],
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

**Response 201:** Returns the created catalog entry with generated `id`.

**Response 400:** Invalid request body.

---

### `GET /api/v1/catalog/{id}`

Get a specific catalog entry by ID.

**Path Parameters:**
- `id` — Entry UUID

**Response 200:** Catalog entry object.

**Response 404:**
```json
{"error": "catalog entry not found"}
```

---

### `DELETE /api/v1/catalog/{id}`

Delete an entry from the catalog.

**Response 204:** No content.

**Response 404:** Entry not found.

---

### `GET /api/v1/catalog/{id}/card`

Get the raw protocol card JSON (A2A or MCP card fetched from the agent).

**Response 200:** Raw JSON card (content varies by protocol).

**Response 404:** Entry or card not found.

---

## Skills

### `GET /api/v1/skills`

Search catalog entries by skill name.

**Query Parameters:**

| Parameter | Type | Description |
|---|---|---|
| `q` | string | Skill name search query |

**Response 200:** Array of CatalogEntry objects that have matching skills.

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
| `k8s` | Discovered via Kubernetes Service annotations |
| `config` | Registered via static config file |
| `push` | Self-registered via `POST /api/v1/catalog` |
| `upstream` | Crawled from an upstream registry |
