# Authentication

AgentLens includes built-in authentication with JWT tokens and role-based access control (RBAC).

---

## Bootstrap (First Run)

On first startup, AgentLens automatically:

1. Creates three default roles: **admin**, **editor**, **viewer**
2. Creates an `admin` user with a randomly generated password
3. Prints the credentials to stdout:

```
============================================
  INITIAL ADMIN CREDENTIALS
  Username: admin
  Password: <generated>
  CHANGE THIS PASSWORD IMMEDIATELY
============================================
```

> **Important:** Save this password — it is only displayed once.

---

## Login Flow

### 1. Obtain a Token

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "<your-password>"}'
```

**Response 200:**
```json
{
  "token": "eyJhbGciOi...",
  "user": {
    "id": "...",
    "username": "admin",
    "role": "admin"
  }
}
```

### 2. Use the Token

Include the JWT in the `Authorization` header for all subsequent requests:

```bash
curl http://localhost:8080/api/v1/catalog \
  -H "Authorization: Bearer <token>"
```

### 3. Refresh the Token

Before the token expires, request a new one:

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Authorization: Bearer <current-token>"
```

### 4. Logout

```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer <token>"
```

---

## JWT Details

| Property | Default |
|---|---|
| Signing algorithm | HS256 |
| Token expiration | 24 hours (configurable via `AGENTLENS_SESSION_DURATION`) |
| Signing secret | Auto-generated on first run (override with `AGENTLENS_JWT_SECRET`) |

Tokens contain the user ID, username, and role in the payload. The server validates tokens on every authenticated request.

---

## Account Lockout Policy

To protect against brute-force attacks, AgentLens enforces an account lockout policy:

- **Max failed attempts:** 5
- **Lockout duration:** 15 minutes
- After 5 consecutive failed login attempts, the account is locked for 15 minutes.
- Successful login resets the failed attempt counter.
- Admins can unlock accounts manually via the user management API.

---

## Password Requirements

Passwords must meet all of the following criteria:

- Minimum **10 characters**
- At least one **uppercase** letter
- At least one **lowercase** letter
- At least one **digit**
- At least one **special character** (e.g., `!@#$%^&*`)

These requirements apply when creating users and changing passwords.

---

## Roles & Permissions

Three default roles are created on first run:

| Role | Permissions |
|------|------------|
| **admin** | Full access: catalog, users, roles, settings (read/write/delete) |
| **editor** | catalog:read/write, users:read, roles:read, settings:read |
| **viewer** | catalog:read, users:read, roles:read, settings:read |

All permissions follow the `resource:action` format:

| Permission | Description |
|---|---|
| `catalog:read` | View catalog entries |
| `catalog:write` | Create/update catalog entries |
| `catalog:delete` | Delete catalog entries |
| `users:read` | View users |
| `users:write` | Create/update users |
| `users:delete` | Delete users |
| `roles:read` | View roles |
| `roles:write` | Create/update/delete roles |
| `settings:read` | View settings |
| `settings:write` | Update settings |

---

## Unauthenticated Endpoints

The following endpoints do **not** require authentication:

- `GET /healthz` — health check
- `POST /api/v1/auth/login` — login

All other `/api/v1/*` endpoints require a valid JWT token.
