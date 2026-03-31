# Settings

AgentLens provides a key-value settings system for runtime configuration. Settings are stored in the database and can be managed via the REST API.

---

## Overview

Settings are organized by **category** and identified by a unique **key**. Each setting has a value, a default, and a description.

---

## Settings Keys

| Key | Category | Default | Description |
|---|---|---|---|
| `general.instance_name` | general | `AgentLens` | Display name for this AgentLens instance |
| `general.log_level` | general | `info` | Logging level (debug/info/warn/error) |
| `discovery.poll_interval` | discovery | `5m` | How often to poll sources for agent discovery |
| `discovery.kubernetes_enabled` | discovery | `false` | Enable Kubernetes service discovery |
| `health.check_enabled` | health | `true` | Enable periodic health checking of agents |
| `health.check_interval` | health | `30s` | Interval between health checks |
| `health.check_timeout` | health | `5s` | Timeout for individual health checks |
| `health.check_concurrency` | health | `10` | Number of concurrent health checks |
| `auth.session_duration` | auth | `24h` | JWT token expiration duration |
| `auth.max_failed_attempts` | auth | `5` | Failed login attempts before lockout |
| `auth.lockout_duration` | auth | `15m` | Account lockout duration |

---

## Categories

| Category | Description |
|---|---|
| `general` | General instance settings |
| `discovery` | Agent discovery settings |
| `health` | Health check settings |
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
    "key": "general.instance_name",
    "value": "AgentLens",
    "default_value": "AgentLens",
    "description": "Display name for this AgentLens instance",
    "category": "general",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

### Get Settings by Category

```bash
curl http://localhost:8080/api/v1/settings?category=health \
  -H "Authorization: Bearer <token>"
```

### Get a Single Setting

```bash
curl http://localhost:8080/api/v1/settings/general.instance_name \
  -H "Authorization: Bearer <token>"
```

**Response 200:**
```json
{
  "key": "general.instance_name",
  "value": "AgentLens",
  "default_value": "AgentLens",
  "description": "Display name for this AgentLens instance",
  "category": "general",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

### Update a Setting

```bash
curl -X PUT http://localhost:8080/api/v1/settings/general.instance_name \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"value": "My AgentLens"}'
```

**Response 200:** Returns the updated setting object.

> **Permission required:** `settings:write` (admin role by default).

### Bulk Update Settings

```bash
curl -X PUT http://localhost:8080/api/v1/settings \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '[
    {"key": "health.check_interval", "value": "60s"},
    {"key": "health.check_concurrency", "value": "20"}
  ]'
```

**Response 200:** Returns the list of updated setting objects.

> **Permission required:** `settings:write` (admin role by default).

---

## Notes

- Settings changes take effect immediately — no restart required.
- Settings stored in the database override values from config files and environment variables.
- The `settings:read` permission is required to view settings (all default roles have this).
- The `settings:write` permission is required to modify settings (admin role only by default).
