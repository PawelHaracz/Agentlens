# Global: PR Feature Checklist

Every PR must pass this seven-item checklist from CLAUDE.md. Non-negotiable.

### Seven-Step Feature Checklist (Every PR)

Every PR must:

1. `rtk make test` passes.
2. `rtk make e2e-test` passes.
3. `docs/api.md` updated (method, path, request/response schema, error codes, required permissions) if the API surface changed.
4. `docs/architecture.md` updated (Mermaid only) if design changed.
5. `docs/end-user-guide.md` updated for UI changes, with screenshots taken via `page.screenshot()` using `data-testid` selectors, saved to `docs/images/`.
6. `docs/settings.md` + `internal/config/` updated when new config keys are added.
7. `rtk make arch-test` passes (100% layer compliance).

Sources: CLAUDE.md, AGENTS.md.

### Documentation Must Match Implementation Exactly

Reviewers consistently flag documentation that describes endpoints, response shapes, config keys, or defaults that drift from the actual implementation. Use exact casing, values, and endpoint paths emitted by the server (e.g., `a2a` lowercase, not `A2A`; `registered` lifecycle state, not `pending`; `health_check.interval 30s`, not `health.check_interval 60s`). Update docs/api.md, docs/end-user-guide.md, docs/settings.md, README.md, and ADRs alongside the code change. Source: PR reviews (9 distinct PRs flagged this).

### Document New Permissions Across Three Files

When adding a permission, update:

- `docs/auth.md` — add to roles/permissions tables.
- `docs/api.md` — note which endpoints require the new permission.
- `README.md` — update the Roles & Permissions table if needed.

Plus add tests that verify enforcement via `RequirePermission`.

Source: CONTRIBUTING.md.
