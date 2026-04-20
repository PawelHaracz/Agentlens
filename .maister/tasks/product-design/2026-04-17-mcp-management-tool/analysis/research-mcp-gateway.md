# MCP Gateway / OpenAPI-to-MCP Converters

Research date: 2026-04-17.

> **⚠️ Source limitation**: In this session, only `modelcontextprotocol.io` hosts were allowlisted for WebFetch. GitHub repos, project docs sites (`gofastmcp.com`, `ibm.github.io`, `higress.io`, `speakeasy.com`), and WebSearch were denied. This document captures what was verifiable against the MCP spec. The **project-inventory portion** (stars, commit dates, licenses for named projects) is NOT verified here — marked `[UNVERIFIED]` where claims would require blocked sources. Relaunch this research with widened allowlist to complete.

---

## 1. MCP Protocol Primitives (spec-verified)

Relevant to any OpenAPI-to-MCP translator:

- **Server primitives**: Tools, Resources, Prompts
- **Client primitives**: Sampling, Roots, Elicitation
- **Experimental**: Tasks (durable execution wrapper)

Source: [architecture overview](https://modelcontextprotocol.io/docs/learn/architecture)

### Dynamic tool registration

Supported via capability `tools: { listChanged: true }` + `notifications/tools/list_changed`. Same pattern for resources and prompts. This is relevant because an OpenAPI-to-MCP gateway may want to re-emit its tool list when the wrapped OpenAPI spec changes at runtime.

Source: [architecture overview](https://modelcontextprotocol.io/docs/learn/architecture)

### Transport requirements (for gateway design)

- `MCP-Protocol-Version: <date>` header required on every HTTP request post-init (default `2025-03-26` if missing)
- `Mcp-Session-Id` → renamed `MCP-Session-Id` in 2025-06-18 (SEP-2243)
- 404 on session → client must re-initialize
- Schema default dialect: JSON Schema 2020-12 (SEP-1613)

Source: [transports 2025-06-18](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)

### Input validation surface (SEP-1303)

Input validation errors are conveyed as tool execution errors (structured), not transport errors. Gateway must map OpenAPI 4xx validation failures into this shape.

---

## 2. Project Inventory `[UNVERIFIED]`

The following projects were in the research brief; their metadata (stars, last-commit, license, deployment model) could not be verified in this session because GitHub was blocked. **Do not treat the descriptions below as ground truth — they are based on assistant's training data and may be stale or wrong.** Re-verify before citing in design decisions.

| Project `[UNVERIFIED]` | URL `[UNVERIFIED]` | Training-data claim |
|---|---|---|
| IBM mcp-context-forge | https://github.com/IBM/mcp-context-forge | Large OSS gateway; virtual servers, federation, transport translation |
| FastMCP (Python) | https://github.com/jlowin/fastmcp | Has `FastMCP.from_openapi()` — OpenAPI → MCP tools at import time |
| snaggle-ai/openapi-mcp-server | https://github.com/snaggle-ai/openapi-mcp-server | Node.js; OpenAPI → MCP adapter |
| Higress MCP Gateway | https://higress.cn/ | Envoy-based API gateway with MCP support; enterprise-leaning |
| Speakeasy MCP | https://speakeasy.com | SaaS code-gen; likely build-time MCP-server generation from OpenAPI |
| mark3labs/mcp-go | https://github.com/mark3labs/mcp-go | Go-native MCP server library — **relevant for AgentLens because AgentLens is Go** |
| metoro-io/mcp-golang | https://github.com/metoro-io/mcp-golang | Alternative Go MCP library |

**Action item before implementation**: verify the Go MCP libraries (`mark3labs/mcp-go`, `metoro-io/mcp-golang`) against actual repo state — stars, last activity, API surface, license, whether they support Streamable HTTP transport and OAuth 2.1 protected resource metadata. Whichever is embeddable into a chi router is the most interesting for AgentLens.

---

## 3. Architectural Patterns (inferred from MCP spec)

Any OpenAPI-to-MCP gateway must solve these translation problems:

### 3.1 Translation timing

- **Build-time (code generation)**: ingest OpenAPI spec, generate Go/TS/Python MCP server code with one tool per operation. Pro: no runtime overhead. Con: spec changes require rebuild.
- **Runtime (dynamic translation)**: load spec at startup, expose tools that are invoked via an HTTP client against the underlying API. Pro: spec changes hot-reload (with `notifications/tools/list_changed`). Con: runtime overhead, schema drift risk.

For AgentLens (self-referential gateway): runtime is a poor fit for the REST API because schemas don't change without a deploy. Build-time (or "server-start-time") translation — enumerate routes at kernel init and register MCP tools — is cleanest.

### 3.2 Operation → tool name mapping

OpenAPI `operationId` → MCP tool `name`. Constraints:
- MCP tool names are short identifiers (SEP-986 format rules)
- Must be unique across the server's tool list
- Should encode resource + verb for LLM discoverability: e.g., `catalog_list`, `catalog_create`, `users_delete`
- Avoid deep nesting: `api_v1_catalog_entries_{id}_projects_{projectId}_delete` is worse than `catalog_remove_from_project`

### 3.3 Parameter schema translation

OpenAPI parameters (path/query/header/cookie/body) → MCP tool `inputSchema` (JSON Schema 2020-12).
- Flatten: tool inputs should be one object; path/query/body merged with distinct property names.
- Default dialect is 2020-12 per SEP-1613.
- Constrain enums explicitly (LLMs pick correctly only with narrowed types).
- Make required fields required in schema; avoid `anyOf`/`oneOf` that confuse LLMs.

### 3.4 Auth passthrough strategy

Per spec: **no token passthrough**. The gateway must obtain its own token (acting as OAuth client) when calling upstream. For AgentLens wrapping its OWN API, this is moot — the MCP server can call handlers directly in-process, no second token needed.

### 3.5 Error mapping

OpenAPI HTTP status → MCP error:
- 400 → InvalidRequest (or tool-execution error per SEP-1303)
- 401 → call-level 401 + `WWW-Authenticate`
- 403 → 403 + scope challenge
- 404 → ResourceNotFound (tool-execution error)
- 409 → tool-execution error with structured field
- 500 → InternalError

For AgentLens: in-process call avoids HTTP translation; map returned `error` field to MCP tool-execution error.

### 3.6 Known limitations of OpenAPI→MCP

Areas where naive translation breaks:
- **Auth schemes** that don't translate (API-key-in-query, complex OAuth flows with refresh)
- **Binary / streaming responses** (file download, WebSocket upgrade)
- **Long-running operations** (202 + Location header polling) — may need MCP `tasks` extension
- **Pagination** via opaque cursors vs. offset — must preserve cursor semantics
- **Nested schemas** / polymorphic request bodies (`oneOf`/`anyOf`) — LLMs struggle
- **File uploads** (multipart/form-data)

For AgentLens: the REST API is well-behaved (JSON, bearer auth, offset pagination), but some endpoints have cursor-like behavior worth verifying.

---

## 4. Trade-offs: Gateway vs. Hand-coded MCP Tools

| Dimension | OpenAPI-to-MCP gateway (wraps all routes) | Hand-coded MCP tools (curated subset) |
|---|---|---|
| Coverage | Automatic — every endpoint becomes a tool | Manual — 20-30 tools max to avoid tool-list bloat |
| LLM usability | Often poor — 60+ auto-named tools overwhelm tool-selection | Excellent — each tool designed for LLM consumption |
| Schema quality | Only as good as OpenAPI spec | Tailored descriptions, narrowed enums, hand-picked params |
| Safety | Every DELETE exposed; confirm flags absent | Destructive ops get dry-run, elicitation, or exclusion |
| Maintenance | Zero per-endpoint cost; automatic on new routes | Each new tool needs explicit wiring |
| Prerequisite | Requires OpenAPI spec (AgentLens has **none** today) | No OpenAPI spec needed |
| Time to ship | Medium — need OpenAPI generator + translator | Fast for small set, slow for wide coverage |
| Bundle size | All 61 endpoints as tools | Curated subset (est. 15-25 tools for admin workflows) |

---

## 5. Recommendations for AgentLens Context

Based on MCP spec + AgentLens codebase findings:

### 5.1 Start narrow, not as a gateway

AgentLens has **no OpenAPI spec today**. Adopting a full OpenAPI-to-MCP translation path would require:
1. Adopt an OpenAPI emission framework (huma, or similar) — refactor all chi handlers
2. Build or adopt an OpenAPI→MCP translator that handles auth discovery metadata (Protected Resource Metadata, AS metadata)
3. Ship all 61 endpoints as tools — LLM-hostile tool list
4. Add safety layer over destructive operations

This is 3-6 months of work before the first useful MCP tool ships.

**Recommendation**: ship a hand-coded MCP tool for a curated set (catalog management, validation, project assignment, capability discovery — ~15 tools). This is weeks of work and ships value immediately.

### 5.2 If a gateway is desired later

Treat as Phase 2 after the hand-coded MVP. Requires:
- Adopt OpenAPI emission (candidate: huma) — this is a **prerequisite** for any generalized gateway, not MCP-specific
- Evaluate whether `mark3labs/mcp-go` or `metoro-io/mcp-golang` are production-ready for Streamable HTTP with OAuth 2.1 PRM/ASM discovery `[UNVERIFIED]`
- Decide between build-time code-gen vs. runtime translation (for AgentLens, runtime-at-start is sufficient — spec fixed per deploy)

### 5.3 Go MCP library evaluation

The single most important unknown is which Go MCP library (if any) supports:
- Streamable HTTP transport (2025-11-25)
- Embeddable into chi router (not standalone process)
- JSON Schema 2020-12
- `notifications/tools/list_changed`
- OAuth 2.1 Protected Resource Metadata / AS Metadata discovery endpoints

If none exist, we implement the MCP protocol directly — feasible but adds 2-4 weeks.

**Blocked research**: verify `mark3labs/mcp-go` and `metoro-io/mcp-golang` against these criteria. Requires GitHub access.

---

## 6. Sources Consulted

**Spec (verified):**
- [MCP landing page](https://modelcontextprotocol.io/)
- [llms.txt index](https://modelcontextprotocol.io/llms.txt) — lists all active SEPs
- [Specification 2025-06-18](https://modelcontextprotocol.io/specification/2025-06-18)
- [Specification 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25)
- [Transports 2025-06-18](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)
- [Architecture overview](https://modelcontextprotocol.io/docs/learn/architecture)

**Blocked (require allowlist expansion to verify):**
- `github.com/*` — all project repos
- `raw.githubusercontent.com` — README content
- `gofastmcp.com` — FastMCP docs
- `ibm.github.io` — mcp-context-forge docs
- `higress.io`, `higress.cn` — Higress docs
- `speakeasy.com` — Speakeasy MCP generator

**Relevant SEPs (from llms.txt, pending full review):**
- SEP-2243 — HTTP header standardization
- SEP-1699 — SSE polling via server-side disconnect
- SEP-1686 — Tasks (durable execution)
- SEP-1303 — Input validation errors as tool execution errors
- SEP-986 — Tool name format
- SEP-1613 — JSON Schema 2020-12 as default dialect
