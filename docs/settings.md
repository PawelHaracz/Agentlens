# Settings

AgentLens provides a key-value settings system for runtime configuration. Settings are stored in the database and can be managed via the REST API.

---

## Overview

Settings are organized by **category** and identified by a unique **key**. Each setting has a value, a default, and a description.

---

## Settings Keys

| Key | Category | Default | Description |
|---|---|---|---|
| `app.name` | general | `AgentLens` | Application display name |
| `app.registration_enabled` | auth | `true` | Allow new user registration |
| `app.default_role` | auth | `viewer` | Default role for new users |

### Health Check

| Key | Default | Env var | Description |
| --- | --- | --- | --- |
| `health_check.enabled` | `true` | `AGENTLENS_HEALTH_CHECK_ENABLED` | Enable/disable periodic health probing |
| `health_check.interval` | `30s` | `AGENTLENS_HEALTH_CHECK_INTERVAL` | How often to probe each entry |
| `health_check.timeout` | `5s` | `AGENTLENS_HEALTH_CHECK_TIMEOUT` | Probe request timeout |
| `health_check.concurrency` | `8` | `AGENTLENS_HEALTH_CHECK_CONCURRENCY` | Parallel probes |
| `health_check.degraded_latency` | `1500ms` | `AGENTLENS_HEALTH_CHECK_DEGRADED_LATENCY` | Latency threshold above which a 2xx response → degraded state |
| `health_check.failure_threshold` | `3` | `AGENTLENS_HEALTH_CHECK_FAILURE_THRESHOLD` | Consecutive failures before → offline state |

---

## Categories

| Category | Description |
|---|---|
| `general` | General instance settings |
| `auth` | Authentication and session settings |

---

## API

### List All Settings

```bash
curl http://localhost:8080/api/v1/settings \
  -H "Authorization: Bearer <token>"
```

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

### Get Settings by Category

```bash
curl http://localhost:8080/api/v1/settings/auth \
  -H "Authorization: Bearer <token>"
```

### Bulk Update Settings

```bash
curl -X PUT http://localhost:8080/api/v1/settings \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "app.name": "My AgentLens",
    "app.default_role": "editor"
  }'
```

**Response 200:** Confirmation message.

> **Permission required:** `settings:write` (admin role by default).

---

## Notes

- Settings changes take effect immediately — no restart required.
- Settings stored in the database override values from config files and environment variables.
- The `settings:read` permission is required to view settings (all default roles have this).
- The `settings:write` permission is required to modify settings (admin role only by default).
