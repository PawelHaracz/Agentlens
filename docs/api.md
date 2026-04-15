# AgentLens API Documentation

Base URL: `http://<host>:8080`

All API responses use JSON (`Content-Type: application/json`).

---

## Health Check

### `GET /healthz`

Liveness probe. Returns server health status (process alive).

**Response 200:**
```json
{"status": "ok"}
```

**Authentication:** None

---

### `GET /readyz`

Readiness probe. Returns server readiness status including database reachability check. Required before routing traffic.

**Response 200 (Ready):**
```json
{"status": "ok"}
```

**Response 503 (Not ready):**
```json
{"status": "error", "reason": "database unreachable"}
```

**Authentication:** None

---

### `GET /metrics`

Prometheus exposition format metrics endpoint. Only available when `telemetry.prometheus.enabled=true` in configuration.

**Response 200 (Prometheus text format):**
```
# HELP go_goroutines Number of goroutines that currently exist
# TYPE go_goroutines gauge
go_goroutines 12
...
```

**Response 404:** If Prometheus metrics are disabled.

**Authentication:** None

---

### `GET /api/v1/telemetry/config`

Frontend telemetry configuration. Used by web UI to initialize OpenTelemetry client-side tracing.

**Response 200:**
```json
{
  "enabled": true,
  "endpoint": "http://otel-collector:4318/v1/traces",
  "serviceName": "agentlens-web"
}
```

When telemetry is disabled, `enabled` is `false` and `endpoint`/`serviceName` are omitted.

**Authentication:** None

---

## Catalog

### `GET /api/v1/catalog`

List all catalog entries with optional filtering.

**Query Parameters:**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `q` | string | — | Full-text search across `display_name`, `description`, `capabilities.name`, `capabilities.description`, `categories`, `provider.organization`. |
| `sort` | string | `lastSuccessAt_desc` | Sort order. Values: `lastSuccessAt_desc`, `displayName_asc`, `createdAt_desc`. Unknown values return `400`. |
| `protocol` | string | — | Filter by protocol: `a2a`, `mcp`, `a2ui` |
| `status` | string | — | Filter by lifecycle status: `registered`, `active`, `degraded`, `offline`, `deprecated` |
| `source` | string | — | Filter by source: `k8s`, `config`, `push`, `upstream` |
| `team` | string | — | Filter by provider team name |
| `categories` | string | — | Comma-separated category filter |
| `project` | string | — | Filter to entries belonging to this project party ID |
| `limit` | int | — | Maximum results to return (default: no limit) |
| `offset` | int | — | Pagination offset |

**Response 200:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "agent_type_id": "66b8c3a2-1111-4b92-b000-000000000001",
    "display_name": "my-agent",
    "description": "Handles customer support",
    "protocol": "a2a",
    "endpoint": "http://my-agent.default.svc:8080",
    "version": "1.0.0",
    "spec_version": "1.0",
    "status": "healthy",
    "source": "k8s",
    "provider": {
      "organization": "Acme Corp",
      "team": "platform"
    },
    "categories": ["nlp", "support"],
    "capabilities": [
      {
        "kind": "a2a.skill",
        "name": "answer_question",
        "description": "Answers user questions",
        "properties": {"input_modes": ["text"], "output_modes": ["text"]}
      },
      {
        "kind": "a2a.security_scheme",
        "name": "bearer",
        "description": "",
        "properties": {"type": "bearer", "method": "header"}
      }
    ],
    "validity": {
      "last_seen": "2024-01-15T10:30:00Z"
    },
    "metadata": {
      "kubernetes.namespace": "default"
    },
    "raw_definition": {"name": "my-agent", "...": "..."},
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### `POST /api/v1/catalog/validate`

Validate an agent card without registering it (dry-run only — does not persist anything). This endpoint auto-detects the A2A specification version (v0.3 vs v1.0) and returns structured validation results (`kernel.ValidationResult`) with a preview of the agent details.

**Authentication:** Required. Requires `catalog:write` permission.

**Request Body:** Raw agent card JSON (any valid JSON is accepted).

**Request Headers:**
```
Content-Type: application/json
```

**Response 200 (Valid card):**
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
    "skills_count": 2,
    "extensions_count": 1,
    "security_schemes": ["oauth2"],
    "interfaces": ["jsonrpc"]
  }
}
```

The `preview` field is a generic key-value map (`map[string]any`) whose contents vary by protocol:

| Protocol | Preview fields |
|----------|---------------|
| `a2a` | `display_name`, `description`, `protocol`, `spec_version`, `skills_count`, `extensions_count`, `security_schemes`, `interfaces` |
| `mcp` | `display_name`, `description`, `protocol`, `tools_count` |

**Response 422 (Invalid card):**
```json
{
  "valid": false,
  "spec_version": "",
  "errors": [
    {
      "field": "url",
      "message": "url or supportedInterfaces is required"
    }
  ],
  "warnings": [],
  "preview": null
}
```

**Error Field Descriptions:**

| Field | Meaning |
|-------|---------|
| `name` | Agent display name is required |
| `url` | Either `url` or `supportedInterfaces[].url` is required |
| `version` | Invalid semantic version format |
| `securitySchemes` | Invalid security scheme structure |
| `extensions` | Invalid extension structure |

**A2A Specification Version Detection:**

The endpoint automatically detects the A2A spec version based on the card structure:

- **v0.3** — if `supportsExtendedAgentCard` is at root level
- **v1.0** — if `supportsExtendedAgentCard` is nested inside `capabilities` object
- **empty** — if neither structure is present

---

### `POST /api/v1/catalog/register`

Register an A2A agent from a raw agent card JSON. The endpoint validates the card, parses it via the A2A parser (raw card to CatalogEntry), and persists the entry.

**Authentication:** Required. Requires `catalog:write` permission.

**Request Body:** Raw A2A agent card JSON.

**Request Headers:**
```
Content-Type: application/json
```

**Response 201 (Created):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "agent_type_id": "66b8c3a2-1111-4b92-b000-000000000001",
  "display_name": "Example Chat Agent",
  "description": "A sample agent demonstrating A2A v1.0 features",
  "protocol": "a2a",
  "endpoint": "https://api.example.com/v1",
  "version": "1.0.0",
  "spec_version": "1.0",
  "status": "unknown",
  "source": "push",
  "capabilities": [
    {
      "kind": "a2a.skill",
      "name": "chat",
      "description": "Chat with the agent",
      "properties": {"input_modes": ["text"], "output_modes": ["text"]}
    },
    {
      "kind": "a2a.extension",
      "name": "urn:example:ext",
      "description": "",
      "properties": {"uri": "urn:example:ext", "required": true}
    },
    {
      "kind": "a2a.security_scheme",
      "name": "oauth2",
      "description": "",
      "properties": {"type": "oauth2"}
    },
    {
      "kind": "a2a.interface",
      "name": "https://api.example.com/v1",
      "description": "",
      "properties": {"url": "https://api.example.com/v1", "binding": "jsonrpc"}
    }
  ],
  "provider": {
    "organization": "Example Corp"
  },
  "categories": [],
  "raw_definition": {"name": "Example Chat Agent", "...": "..."},
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Response 400 (Bad Request):**
```json
{"error": "request body is empty"}
```

**Response 409 (Conflict):**
```json
{"error": "catalog entry with this endpoint already exists"}
```

**Response 422 (Unprocessable Entity):**

Returns the same `kernel.ValidationResult` structure as `/catalog/validate` when the card fails validation:

```json
{
  "valid": false,
  "spec_version": "",
  "errors": [
    {
      "field": "name",
      "message": "name is required"
    }
  ],
  "warnings": [],
  "preview": null
}
```

---

### `POST /api/v1/catalog/import`

Import an agent card by fetching it from a URL. The server fetches the card from the provided URL, auto-detects (or uses the specified) protocol, parses the card, and registers the entry.

**Authentication:** Required. Requires `catalog:write` permission.

**Request Body:**
```json
{
  "url": "https://my-agent.example.com/.well-known/agent.json",
  "protocol": "a2a"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | ✅ | HTTPS or HTTP URL to fetch the agent card from |
| `protocol` | string | ❌ | Force a protocol: `a2a` or `mcp`. Omit for auto-detection. (`a2ui` is not supported for import — use `POST /api/v1/catalog` for direct creation) |

**Auto-detection rules (when `protocol` is omitted):**

| Signal | Detected protocol |
|--------|-------------------|
| URL contains `/.well-known/agent` | `a2a` |
| URL contains `/mcp` | `mcp` |
| Card JSON has `"skills"` array | `a2a` |
| Card JSON has `"tools"` array | `mcp` |
| None of the above | Error — must specify `protocol` |

**Safety restrictions:**

- URL scheme must be `http` or `https` (no `file://`, `ftp://`, etc.)
- URLs that resolve to private/internal networks are rejected: `127.x`, `10.x`, `172.16–31.x`, `192.168.x`, `169.254.x`, `::1`, `fc00::/7`, `fe80::/10`, and `localhost`
- Response body is capped at 1 MB
- HTTP timeout: 10 seconds
- Maximum 3 redirects followed

**Response 201 (Created):** Same as `POST /api/v1/catalog/register` — the created `CatalogEntry` object.

**Response 400 (Bad Request):**
```json
{"error": "url scheme must be http or https"}
```
```json
{"error": "url resolves to a private or reserved address"}
```
```json
{"error": "could not detect protocol; specify 'protocol' in the request body"}
```

**Response 409 (Conflict):**
```json
{"error": "an entry with this endpoint already exists"}
```

**Response 422 (Unprocessable Entity):**

Returned when the fetched JSON is not a valid agent card:
```json
{
  "valid": false,
  "spec_version": "",
  "errors": [
    {"field": "name", "message": "name is required"}
  ],
  "warnings": [],
  "preview": null
}
```

**Response 502 (Bad Gateway):**
```json
{"error": "could not fetch card from url: fetching url: connection refused"}
```

Returned when the remote URL is unreachable, returns a non-2xx HTTP status, or returns non-JSON content.

**Example — import an A2A agent card:**
```bash
curl -X POST http://localhost:8080/api/v1/catalog/import \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://my-agent.example.com/.well-known/agent.json"}'
```

**Example — import an MCP server card with explicit protocol:**
```bash
curl -X POST http://localhost:8080/api/v1/catalog/import \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://my-mcp-server.example.com/card", "protocol": "mcp"}'
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
  "version": "1.2.3"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `display_name` | string | ✅ | Human-readable name for the entry |
| `description` | string | ❌ | Optional description |
| `protocol` | string | ✅ | One of: `a2a`, `mcp`, `a2ui` |
| `endpoint` | string | ✅ | Agent endpoint URL (must be unique) |
| `version` | string | ❌ | Agent version string |

This endpoint creates a minimal catalog entry (no capabilities). To register with a full agent card including capabilities, use `POST /api/v1/catalog/register` instead.

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

**Response headers:**
- `Content-Type` — MIME type as stored in the card store
- `X-Raw-Card-Fetched-At` — ISO 8601 timestamp indicating when the card was last fetched
- `ETag` — weak entity tag (e.g., `W/"abc123"`) for conditional GET support
- `X-Raw-Card-Truncated: true` — present only when the card was truncated at the 256 KiB storage cap; the returned bytes are incomplete

**Conditional GET:** include `If-None-Match` with a previously received `ETag` value; if the card has not changed, the server returns `304 Not Modified` with no body.

**Response 200:** Raw JSON card (content varies by protocol).

**Response 304:** Not Modified (ETag matched — card unchanged since last fetch).

**Response 404:** Entry not found, or entry exists but no card has been stored (e.g., manually created entries):
```json
{"error": "no raw card stored"}
```

---

### Health check fields

All catalog entry responses now include a `health` object:

```json
{
  "status": "active",
  "health": {
    "state": "active",
    "lastProbedAt": "2026-04-07T11:42:13Z",
    "lastSuccessAt": "2026-04-07T11:42:13Z",
    "latencyMs": 142,
    "consecutiveFailures": 0,
    "lastError": ""
  }
}
```

The `status` field is a backward-compatible alias for `health.state`.

**Lifecycle states:** `registered` (not yet probed) · `active` · `degraded` · `offline` · `deprecated`

### Authentication fields

A2A catalog entries include two computed authentication fields derived from their stored capabilities at serialization time. These fields are never stored directly — they are always computed fresh from the `a2a.security_scheme` and `a2a.security_requirement` capabilities.

#### `auth_summary` (list and detail responses)

Present on all A2A entries that declare at least one security scheme. Absent (`omitempty`) for open agents and all MCP entries.

```json
{
  "auth_summary": {
    "types": ["http:Bearer", "apiKey"],
    "label": "Bearer JWT + API Key",
    "required": true
  }
}
```

| Field | Type | Description |
|---|---|---|
| `types` | `string[]` | Normalized scheme type strings. `http` schemes are qualified as `http:Bearer`, `http:Basic`, etc. |
| `label` | `string` | Human-readable summary (max 40 chars, truncated with `…` if longer). |
| `required` | `bool` | `true` when at least one top-level (non-skill) `a2a.security_requirement` exists. |

#### `security_detail` (detail response only — `GET /api/v1/catalog/{id}`)

Present when an A2A entry has security schemes. Contains the full scheme and requirement data for use in connection UIs.

```json
{
  "security_detail": {
    "security_schemes": [
      {
        "scheme_name": "httpAuth",
        "type": "http",
        "description": "JWT Bearer token",
        "http_scheme": "Bearer",
        "bearer_format": "JWT"
      },
      {
        "scheme_name": "apiKeyAuth",
        "type": "apiKey",
        "api_key_location": "header",
        "api_key_name": "X-API-Key"
      }
    ],
    "security_requirements": [
      { "schemes": { "httpAuth": [] } },
      { "schemes": { "apiKeyAuth": [] } }
    ]
  }
}
```

**`security_schemes` item fields:**

| Field | Type | Scheme types | Description |
|---|---|---|---|
| `scheme_name` | `string` | all | Key from the agent card's `securitySchemes` map. |
| `type` | `string` | all | `http` · `apiKey` · `oauth2` · `openIdConnect` · `mutualTls` |
| `description` | `string` | all | Optional human-readable description. |
| `http_scheme` | `string` | `http` | `Bearer` · `Basic` · `Digest` |
| `bearer_format` | `string` | `http` | e.g. `JWT` |
| `api_key_location` | `string` | `apiKey` | `header` · `query` · `cookie` |
| `api_key_name` | `string` | `apiKey` | e.g. `X-API-Key` |
| `oauth_flows` | `object[]` | `oauth2` | Array of flow objects (see below). |
| `oauth2_metadata_url` | `string` | `oauth2` | OAuth 2.0 server metadata URL. |
| `openid_connect_url` | `string` | `openIdConnect` | OIDC discovery document URL. |

**`oauth_flows` item fields:**

| Field | Type | Description |
|---|---|---|
| `flow_type` | `string` | `authorizationCode` · `clientCredentials` · `deviceCode` · `implicit` · `password` (last two have `deprecated: true` and are filtered from the UI) |
| `authorization_url` | `string` | Authorization endpoint (authorizationCode). |
| `token_url` | `string` | Token endpoint. |
| `device_auth_url` | `string` | Device authorization endpoint (deviceCode). |
| `scopes` | `object` | Map of scope name → description. |
| `deprecated` | `bool` | `true` for legacy flows (implicit, password). |

**`security_requirements` item fields:**

| Field | Type | Description |
|---|---|---|
| `schemes` | `object` | Map of scheme name → required scopes (empty array = any scope). |
| `skill_ref` | `string` | Non-empty when this requirement applies to a specific skill only. |

Multiple `security_requirements` entries are OR'd: the client must satisfy at least one complete entry.

### GET /api/v1/catalog

**New query parameter:** `?state=` — comma-separated lifecycle states filter. Example: `?state=active,degraded`

Returns 400 if an unknown state value is provided.

### PATCH /api/v1/catalog/{id}/lifecycle

Update the lifecycle state of a catalog entry.

**Permission:** `catalog:write` (editor or admin)

**Request body:**
```json
{ "state": "deprecated" }
```

**Allowed values:** `deprecated`, `active` (to un-deprecate)

**Response:** Updated catalog entry (200), 400 for invalid state, 404 for missing entry.

### POST /api/v1/catalog/{id}/probe

Trigger an immediate health probe for a catalog entry.

**Permission:** `catalog:write` (editor or admin)

**Rate limit:** 1 request per entry per 5 seconds (429 if exceeded)

**Response:**
```json
{
  "state": "active",
  "lastProbedAt": "2026-04-07T11:42:13Z",
  "lastSuccessAt": "2026-04-07T11:42:13Z",
  "latencyMs": 142,
  "consecutiveFailures": 0,
  "lastError": ""
}
```

Returns 503 if health monitoring is disabled.

---

## Capabilities

### GET /api/v1/capabilities

List capability instances (one per agent per capability) with agent metadata.

**Query Parameters:**
- `q` (string, optional): Case-insensitive substring search on capability name, description, and properties
- `kind` (string, optional): Filter by capability kind (e.g., `a2a.skill`, `mcp.tool`). Must be a discoverable kind.
- `limit` (integer, optional, default 50): Max results per page
- `offset` (integer, optional, default 0): Pagination offset
- `sort` (string, optional, default `name_asc`): Sort order. Values: `name_asc`, `agentName_asc`

**Permission:** `catalog:read`

**Response:** 200 OK

```json
{
  "total": 3,
  "items": [
    {
      "kind": "a2a.skill",
      "name": "Translate EN-DE",
      "description": "Bidirectional translation",
      "tags": ["translation", "german"],
      "input_modes": ["text/plain"],
      "output_modes": ["text/plain"],
      "agent_id": "entry-uuid-1",
      "agent_name": "Translation Agent",
      "protocol": "a2a",
      "status": "active",
      "spec_version": "1.0",
      "provider_org": "Acme",
      "provider_url": "https://acme.io",
      "health_state": "active",
      "latency_ms": 142
    }
  ]
}
```

**Error Responses:**
- 400 Bad Request: Invalid `kind` or `sort` parameter

---

### GET /api/v1/capabilities/{key}

Get all agents offering a specific capability.

**Path Parameters:**
- `key` (string, required): Capability identifier in format `kind::name` (URL-encoded). Example: `a2a.skill::Translate%20EN-DE`

**Permission:** `catalog:read`

**Response:** 200 OK

```json
{
  "capability": {
    "kind": "a2a.skill",
    "name": "Translate EN-DE"
  },
  "agents": [
    {
      "id": "entry-uuid-1",
      "display_name": "Translation Agent",
      "protocol": "a2a",
      "provider": { "organization": "Acme", "url": "https://acme.io" },
      "health": { "state": "active", "latencyMs": 142 },
      "spec_version": "1.0",
      "status": "active",
      "capability_snippet": {
        "kind": "a2a.skill",
        "name": "Translate EN-DE",
        "description": "Bidirectional translation",
        "tags": ["translation"],
        "inputModes": ["text/plain"],
        "outputModes": ["text/plain"]
      }
    }
  ]
}
```

**Error Responses:**
- 400 Bad Request: Malformed key (missing `::` separator)
- 404 Not Found: No agents offer this capability

---

### ~~GET /api/v1/skills~~ (REMOVED)

**Breaking Change:** This endpoint has been removed in favor of `/api/v1/capabilities`. See CHANGELOG.

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
| `401` | Unauthorized — missing or invalid JWT token |
| `403` | Forbidden — insufficient permissions |
| `404` | Resource not found |
| `409` | Conflict — resource already exists |
| `422` | Unprocessable Entity — validation failed |
| `423` | Locked — account locked due to failed login attempts |
| `500` | Internal server error |
| `502` | Bad Gateway — remote URL unreachable or returned non-JSON (import only) |

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

---

## Authentication

All endpoints except `GET /healthz` and `POST /api/v1/auth/login` require a valid JWT token in the `Authorization` header:

```
Authorization: Bearer <token>
```

### `POST /api/v1/auth/login`

Authenticate and obtain a JWT token. **No auth required.**

**Request Body:**
```json
{
  "username": "admin",
  "password": "your-password"
}
```

**Response 200:**
```json
{
  "token": "eyJhbGciOi...",
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "username": "admin",
    "role": "admin"
  }
}
```

**Response 401:** Invalid credentials.

**Response 423:** Account locked.

---

### `POST /api/v1/auth/logout`

Invalidate the current JWT token. **Requires auth.**

**Response 200:**
```json
{"message": "logged out"}
```

---

### `POST /api/v1/auth/refresh`

Refresh the current JWT token before it expires. **Requires auth.**

**Response 200:**
```json
{
  "token": "eyJhbGciOi..."
}
```

---

### `GET /api/v1/auth/me`

Get the current authenticated user's information. **Requires auth.**

**Response 200:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "username": "admin",
  "role": "admin",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

---

### `GET /api/v1/auth/me/projects`

Return the caller's resolved project memberships (direct and transitive through groups). **Requires auth.**

**Response 200:**
```json
[
  {
    "project": { "id": "...", "kind": "project", "name": "orion", "is_system": false, "created_at": "...", "updated_at": "..." },
    "role": "project:developer"
  }
]
```

Sorted by project name ASC. Empty array when the user has no memberships (or no Person party has been created yet — see ADR-011).

**Errors:** `401 Unauthorized` · `500 Internal Server Error`.

**Role resolution.** When a user reaches a project via multiple paths, the highest-privilege role wins (`project:owner > project:developer > project:viewer`) per [ADR-014](../adr/014-project-role-resolution-highest-privilege.md).

---

### `PUT /api/v1/auth/password`

Change the current user's password. **Requires auth.**

**Request Body:**
```json
{
  "current_password": "old-password",
  "new_password": "New-P@ssw0rd!"
}
```

**Response 200:**
```json
{"message": "password changed"}
```

**Response 400:** Password does not meet requirements (min 10 chars, upper/lower/digit/special).

---

## Users

### `GET /api/v1/users`

List all users. **Requires auth.** Permission: `users:read`.

**Response 200:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "username": "admin",
    "role": "admin",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### `POST /api/v1/users`

Create a new user. **Requires auth.** Permission: `users:write`.

**Request Body:**
```json
{
  "username": "newuser",
  "password": "Str0ng-P@ss!",
  "role": "editor"
}
```

**Response 201:** Returns the created user (without password).

**Response 400:** Invalid request or password requirements not met.

**Response 409:** Username already exists.

---

### `GET /api/v1/users/{id}`

Get a user by ID. **Requires auth.** Permission: `users:read`.

**Response 200:** User object.

**Response 404:** User not found.

---

### `PUT /api/v1/users/{id}`

Update a user. **Requires auth.** Permission: `users:write`.

**Request Body:**
```json
{
  "role": "viewer"
}
```

**Response 200:** Returns the updated user.

**Response 404:** User not found.

---

### `DELETE /api/v1/users/{id}`

Delete a user. **Requires auth.** Permission: `users:delete`.

**Response 204:** No content.

**Response 404:** User not found.

---

## Roles

### `GET /api/v1/roles`

List all roles. **Requires auth.** Permission: `roles:read`.

**Response 200:**
```json
[
  {
    "id": "...",
    "name": "admin",
    "description": "Full access to all resources",
    "permissions": [
      "catalog:read", "catalog:write", "catalog:delete",
      "users:read", "users:write", "users:delete",
      "roles:read", "roles:write", "roles:delete",
      "settings:read", "settings:write"
    ],
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
]
```

---

### `POST /api/v1/roles`

Create a new role. **Requires auth.** Permission: `roles:write`.

**Request Body:**
```json
{
  "name": "custom-role",
  "description": "A custom role",
  "permissions": ["catalog:read", "catalog:write"]
}
```

**Response 201:** Returns the created role.

**Response 409:** Role name already exists.

---

### `GET /api/v1/roles/{id}`

Get a role by ID. **Requires auth.** Permission: `roles:read`.

**Response 200:** Role object.

**Response 404:** Role not found.

---

### `PUT /api/v1/roles/{id}`

Update a role. **Requires auth.** Permission: `roles:write`.

**Request Body:**
```json
{
  "description": "Updated description",
  "permissions": ["catalog:read", "catalog:write", "users:read"]
}
```

**Response 200:** Returns the updated role.

**Response 404:** Role not found.

---

### `DELETE /api/v1/roles/{id}`

Delete a role. **Requires auth.** Permission: `roles:write`.

**Response 204:** No content.

**Response 404:** Role not found.

---

## Settings

### `GET /api/v1/settings`

List all settings. **Requires auth.** Permission: `settings:read`.

**Response 200:**
```json
[
  {
    "key": "app.name",
    "value": "AgentLens",
    "description": "Application display name",
    "category": "general",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### `GET /api/v1/settings/{category}`

Get settings filtered by category. **Requires auth.** Permission: `settings:read`.

**Response 200:** Array of setting objects in the given category.

---

### `PUT /api/v1/settings`

Bulk update settings. **Requires auth.** Permission: `settings:write`.

**Request Body:**
```json
{
  "app.name": "My AgentLens",
  "app.default_role": "editor"
}
```

---

## Party Archetype — Groups, Projects, Memberships

AgentLens uses a unified **party archetype** model. Users, groups, and projects are all `Party` entities connected by named directed `PartyRelationship` edges. This enables hierarchical groups and project-scoped RBAC.

**Project-scoped roles:** `project:owner`, `project:developer`, `project:viewer`

| Role | catalog:read | catalog:write | catalog:delete |
| --- | :---: | :---: | :---: |
| `project:owner` | ✓ | ✓ | ✓ |
| `project:developer` | ✓ | ✓ | — |
| `project:viewer` | ✓ | — | — |

Global role bypass: any user whose JWT carries the required permission is allowed regardless of project membership.

---

### `GET /api/v1/groups`

List all groups. **Requires auth.**

**Response 200:**
```json
[{"id": "...", "kind": "group", "name": "eng-team", "is_system": false, "created_at": "..."}]
```

---

### `POST /api/v1/groups`

Create a new group. **Requires auth.** Permission: `users:write`.

**Request Body:**
```json
{"name": "eng-team"}
```

**Response 201:** Party object.

**Errors:** `400` name missing · `403` insufficient permissions

---

### `GET /api/v1/groups/{partyID}`

Get a group by ID. **Requires auth.**

**Response 200:** Party object. **Response 404:** Not found.

---

### `DELETE /api/v1/groups/{partyID}`

Delete a group (non-system only). **Requires auth.** Permission: `users:write`.

**Response 204.** **Errors:** `400` system party or not found · `403` insufficient permissions

---

### `GET /api/v1/groups/{partyID}/members`

List direct members of a group. **Requires auth.**

**Response 200:**
```json
[{"id": "...", "from_party_id": "alice-id", "from_role": "member", "to_party_id": "eng-id", "to_role": "group", "relationship_name": "group_member"}]
```

---

### `POST /api/v1/groups/{partyID}/members`

Add a member to a group. **Requires auth.** Permission: `users:write`.

**Request Body:**
```json
{"party_id": "<person-or-group-party-id>", "role": "member"}
```

Valid roles: `member`. **Response 201:** PartyRelationship object.

**Errors:** `400` invalid role / cycle detected · `403` insufficient permissions

---

### `DELETE /api/v1/groups/{partyID}/members/{memberPartyID}`

Remove a member from a group. **Requires auth.** Permission: `users:write`.

**Response 204.**

---

### `GET /api/v1/projects`

List all projects. **Requires auth.**

**Response 200:** Array of Party objects with `kind: "project"`.

---

### `POST /api/v1/projects`

Create a new project. **Requires auth.** Permission: `catalog:write`.

**Request Body:**
```json
{"name": "my-project"}
```

**Response 201:** Party object.

---

### `GET /api/v1/projects/{partyID}`

Get a project by ID. **Requires auth.**

**Response 200:** Party object. **Response 404:** Not found.

---

### `DELETE /api/v1/projects/{partyID}`

Delete a project (non-system only). **Requires auth.** Permission: `catalog:write`.

**Response 204.** **Errors:** `400` system party (default project cannot be deleted).

---

### `GET /api/v1/projects/{partyID}/members`

List direct members of a project. **Requires auth.**

**Response 200:** Array of PartyRelationship objects.

---

### `POST /api/v1/projects/{partyID}/members`

Assign a user or group to a project with a role. **Requires auth.** Permission: `catalog:write`.

**Request Body:**
```json
{"party_id": "<person-or-group-party-id>", "role": "project:developer"}
```

Valid roles: `project:owner`, `project:developer`, `project:viewer`.

**Response 201:** PartyRelationship object.

---

### `DELETE /api/v1/projects/{partyID}/members/{memberPartyID}`

Remove a member from a project. **Requires auth.** Permission: `catalog:write`.

**Response 204.**

---

### `PATCH /api/v1/projects/{partyID}/members/{memberID}`

Update a member's role on a project in place. **Requires auth.** Permission: `catalog:write`.

**Body:** `{"role": "project:owner" | "project:developer" | "project:viewer"}`

**Response 204.**

**Errors:** `400` missing or invalid role · `403` insufficient permissions · `404` member not found · `500 Internal Server Error`

---

### `GET /api/v1/catalog/{id}/projects`

List all projects a catalog entry belongs to. **Requires auth.**

**Response 200:** Array of Party objects.

---

### `POST /api/v1/catalog/{id}/projects`

Assign a catalog entry to a project. **Requires auth.** Permission: `catalog:write`.

**Request Body:**
```json
{"project_id": "<project-party-id>"}
```

**Response 201.** Idempotent — duplicate assignments are silently ignored.

---

### `DELETE /api/v1/catalog/{id}/projects/{projectID}`

Remove a catalog entry from a project. **Requires auth.** Permission: `catalog:write`.

**Response 204.**

**Response 200:** Confirmation message.
