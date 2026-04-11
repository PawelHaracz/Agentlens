# ADR-008: Poll-Based Agent Discovery

Date: 2026-04-11
Status: Accepted
Related: [ADR-003](003-microkernel-plugin-architecture.md) (source plugins), [ADR-004](004-product-catalog-pattern.md) (domain model)

## Context

AgentLens must discover agents from multiple sources — static configuration, Kubernetes services, and future cloud providers. Each source has different latency characteristics and failure modes, but all share the same output: a list of `AgentType` entries to upsert into the catalog.

Two competing forces shape the design:

1. **Freshness** — operators expect the catalog to reflect the current state of their infrastructure within minutes, not hours.
2. **Simplicity** — agents should not need to know about AgentLens. Discovery must be pull-based so agents remain protocol-native (A2A, MCP) without an AgentLens SDK or registration callback.

A push-based model (agents notify AgentLens of their existence) inverts the dependency direction — every agent deployment would need AgentLens-specific configuration. An event-driven model (message queue between sources and catalog) adds external infrastructure for what is fundamentally a periodic sync.

## Decision

Use a **poll-based discovery model** where a discovery manager runs a ticker loop at a configurable `pollInterval` (default 5 minutes), calling each registered source sequentially.

### Discovery loop

1. On startup, fire an immediate discovery cycle (no wait for first tick).
2. Each cycle iterates registered sources in order. One source failing does not block others — errors are logged, processing continues.
3. Each source returns `[]*AgentType`. The discovery manager upserts by `endpoint` (UNIQUE constraint). Existing entries are updated; new ones are created.
4. Capabilities use full-replacement semantics on update: delete-all + re-insert for the agent's capabilities.
5. Agents not returned by **any** source in a cycle are marked `LifecycleOffline`. This handles agents removed from K8s or dropped from static config.
6. Entries with `SourcePush` (created via REST API) are never overwritten by discovery. Discovery skips them entirely.

### Source interface

Sources implement `Name() string` + `Discover(ctx) ([]*AgentType, error)`. Three implementations exist:

- **StaticSource** — fetches agent cards from URLs defined in config.
- **K8sSource** — discovers agents via Kubernetes Service annotations (`agentlens.io/type`, `agentlens.io/card-path`, `agentlens.io/tags`, `agentlens.io/team`). Uses the List API to enumerate annotated services across namespaces and builds cluster-internal URLs.
- **Future sources** — added as `SourcePlugin` implementations per ADR-003.

### Crawler

Both static and K8s sources delegate HTTP fetching to a shared crawler: HTTP GET with a 10-second hardcoded timeout. The crawler fetches raw agent card bytes, which parser plugins (ADR-003) convert into `AgentType` + `[]Capability` (ADR-004).

### Raw card storage

If a `CardStorePlugin` is registered, raw card bytes are persisted alongside the parsed `AgentType`. This is optional — the system functions without it.

## Consequences

### Positive

- Agents need zero awareness of AgentLens — discovery is fully pull-based, preserving protocol-native deployments.
- Source independence — one failing source (e.g., K8s API timeout) does not block others or crash the cycle.
- Offline marking provides automatic catalog hygiene — disappeared agents are flagged without manual intervention.
- Push-created entries are protected — REST API registrations survive discovery cycles unchanged.
- Adding a new source requires one `SourcePlugin` file + one `pm.Register()` call (ADR-003).

### Negative / Trade-offs

- **Eventually consistent** — up to `pollInterval` delay between an agent appearing/disappearing and the catalog reflecting the change. No real-time notifications to the UI.
- **Sequential source processing** — a slow source delays others within the same cycle. Acceptable because `pollInterval` absorbs per-cycle latency, but could become a problem with many slow sources.
- **K8s List, not Watch** — full re-list every cycle is O(N) API calls where N = watched namespaces. Sufficient at current scale but does not scale to thousands of namespaces.
- **No response size limit on crawler** — a malicious or misconfigured endpoint returning a large response could cause memory pressure.
- **Hardcoded 10s crawler timeout** — not configurable per source or endpoint.

### Neutral

- Full-replacement capability updates (delete-all + re-insert) match the approach documented in ADR-004. Diff-based updates would reduce write I/O but add complexity for no measurable benefit at current scale.
- K8s source uses annotations on Services, not CRDs. This avoids requiring cluster-admin privileges to install a CRD but limits metadata to what fits in annotation values.

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| **WebSocket/SSE push from agents** | Reverses dependency direction — agents must know about AgentLens and maintain connections. Breaks protocol-native deployment model |
| **K8s Watch API** | More complex than List (reconnection logic, bookmark handling, partial event streams). Polling sufficient at current scale; Watch can be added later as an optimization |
| **Parallel source fetching** | Sequential is simpler and sources are I/O-bound. `pollInterval` absorbs per-cycle latency. Parallelism adds goroutine coordination complexity for marginal gain |
| **Exponential backoff on source failure** | Simple retry-on-next-cycle is sufficient. Backoff adds state tracking per source and risks delaying recovery when a transient issue resolves |
| **Circuit breaker per source** | Unnecessary complexity at current source count (2-3). A circuit breaker would prevent retries, but the cost of a failed retry is one logged error per cycle |
| **Event-driven via message queue** | External infrastructure dependency (Redis, NATS, etc.) for what is fundamentally a periodic sync. Overkill for catalog freshness requirements measured in minutes |
