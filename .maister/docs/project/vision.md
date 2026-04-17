# Project Vision

## Overview

**AgentLens** is a self-hosted service discovery and catalog platform for AI agents. It answers a question that is quickly becoming central to any organization running more than one agent: *"Which agents do we run, where are they, what can they do, and are they healthy right now?"*

It is a single-binary Go service with an embedded React dashboard. Agents are discovered from Kubernetes annotations, static configuration, or pushed via an API. Each agent is parsed through protocol-specific plugins (A2A, MCP) into a typed catalog entry that downstream orchestrators, UIs, and humans can query.

---

## Current State

- **Age**: Active development, mature design baseline, ten ADRs codifying core decisions.
- **Status**: Active — commits in the last 24 hours; pre-1.0 but production-ready architecture.
- **Users**: Early-stage adopters running self-hosted agent platforms; target expansion to the wider OSS community.
- **Tech stack**: Go 1.26.1 (backend) + React 18 + TypeScript (frontend, embedded via `embed.FS`). Dual-database: SQLite default, PostgreSQL for enterprise/multi-instance.
- **Deployment**: Docker + Helm chart (`0.2.0`). Observability via OpenTelemetry and Prometheus. Auth via JWT + RBAC.

---

## Purpose

Agent ecosystems fragment fast. Protocols diverge (A2A, MCP, A2UI), runtimes spread across clusters and vendors, and the answer to *"what capabilities do we offer?"* quickly drifts out of sync with reality.

AgentLens exists to close that gap:

- **Single source of truth** for agent identity, capabilities, and health across protocols.
- **Self-hosted by design** — the catalog of your agents is not something you hand to a third party.
- **Protocol-agnostic at the edges, typed at the center** — parser plugins normalize disparate agent cards into a common domain model built around `AgentType`, `CatalogEntry`, and polymorphic `Capability`.
- **Extensible via plugins** — a microkernel separates the stable core from the protocols and sources that will keep multiplying.

The domain model follows the Product Archetype pattern (ADR-001, ADR-004): `AgentType` describes what an agent *is* (protocol, endpoint, `AgentKey = SHA256(protocol+endpoint)`, capabilities); `CatalogEntry` is how it is *offered* (display name, status, source, lifecycle state). This separation is deliberate — it lets multiple catalogs express the same underlying agent differently without duplicating its identity.

---

## Goals (Next 3–6 Months)

Driven by the four priorities chosen for this initiative:

### 1. Public Release / OSS Polish
- Smooth first-run experience (`quickstart`, example agents, seeded fixtures).
- User-facing docs hardened: `docs/end-user-guide.md` kept current with screenshots via Playwright.
- Public landing narrative, clear licensing story for enterprise-gated features.
- Community contribution paths (good-first-issue labels, plugin author guide).

### 2. Enterprise Features
- PostgreSQL production hardening: migration parity tests in CI, backup/restore runbook.
- RBAC depth: beyond `admin/editor/viewer`, custom permission sets, fine-grained scopes.
- SSO: SAML + OIDC parity, session lifecycle aligned with enterprise IdP policies.
- Audit: queryable audit stream (today license-gated via `plugins/enterprise/audit`).

### 3. Observability Maturity
- Reference Grafana dashboards shipped alongside the Helm chart.
- Sample OTel Collector pipelines (local + cloud).
- Per-agent health SLO metrics surfaced in the UI.
- Built-in `/metrics` cardinality guardrails.

### 4. Plugin Ecosystem
- Plugin scaffold CLI (`agentlens plugin new parser|source`).
- Public plugin author guide — lifecycle (`Register → InitAll → StartAll → StopAll`), layer boundaries (plugins import `kernel` + `foundation`, never `api`/`auth`/`server`/`cmd`).
- Example community plugins (additional protocols, new source types).
- Versioned plugin API contract (semver on the `Plugin` interface).

---

## Evolution

The project has consistently reinforced three directional bets:

1. **Typed core, polymorphic edges.** Capabilities are discriminated unions (`a2a.skill`, `mcp.tool`, …) rather than a stringly-typed bag. Parsers translate, the kernel never guesses.
2. **Layered, enforced architecture.** `arch-go` runs at 100% in CI. Plugins do not reach into the API layer; the API layer does not reach into plugins. This has kept the plugin system viable as the core matured.
3. **Single binary, batteries included, no required externals.** SQLite default, embedded SPA, optional PostgreSQL and OpenTelemetry. Operators can run AgentLens in five minutes or integrate it with a full enterprise observability stack — the same binary serves both.

Where it is headed: a hub that organizations actually want at the center of their agent fleet, credible enough for enterprise deployment and friendly enough for OSS adoption. The near-term work aligns the product, documentation, and plugin ergonomics to that outcome.

---

*Last Updated*: 2026-04-17
