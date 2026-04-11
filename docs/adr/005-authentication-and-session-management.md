# ADR-005: Authentication and Session Management

Date: 2026-04-11
Status: Accepted
Related: [ADR-003](003-microkernel-plugin-architecture.md) (enterprise SSO gates via plugin)

## Context

AgentLens needs authentication for its catalog API. The system serves two client types: browser-based SPA (the embedded web UI) and non-browser API clients (CLI tools, other agents). Requirements:

1. **Secure password storage** — resistant to offline brute-force if the database is compromised.
2. **Stateless session tokens** — horizontal scaling without shared session state or sticky sessions.
3. **Browser and non-browser clients** — SPA needs XSS-safe cookie transport; API clients need header-based auth.
4. **Brute-force protection** — rate-limit login attempts without external infrastructure.
5. **Zero-config bootstrap** — first boot must produce a working admin account without manual setup.
6. **Simple authorization model** — permission checks on API endpoints without external policy engines.

The auth subsystem lives in the infrastructure layer (`internal/auth/`), used by the API layer's middleware. Enterprise SSO (OIDC, SAML) is a separate concern handled by plugins (ADR-003) and is out of scope here.

## Decision

### Password hashing

bcrypt with cost factor 12. Password policy enforced at validation time: minimum 10 characters, at least one uppercase, one lowercase, one digit, one special character. Implemented in `internal/auth/password.go`.

### JWT tokens

HMAC-SHA256 (HS256) symmetric signing with a single shared secret. Secret sourced from `AGENTLENS_JWT_SECRET` env var or `auth.jwt_secret` in YAML config. If neither is set, a 64-byte cryptographically random secret is auto-generated at startup (tokens will not survive process restarts).

Token claims embed `UserID`, `Username`, `RoleID`, and `Permissions` directly — the API layer can authorize requests without a database lookup on every request.

Default expiration: 24 hours. Refresh window: last 30 minutes before expiry. Refresh re-issues a new JWT with current claims from the database, picking up any role/permission changes.

### Dual transport: header + cookie

Auth middleware checks `Authorization: Bearer <token>` header first, then falls back to `agentlens_token` cookie. Login endpoint sets both:

- **Cookie**: `HttpOnly=true`, `Secure=true`, `SameSite=Strict` — immune to XSS, automatically sent by browsers.
- **JSON body**: token returned in the login response — non-browser clients extract and send via header.

### No server-side token revocation

Logout clears the cookie client-side. The JWT remains valid until its expiry time. There is no server-side blacklist or token store. This is a deliberate trade-off: stateless simplicity over instant revocation.

### Account lockout

5 consecutive failed login attempts trigger a 15-minute lockout. Tracked in the database via `FailedAttempts` and `LockedUntil` columns on the user record. Lock status is checked before password verification (prevents timing attacks leaking password validity during lockout). Successful login resets the counter.

### Bootstrap admin

On first boot (user count = 0), the system auto-creates an admin user with a 20-character cryptographically random password printed to stdout. No force-change-on-first-login mechanism.

### RBAC model

Flat role-based access control. Each user has exactly one role. Each role has N permissions as `resource:action` strings (e.g., `catalog:write`, `users:read`). Three system roles seeded via database migration: `admin`, `editor`, `viewer`. System roles are marked `IsSystem=true` and cannot be deleted or modified. Deletion of the last admin user is rejected.

Permissions are embedded in JWT claims. After a role change, the user's permissions remain stale until the token is refreshed (within the 30-minute refresh window) or the user re-authenticates.

## Consequences

### Positive

- Stateless auth — no session store, no Redis dependency, scales horizontally.
- Dual transport covers both browser and API client use cases without separate auth flows.
- bcrypt cost 12 provides strong offline brute-force resistance at acceptable login latency (~250ms).
- Lockout mechanism blocks online brute-force without external rate-limiting infrastructure.
- Zero-config bootstrap — `docker run` produces a working system with printed credentials.
- Flat RBAC is explicit and auditable — no role inheritance chains to debug.
- Permissions in JWT eliminate per-request database lookups for authorization.

### Negative / Trade-offs

- **No instant revocation** — compromised tokens remain valid until expiry (up to 24h). Mitigation: reduce token TTL if threat model requires it.
- **Stale permissions** — role changes are invisible to active sessions until token refresh. Worst case: 30 minutes of stale permissions if the user is just past the refresh window.
- **Single JWT secret, no rotation** — rotating the secret invalidates all active tokens. No graceful dual-key rotation mechanism.
- **Hardcoded password policy** — cannot be adjusted per deployment without code changes.
- **No forced password rotation** — bootstrap admin password persists until manually changed.
- **Single role per user** — cannot express "admin for project A, viewer for project B" without model changes.

### Neutral

- Symmetric signing (HS256) is sufficient for a single-service application. If AgentLens federates tokens to other services in the future, migration to asymmetric signing (RS256/ES256) would be required.
- Permissions use a flat `resource:action` string pattern. This is extensible (new resources/actions are just new strings) but not resource-instance-scoped (cannot express "write access to catalog entry X").

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| **argon2id / scrypt** | More tunable memory-hardness, but bcrypt is simpler, well-supported in Go's stdlib ecosystem, and sufficient for the expected user count (not a mass-consumer system) |
| **RS256 / ES256 asymmetric signing** | Requires key pair management; unnecessary when signing and verification happen in the same process |
| **Opaque refresh tokens** | Adds a server-side token store (defeating stateless design) for marginal UX benefit; JWT re-issue within refresh window achieves similar effect |
| **Server-side token blacklist** | Requires shared state (Redis/DB lookup per request); trades stateless simplicity for instant revocation that the current threat model doesn't demand |
| **Cookie-only auth** | Blocks non-browser API clients (CLI tools, agent-to-agent calls) |
| **Header-only auth** | Exposes tokens to XSS in browser context; cookies with HttpOnly flag are strictly safer for SPAs |
| **Casbin / OPA / ABAC** | External policy engine overhead unjustified for current resource model (~5 resource types, ~3 actions each); flat RBAC covers all current use cases |
| **Hierarchical roles** | Implicit permission inheritance is harder to audit; flat model makes granted permissions explicit |
| **Multiple roles per user** | Increases model complexity (join table, permission merge logic, conflict resolution); single role sufficient for current team-size deployments |
