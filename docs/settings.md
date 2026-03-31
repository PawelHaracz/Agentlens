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
