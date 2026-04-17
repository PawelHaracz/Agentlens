# Security: Authorization

RBAC with permission checks mounted at route registration time via middleware. Inline permission checks in handlers are prohibited.

### Permissions Enforced via `RequirePermission` Middleware

Authorization is enforced through the `RequirePermission` middleware mounted at route registration (see `registerCatalogRoutes`, `registerUserRoutes`). Never call `auth.HasPermission(...)` inline inside handlers. Use `auth.Perm*` constants, not raw strings such as `"users:write"`. Sources: CLAUDE.md, PR reviews (#23, #1).

### `resource:action` Permission Format

All permissions follow the `resource:action` format (e.g., `catalog:read`, `users:write`). Sources: README.md, docs/auth.md, CONTRIBUTING.md.

### System Role & Last-Admin Protection

System roles (`IsSystem=true`) must be undeletable and unmodifiable via the API. Deletion of the last active admin user must be rejected. Sources: CLAUDE.md, ADR-005.
