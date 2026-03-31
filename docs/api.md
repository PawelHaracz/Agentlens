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
| `401` | Unauthorized — missing or invalid JWT token |
| `403` | Forbidden — insufficient permissions |
| `404` | Resource not found |
| `409` | Conflict — resource already exists |
| `423` | Locked — account locked due to failed login attempts |
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

**Response 200:** Confirmation message.
