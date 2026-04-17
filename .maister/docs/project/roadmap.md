# Development Roadmap

## Current State

- **Version**: Helm chart `0.2.0` / app `0.2.0`.
- **Key features**:
  - Agent catalog with REST API (`/api/v1/catalog`, `/capabilities`, `/stats`).
  - Kubernetes annotation-based discovery (`plugins/sources/k8s`), static config, and push registration.
  - A2A and MCP parser plugins producing a typed `Capability[]`.
  - JWT + RBAC auth (admin/editor/viewer), account lockout, bcrypt cost 12.
  - Embedded React SPA (Vite + shadcn/ui + TanStack Query).
  - Dual-database: SQLite default, PostgreSQL (enterprise).
  - OpenTelemetry traces/metrics/logs + Prometheus `/metrics`.
  - Helm chart, Docker image, health/readiness probes.
- **Recent updates** (from git history):
  - `feat: OpenTelemetry observability and production-ready Helm chart` (#22)
  - `feat: agent security capability` (#18)
  - Git hook devops hardening (#19), SchemeCard test coverage (#20, #21).
  - Ongoing `chore/migrate-to-maister` — framework migration to AI SDLC standards.

---

## Planned Enhancements (Next 3–6 Months)

Organized by the four priorities set for the initiative: **OSS polish, enterprise, observability, plugin ecosystem**.

### High Priority

#### OSS / Public Release
- [ ] **Quickstart-in-5-minutes** — seeded SQLite store, example agents, one-command local bring-up. `Effort: M`
- [ ] **Plugin author guide** — covering lifecycle, layer boundaries, `Plugin` suffix rule, `ErrLicenseRequired` silent-skip semantics. `Effort: M`
- [ ] **End-user guide parity** — auto-refresh screenshots via Playwright `data-testid` selectors into `docs/images/`. `Effort: S`
- [ ] **Licensing clarity** — clear delineation between OSS core and enterprise-gated plugins (`plugins/enterprise/*`). `Effort: S`

#### Plugin Ecosystem
- [ ] **Plugin scaffold CLI** — `agentlens plugin new parser|source <name>` producing compilable boilerplate with tests. `Effort: L`
- [ ] **Versioned plugin API** — semver guarantees on `kernel.Plugin` and parser/source interfaces. `Effort: M`

#### Enterprise
- [ ] **PostgreSQL CI parity** — containerized PG in test matrix, not just `sqlite :memory:`. `Effort: M`
- [ ] **Backup/restore runbook** — documented procedures for SQLite and PostgreSQL deployments. `Effort: S`

#### Observability
- [ ] **Reference Grafana dashboards** — shipped under `deploy/grafana/`. `Effort: M`
- [ ] **OTel Collector pipelines** — sample configs for local and cloud backends. `Effort: S`

### Medium Priority

#### OSS / Public Release
- [ ] **OpenAPI spec generation** — auto-derive from chi routes, publish alongside docs. `Effort: M`
- [ ] **Contribution paths** — `good-first-issue`, `help-wanted`, CODEOWNERS tuning. `Effort: S`

#### Plugin Ecosystem
- [ ] **Example community plugins** — at least one additional protocol parser and one new source type as reference implementations. `Effort: L`
- [ ] **Plugin manifest metadata** — capabilities/version/compatibility declared by each plugin. `Effort: M`

#### Enterprise
- [ ] **RBAC: custom roles** — move beyond the three built-in system roles with user-defined permission sets. `Effort: L`
- [ ] **SSO parity** — SAML + OIDC feature parity, session lifecycle aligned with IdP policies. `Effort: L`
- [ ] **Audit stream API** — queryable export for SIEMs. `Effort: M`

#### Observability
- [ ] **Per-agent SLO metrics** — surface health SLOs in the UI and via `/metrics`. `Effort: M`
- [ ] **Cardinality guardrails** — configurable limits and warnings for label cardinality. `Effort: S`

### Technical Debt / Hardening
- [ ] **Remove CORS `*` default** — replace with an allowlist (sign-off required; flagged in CLAUDE.md). `Effort: S`
- [ ] **Code-splitting the SPA** — route-based `React.lazy` to shrink initial bundle. `Effort: S`
- [ ] **Bundle-size budget in CI** — fail PRs that regress the web bundle. `Effort: S`
- [ ] **Multi-dialect migration tests** — run all migrations against both SQLite and PostgreSQL in CI. `Effort: M`

---

## Future Considerations

- **Agent mesh integrations** — direct integrations with service-mesh control planes (Istio, Linkerd) for transport-level discovery.
- **Policy plane** — per-capability allow/deny policy enforced at registration time.
- **Federation** — multiple AgentLens instances sharing catalogs across trust boundaries.
- **Terraform/Pulumi provider** — declarative management of catalog entries and providers.
- **Plugin marketplace** — discovery index for community plugins.

---

**Effort Scale**: `S`: 2–3 days • `M`: 1 week • `L`: 2+ weeks

*Last Updated*: 2026-04-17
