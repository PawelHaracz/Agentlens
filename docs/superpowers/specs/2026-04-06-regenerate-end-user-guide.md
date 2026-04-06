# Task: Regenerate `docs/end-user-guide.md` end-to-end with fresh screenshots

## Background

The AgentLens UI and feature surface have been heavily refactored. `docs/end-user-guide.md` and the screenshots in `docs/images/` are stale — they reference older layouts, old labels, and flows that no longer exist. We need a **complete regeneration** of the end-user guide and **all** of its screenshots, captured against the *current* UI.

This is not a patch job. Re-derive every section from the live application, not from the existing markdown.

## Authoritative sources of truth

When in doubt about what a feature is called, what permissions it needs, or how it behaves, the **code is canonical**, in this order:

1. `web/src/**` — React components, route definitions, labels, and UI copy. This is what users actually see.
2. `internal/api/router.go` and `internal/api/*_handlers.go` — endpoints, permissions enforced via `RequirePermission`, request/response shapes.
3. `internal/model/**` — domain types (`CatalogEntry`, `AgentType`, `Capability`, `Status`, `Protocol`, `SourceType`).
4. `internal/auth/permissions.go` — permission strings (`catalog:read`, `users:write`, etc.) and which roles get them.
5. `docs/api.md` — already-updated REST reference.
6. `docs/architecture.md` — system diagrams (Mermaid). Use these to inform the conceptual sections, do not duplicate.

**Do not** copy text from the existing `docs/end-user-guide.md` without verifying it against the code. If text and code disagree, the code is right.

## Scope of the rewrite

The new `docs/end-user-guide.md` must cover, at minimum:

1. **Introduction** — what AgentLens is, what protocols it supports today (check `internal/model/protocol.go` and parser plugins under `plugins/parsers/`), who the document is for.
2. **Signing in** — login page, password requirements (see `internal/auth/password.go`: bcrypt cost 12, ≥10 chars, upper+lower+digit+special, lockout after 5 failed attempts → 15 min). Mention the first-run admin bootstrap printed to stdout.
3. **Dashboard / catalog overview** — stats, columns, sort order, empty state.
4. **Browsing the catalog** — pagination, sorting, the columns shown today.
5. **Searching and filtering** — every filter that exists in the current UI (protocol, status, source, category, free-text). Verify by reading `web/src/**` and `internal/api/handlers.go` (the catalog list handler).
6. **Viewing agent details** — the entry detail view, what fields are shown, the Capability tabs (`a2a.skill`, `a2a.interface`, `a2a.security_scheme`, `a2a.extension`, `a2a.signature`, `mcp.tool`, `mcp.resource`, `mcp.prompt`), the raw JSON view if present, and any actions (delete, refresh health, etc.).
7. **Registering an agent** — every register flow that currently exists. Read `web/src/**` to enumerate them. Today this is at least:
   - Import from URL (auto-detects A2A vs MCP, with SSRF protection — mention that private/loopback URLs are rejected by design).
   - Paste / upload a JSON card.
   - Manual form entry (if still present).
   For each: show empty state → filled form → validation errors → success.
8. **Status indicators** — what each value of `model.Status` means and when it changes. Cross-reference `internal/health/checker.go`.
9. **Protocol types** — A2A, MCP (and any others present in `plugins/parsers/`). Brief, user-facing.
10. **Settings** — every settings page currently in the UI (general, account, etc.). Cross-reference `internal/api/settings_handlers.go` and `docs/settings.md`.
11. **User management** — list, create, edit, delete, lockout reset. Cite required permissions (`users:read`, `users:write`).
12. **Role management** — list, create, edit (only non-system roles), delete. Mention that system roles (`IsSystem=true`) cannot be modified or deleted, and that the last active admin cannot be deleted.
13. **My account** — change password, view sessions if implemented.
14. **Using the REST API** — short pointer to `docs/api.md`, an `Authorization: Bearer …` example, and how to mint a token via `POST /api/v1/auth/login`. Do not duplicate the full API reference.
15. **FAQ / Troubleshooting** — locked-out account, forgotten admin password (re-bootstrap procedure), private-URL import rejected, etc.

If the live UI has features not in the list above, **add them**. If the list above mentions something the UI no longer has, **drop it**.

## Screenshot requirements

All screenshots must be regenerated. **Delete every existing PNG in `docs/images/`** and capture fresh ones using a Playwright script.

### Where & how

- Add a new Playwright spec at `e2e/tests/docs-screenshots.spec.ts`. It is a *generator*, not an assertion test — it walks through the UI and saves screenshots to `docs/images/`.
- Reuse `e2e/tests/helpers.ts` (`loginViaUI`, `loginViaAPI`, `createCatalogEntry`, `createUser`, `BASE`, `adminPassword`).
- Drive the existing harness: `make e2e-test` already builds the binary, starts the server on `:18080` with a fresh SQLite DB, captures the bootstrap admin password into `AGENTLENS_ADMIN_PASSWORD`, and runs Playwright. Add a new make target `make docs-screenshots` that invokes `e2e/run-e2e.sh tests/docs-screenshots.spec.ts` so the same harness is reused.
- Use `await page.screenshot({ path: 'docs/images/<name>.png', fullPage: false })` for focused shots, `fullPage: true` for long pages (catalog list, settings).
- Set a stable viewport: `await page.setViewportSize({ width: 1440, height: 900 })` at the top of each test. Prefer 1440×900 — it matches what we shipped before and looks sharp on retina without bloating PNG size.
- Disable animations to avoid flake: add `await page.emulateMedia({ reducedMotion: 'reduce' })`.
- Seed deterministic data via the API helpers before screenshotting (e.g. `createCatalogEntry` for an A2A and an MCP entry, `createUser` for a non-admin user). Avoid `Date.now()` in `display_name` — use stable strings like `"Demo Translator Agent"` so screenshots are reproducible.
- For destructive flows (delete confirmation, validation errors), capture the modal/error state but do NOT actually destroy the seed data unless you reseed afterward.

### Required screenshots (minimum set — add more if a section needs them)

Save with these exact filenames so the markdown can reference them stably:

| File | What it shows |
|---|---|
| `login-page.png` | Empty login form |
| `login-error.png` | Login form with "invalid credentials" error |
| `dashboard-overview.png` | Dashboard / catalog landing, populated with seeded entries |
| `catalog-search.png` | Catalog with an active free-text search |
| `catalog-filter-protocol.png` | Catalog filtered by protocol (e.g. A2A only) |
| `catalog-filter-status.png` | Catalog filtered by status |
| `entry-detail-a2a.png` | Detail view of an A2A entry, default tab |
| `entry-detail-a2a-skills.png` | Detail view, skills tab populated |
| `entry-detail-mcp.png` | Detail view of an MCP entry, tools tab |
| `entry-detail-raw-json.png` | Raw JSON / definition view (if present) |
| `register-menu.png` | The "Register" entrypoint (button or menu) on the catalog page |
| `register-import-url-empty.png` | Import-from-URL tab, empty state |
| `register-import-url-filled.png` | Import-from-URL tab with a public URL filled in |
| `register-import-url-error-private.png` | Import-from-URL rejected for a private/loopback URL — important: shows the SSRF guard message |
| `register-import-url-success.png` | Successful import landing on the new entry |
| `register-paste-json-empty.png` | Paste-JSON tab, empty |
| `register-paste-json-validation.png` | Paste-JSON tab showing schema validation errors |
| `register-paste-json-preview.png` | Paste-JSON tab showing the parsed card preview before save |
| `register-success-toast.png` | Success notification after register |
| `settings-general.png` | Settings → General |
| `settings-account.png` | Settings → My Account (change password form) |
| `settings-users-list.png` | Settings → Users list |
| `settings-users-create.png` | Settings → Users → New user dialog |
| `settings-users-locked.png` | A locked-out user row, with reset action visible |
| `settings-roles-list.png` | Settings → Roles list (showing system + custom) |
| `settings-roles-edit.png` | Editing a non-system role |
| `user-dropdown.png` | Top-bar user menu open |

If the current UI has tabs/pages not in this table (e.g. health dashboard, audit log, license page), **add them** with sensible filenames.

After capturing, run `git status -- docs/images/` and confirm only the expected files are present. Delete any leftover stale PNGs.

## Markdown style & rules

- Use GitHub-flavored markdown.
- Keep the existing TOC pattern (`## Table of Contents` with anchor links). Regenerate it from the new headings — don't carry stale entries.
- Embed screenshots with `![alt text](images/<file>.png)` and a one-line caption underneath. Alt text must describe the screenshot for accessibility.
- Use Mermaid for any sequence/flow diagrams (the project standard — see `CLAUDE.md`). Do not use ASCII art or PlantUML.
- Cross-link to other docs with relative paths: `[REST API](api.md)`, `[Architecture](architecture.md)`, `[Settings reference](settings.md)`.
- When you mention a permission, write it in backticks: `catalog:read`.
- When you mention an env var, write it in backticks: `AGENTLENS_DB_DIALECT`.
- Do not invent features. If something is unclear from code + UI, mark it `<!-- TODO: verify -->` instead of guessing, and list every TODO in the PR description.

## Verification before opening the PR

Run all of these and make sure they're green:

1. `make web-build` — frontend builds.
2. `make build` — Go binary builds.
3. `make test` — unit tests pass.
4. `make docs-screenshots` (the new target) — regenerates `docs/images/`.
5. `make e2e-test` — full E2E suite still passes (your generator spec must not break the existing assertions, and must clean up after itself or be opt-in via filename).
6. `make arch-test` — architecture rules pass.
7. Manual smoke: open the new `docs/end-user-guide.md` in a markdown previewer and confirm every image renders, every internal link resolves, the TOC matches the headings, and there are no `<!-- TODO -->` markers left unflagged.
8. `git status` — only `docs/end-user-guide.md`, `docs/images/*.png`, `e2e/tests/docs-screenshots.spec.ts`, and `Makefile` (for the new target) should be modified.

## Pull request

- Title: `docs: regenerate end-user guide and screenshots after UI refactor`
- Body must include:
  - Bullet summary of what changed (sections added, sections removed, images regenerated).
  - The exact `make` commands you ran for verification, with status.
  - Any `<!-- TODO: verify -->` items still in the doc, with a note on why you couldn't resolve them.
  - A screenshot count diff (`before: N images, after: M images`).
- Do not commit anything under `e2e/test-results/`.
- Do not modify any source code under `web/src/**` or `internal/**` — this PR is documentation-only plus the new Playwright generator and Makefile target.

## Things NOT to do

- Do not edit `docs/api.md`, `docs/architecture.md`, `docs/developer-guide.md`, or `docs/devops-guide.md`. They're maintained separately.
- Do not add screenshots of internal/admin-only debug pages.
- Do not embed user passwords, tokens, JWTs, or any value from `AGENTLENS_ADMIN_PASSWORD` in either the markdown or the screenshots. If a screenshot would show a token, mask it before saving.
- Do not skip the screenshot regeneration "because the old ones look fine" — they don't, that's why we're doing this.
