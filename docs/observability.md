# MCP Discovery Server — Observability

## OTel Spans

All spans use the `agentlens.mcp` tracer scope.

| Span | Triggered by | Key Attributes |
|---|---|---|
| `agentlens.mcp.initialize` | MCP `initialize` handshake | `principal_id`, `principal_type`, `session_id` |
| `agentlens.mcp.tool_call` | `tools/call` dispatch | `tool.name`, `principal_id`, `status` |
| `agentlens.mcp.authdispatch` | Auth middleware chain | `auth_method`, `outcome` |
| `agentlens.mcp.jwks_refresh` | Dex JWKS forced refresh | `provider`, `stale` |

---

## OTel Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `agentlens_mcp_invocations_total` | Counter | — | Total JSON-RPC requests received |
| `agentlens_mcp_tool_calls_total` | Counter | `tool` | Tool/call dispatches by tool name |
| `agentlens_mcp_active_sessions` | Gauge | — | Current non-revoked, non-expired sessions |
| `agentlens_mcp_credcache_hits_total` | Counter | — | API-key bcrypt cache hits |
| `agentlens_mcp_credcache_misses_total` | Counter | — | Cache misses (full bcrypt compare) |
| `agentlens_mcp_last_seen_drops_total` | Counter | — | Async last_seen_at updates dropped (channel full) |
| `agentlens_federation_jwks_stale_serves_total` | Counter | — | Stale JWKS served on refresh failure |

---

## Structured Audit Log

Every tool invocation emits a structured `slog.InfoContext` entry at level INFO:

```json
{
  "msg": "mcp.audit",
  "principal_id": "<id>",
  "principal_kind": "service_account",
  "auth_method": "api_key",
  "tool": "agent_search",
  "project_ids": "proj-1,proj-2",
  "outcome": "success",
  "ts": "2026-04-18T12:00:00Z"
}
```

Secret material is never logged. Disable with `mcp_server.audit_enabled=false` (startup WARN emitted).

---

## Operator Alert Recipes

### High tool-call error rate

```promql
rate(agentlens_mcp_tool_calls_total{status="error"}[5m])
  / rate(agentlens_mcp_tool_calls_total[5m]) > 0.05
```

### Dex JWKS stale serves (federation health degraded)

```promql
increase(agentlens_federation_jwks_stale_serves_total[10m]) > 0
```

### Bcrypt cache miss rate high (p95 latency risk)

```promql
rate(agentlens_mcp_credcache_misses_total[5m])
  / (rate(agentlens_mcp_credcache_hits_total[5m]) + rate(agentlens_mcp_credcache_misses_total[5m])) > 0.5
```

### Active session count anomaly

```promql
agentlens_mcp_active_sessions > 1000
```

---

## Health Endpoints

| Endpoint | Auth | Description |
|---|---|---|
| `GET /healthz` | None | Liveness (process alive) |
| `GET /readyz` | None | Readiness (DB ping; + Dex JWKS when federation enabled) |
| `GET /api/mcp/status` | None | MCP plugin status + active session count |
