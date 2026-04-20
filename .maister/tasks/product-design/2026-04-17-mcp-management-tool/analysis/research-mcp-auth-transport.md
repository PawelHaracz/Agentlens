# MCP Server Authentication and Transport Patterns

Research date: 2026-04-17. Primary source: MCP specification (current protocol version **2025-11-25**), with cross-references to prior revisions 2025-06-18 and 2024-11-05.

---

## 1. Transport Matrix

### 1.1 Protocol versions (as of April 2026)

MCP uses date-coded string versions (`YYYY-MM-DD`). Chronological timeline:

| Version | Status | Key transport points |
|---|---|---|
| `2024-11-05` | **Deprecated HTTP+SSE**. Backwards compatibility only. | Two-endpoint HTTP+SSE. |
| `2025-03-26` | Superseded. Default assumed version when no `MCP-Protocol-Version` header is sent. |
| `2025-06-18` | Superseded. First revision mandating OAuth 2.1 Resource Server role + RFC 8707 Resource Indicators. |
| **`2025-11-25`** | **Current**. Adds OIDC Discovery alongside RFC 8414, Client ID Metadata Documents (CIMD), SSE polling via server-initiated disconnect, HTTP header standardization (`MCP-Session-Id`), and `403 Forbidden` for invalid `Origin`. |

Clients and servers **MAY** support multiple versions simultaneously but **MUST** agree on one per session. Over HTTP, client **MUST** echo negotiated version via `MCP-Protocol-Version: <date>`; invalid value → `400 Bad Request`.

### 1.2 Transport options

| Transport | Standardized? | Network-accessible? | Notes |
|---|---|---|---|
| **stdio** | Yes. Clients **SHOULD** support whenever possible. | No. Subprocess; stdin/stdout JSON-RPC; stderr is logging. | Authorization spec explicitly does **not** apply. |
| **Streamable HTTP** | Yes — **recommended for remote servers**. Single MCP endpoint; POST (client→server JSON-RPC, response JSON or `text/event-stream`) + GET (server→client SSE). | Yes. | 2025-11-25 refined SSE handling (initial event ID + empty data, `retry` field, polling via `Last-Event-ID`). |
| **HTTP+SSE (legacy, 2024-11-05)** | **Deprecated.** | Yes. | Two-endpoint design. Backwards-compat path only. |
| **Custom** | Permitted (protocol is transport-agnostic). | Varies. | Must preserve JSON-RPC message format + lifecycle. |

### 1.3 Recommendation for network-accessible MCP server (AgentLens)

**Streamable HTTP only; advertise `MCP-Protocol-Version: 2025-11-25`.** SSE is not a separate transport in current spec — it's an optional upgrade of POST responses or server-initiated GET stream. No need for deprecated HTTP+SSE unless specific client (e.g., Cursor) requires it; probe-and-fallback can be added later.

### 1.4 Hard requirements (all normative, 2025-11-25)

- Single MCP endpoint accepting POST + GET.
- `Origin` header validation → `403 Forbidden` on invalid.
- Local bindings use `127.0.0.1`, not `0.0.0.0`.
- Clients **MUST** `Accept: application/json, text/event-stream` on POST.
- Session management via `MCP-Session-Id` (renamed from `Mcp-Session-Id` in 2025-06-18 per SEP-2243). Cryptographically secure (UUID/JWT/hash). Client **MUST** echo on every subsequent request. Server **MAY** expire anytime → `404 Not Found` → client re-initializes.
- Resumability via SSE event IDs + `Last-Event-ID`. IDs **MUST** be globally unique within session.
- Authorization **MUST** be included in every HTTP request — not just first.
- **Sessions MUST NOT be used for authentication.**

---

## 2. Auth Pattern Catalog

### 2.1 Spec-defined auth

Yes, as of 2025-06-18, refined 2025-11-25. MCP servers are classified as OAuth 2.1 Resource Servers.

- Authorization is **OPTIONAL** for MCP implementations. When supported over HTTP-based transport, implementations **SHOULD** conform.
- Not framed for stdio: "Implementations using STDIO transport **SHOULD NOT** follow this specification, and instead retrieve credentials from the environment."

Standards built on: OAuth 2.1 draft-13, RFC 8414 (AS metadata), RFC 7591 (Dynamic Client Registration — demoted to MAY in 2025-06-18), RFC 9728 (Protected Resource Metadata), RFC 8707 (Resource Indicators), IETF draft for OAuth Client ID Metadata Documents.

### 2.2 Canonical auth flow (user-delegated)

OAuth 2.1 authcode + PKCE + RFC 8707 resource indicator:

1. Client hits MCP endpoint without token.
2. Server responds `401 Unauthorized` with `WWW-Authenticate: Bearer resource_metadata="…/.well-known/oauth-protected-resource", scope="…"`. Scope hint is SHOULD (principle of least privilege).
3. Client fetches Protected Resource Metadata; discovers `authorization_servers[]`.
4. Client fetches AS metadata. In 2025-11-25 **MUST** try OAuth 2.0 AS Metadata first, then OpenID Connect Discovery.
5. Client registration priority: pre-registration → **CIMD (new in 2025-11-25)** → DCR (fallback) → user-entered.
6. Client runs OAuth 2.1 authcode+PKCE. PKCE `S256` is **MUST when technically capable**; clients **MUST verify** PKCE support via `code_challenge_methods_supported` in AS metadata.
7. Resource parameter is **MUST** in authorization and token requests — identifies canonical MCP server URI.
8. Token presented as `Authorization: Bearer <token>`. **MUST NOT** be in URI query string.
9. Server validates audience (RFC 8707 / RFC 9068) — reject tokens not issued for itself.

Insufficient-scope runtime error: `403` + `WWW-Authenticate: Bearer error="insufficient_scope", scope="..."`. Clients perform **step-up authorization flow** (incremental-consent model, SEP-835).

### 2.3 Non-user-delegated patterns

| Pattern | Status | Use |
|---|---|---|
| **Bearer token via `Authorization` header** | Core, MUST for HTTP. Every request. | Universal. |
| **OAuth 2.1 authcode + PKCE** | Core. | Interactive clients (IDEs, desktop). |
| **OAuth 2.0 Client Credentials** | Extension `io.modelcontextprotocol/oauth-client-credentials`. Supports `client_id`+`client_secret` and JWT Bearer Assertions (RFC 7523). | Background services, CI/CD, daemons. |
| **Enterprise-Managed Authorization (ID-JAG)** | Extension `io.modelcontextprotocol/enterprise-managed-authorization`. Enterprise IdP issues Identity Assertion JWT Authorization Grant. | Corporate SSO; centralized policy. |
| **API keys / custom headers** | Not spec-defined. Architecture overview: "supports standard HTTP authentication methods including bearer tokens, API keys, and custom headers. **MCP recommends using OAuth.**" | Fallback for clients that support custom-connector secret entry. |
| **Per-request claims / session-based** | **Explicitly forbidden for auth.** Sessions may scope user context, but server MUST validate bearer token every request. | n/a |
| **Token passthrough to downstream APIs** | **Explicitly forbidden.** | n/a |

### 2.4 Server-side validation rules (all MUST)

- Validate every token per OAuth 2.1 §5.2. Invalid/expired → 401.
- Audience validation vs. canonical MCP server URI (RFC 8707). Reject tokens for other audiences.
- Never accept or transit tokens issued by different AS.
- For upstream API calls, obtain separate token (server acts as OAuth client to upstream AS). Do not pass through client's token.
- HTTPS for all AS endpoints and redirect URIs (loopback localhost allowed).

---

## 3. Client UX

Current-state support for four auth capabilities (DCR, CIMD, Client Credentials, Enterprise-Managed):

| Client | OAuth for remote | Transports | DCR | CIMD | Client Creds | Enterprise-Managed |
|---|---|---|---|---|---|---|
| Claude.ai (web) | Yes — "Custom Connectors" UI, browser redirect | Remote (Streamable HTTP) | Yes | Yes | — | — |
| Claude Desktop | Yes — Custom Connectors | stdio + remote | Yes | — | — | — |
| Claude Code | Full spec support | stdio + HTTP | Yes | — | — | — |
| Cursor | OAuth via DCR | stdio + SSE | Yes | — | — | — |
| VS Code + Copilot | Most feature-complete. All transports, input-variable secrets, enterprise management via GitHub policies | all three | Yes | Yes | — | — |
| Continue | Tools/Resources/Prompts via `mcp.json` | stdio + HTTP | — | — | — | — |
| Codex (OpenAI) | "STDIO + HTTP streaming with OAuth" | stdio + HTTP | — | — | — | — |
| Archestra | Full enterprise stack | Gateway | Yes | Yes | — | Yes |

### 3.1 UX for remote server with OAuth (Claude.ai / Desktop)

1. Settings → Connectors → "Add custom connector"
2. Paste remote MCP URL (`https://…/mcp`)
3. Browser redirect OAuth flow (not bearer-token paste)
4. Tools/prompts/resources appear in composer
5. Per-tool permissions and usage limits in connector settings

**For AgentLens**: a properly-implemented server with Protected Resource Metadata + AS Metadata + DCR/CIMD gets **zero-config OAuth** from all major clients.

---

## 4. Multi-Tenant / Project-Scoped Authorization

MCP spec does **not** define a multi-tenancy model. Primitives available:

### 4.1 Tenant identification via canonical URI

RFC 8707 resource indicators bind tokens to canonical URI. Spec explicitly permits path-scoped forms:
- `https://mcp.example.com/mcp`
- `https://mcp.example.com/server/mcp`

**Spec-sanctioned multi-tenant model**: distinct endpoints per tenant/project. `https://agentlens.example.com/projects/{project_id}/mcp`. Each tenant URI is a separate OAuth resource; tokens are audience-bound per tenant. Protected Resource Metadata can be per-sub-path or root.

### 4.2 Per-request project claim in token

Single endpoint + `project_id` claim works but not explicitly covered. Spec mandates session-binding to user identity: sessions combined with unique user info (`<user_id>:<session_id>`). By analogy, permission + project_id claim must be validated server-side each request; cached per-session context must be keyed by `<tenant>:<user>:<session>`.

### 4.3 Enterprise-Managed as multi-tenant primitive

Enterprise-Managed Authorization extension is effectively multi-tenant: each enterprise IdP is a tenant, policies centrally administered, ID-JAGs carry claims the MCP AS maps to per-user per-server permissions.

### 4.4 Scope minimization as tenant isolation

Spec-discouraged broad scopes. Should be granular (`catalog:read`, `users:write` — directly analogous to AgentLens's existing permission format) and requested incrementally via step-up.

### 4.5 Reference implementation: Archestra

Described as "MCP registry/orchestrator, MCP gateway… Unified MCP gateway exposes single endpoint for orchestrating tools across remote and self-hosted MCP servers… Supports Enterprise-Managed Authorization… LLM proxy with deterministic context-aware tool guardrails… per-team cost tracking." Clearest shipping multi-tenant pattern: edge gateway fanning out to many backend MCP servers with per-tenant policy.

---

## 5. Observability, Rate Limiting, Audit

### 5.1 Spec coverage

MCP specs **do not define** rate limiting, audit logging formats, or OTel semantic conventions. Server-operator responsibilities. Normatively required:

- **Accountability / audit trail preservation**: reason token passthrough is forbidden — destroys audit integrity.
- **Rate limiting implied, not specified**: "MCP Server or downstream APIs might implement important security controls like rate limiting… that depend on token audience or other credential constraints."
- **Elevation-event logging**: "Log elevation events (scope requested, granted subset) with correlation IDs."
- **In-band logging primitive**: servers can send log messages to clients via `logging` — for connected LLM client only, not operator observability.

### 5.2 OpenTelemetry

No MCP spec text references OTel. Server-operator concern. AgentLens already ships OTel via `internal/telemetry/` (ADR-009); MCP surface reuses foundation — trace spans per JSON-RPC method, metrics per tool, structured logs per request.

### 5.3 Session hijacking detection (required)

Session IDs **MUST** be non-guessable; secure RNG; rotating/expiring reduces risk. Suspicious usage (different IP on same session ID, missing re-auth) should trigger alerts.

---

## 6. Recommendations for AgentLens MCP Server

### 6.1 Transport

- Streamable HTTP only, single path (e.g., `/api/mcp`), reuse chi middleware (Recovery, Logger, CORS, RequestID, otelhttp).
- Advertise `MCP-Protocol-Version: 2025-11-25`. Reject unknown versions with 400.
- Do not implement deprecated HTTP+SSE initially. Add only if Cursor compat proves required.
- Enforce `Origin` validation → 403 (aligns with CORS middleware role, matches 2025-11-25 clarification).

### 6.2 Authorization

**Treat AgentLens API server as both MCP Resource Server and Authorization Server.** Avoids second identity store; centralizes RBAC.

1. **Expose Protected Resource Metadata** at `/.well-known/oauth-protected-resource` pointing to AgentLens itself as `authorization_servers[0]`. Include `scopes_supported` with minimal set; let per-operation scope challenges escalate.
2. **Expose AS metadata** at `/.well-known/oauth-authorization-server`. Advertise `code_challenge_methods_supported: ["S256"]` (non-negotiable in 2025-11-25).
3. **Add OAuth 2.1 endpoints**: `/oauth/authorize`, `/oauth/token`, `/oauth/register` (DCR, optional feature flag).
4. **Token format**: reuse existing JWT (`golang-jwt/jwt/v5`). Mandatory claims:
   - `aud` = canonical MCP server URI (audience binding — non-negotiable audit/security boundary)
   - `scope` = space-separated RBAC permissions (mirrors AgentLens format)
   - `sub` = AgentLens user ID
   - `jti` = for revocation; short TTL (~15 min) with refresh rotation
5. **Reuse `RequirePermission` middleware** for MCP tool handlers. Token missing required permissions → `403` + `WWW-Authenticate: Bearer error="insufficient_scope", scope="users:write"`.
6. **Never accept** tokens not issued by AgentLens's own AS. If calling upstream agents, obtain separate token per upstream.

### 6.3 Multi-tenancy

AgentLens already has project parties. When exposing per-project:

- Prefer **path-scoped MCP endpoints** per project (`/projects/{project_id}/mcp`). Spec-sanctioned canonical URIs. Tokens audience-bound per tenant.
- Alternative: single endpoint + `project_id` scope claim, validated per request in middleware. Document whichever path is chosen; do not mix.
- Plan for Enterprise-Managed Authorization later. Keep JWT issuance composable so ID-JAG exchange can slot in.

### 6.4 Machine / CI/CD access

Add OAuth Client Credentials extension day-one for programmatic access:
- **`client_id` + `client_secret`** (new `api_clients` table, bcrypt-hashed secrets). Start here.
- **JWT Bearer Assertions (RFC 7523)** — follow later if customer needs.

Declare extension capability in `initialize`.

### 6.5 Client UX expectations

With proper Protected Resource Metadata + AS Metadata + DCR/CIMD, AgentLens MCP should "just work" with:
- Claude.ai Custom Connector (paste URL → browser OAuth → done)
- Claude Code (`claude mcp add` — DCR supported)
- VS Code + Copilot (workspace `mcp.json` or gallery install)
- Cursor (may need HTTP+SSE compat path if reported)
- Codex (OAuth compatible)

No manual bearer-token paste UX required for mainstream clients.

### 6.6 Hard requirements PR checklist

- [ ] `Origin` header validation → 403 on invalid
- [ ] `Authorization: Bearer` on every HTTP request, validated every time (no session-auth short-circuit)
- [ ] `aud` claim validation (reject JWTs issued for REST UI)
- [ ] `MCP-Protocol-Version` header echoed, invalid → 400
- [ ] `MCP-Session-Id` cryptographically secure UUID (not sequential); bind store entries by `<user_id>:<session_id>`
- [ ] `WWW-Authenticate` header on 401 with `resource_metadata` URI
- [ ] PKCE `S256` enforced, `code_challenge_methods_supported` in AS metadata
- [ ] No token passthrough for upstream calls
- [ ] Scope minimization: issue minimal scopes, step-up via 403 + scope challenge
- [ ] Tokens short-lived (~15 min), refresh tokens rotated
- [ ] OTel spans per JSON-RPC method + per tool; audit log per tool invocation with `user_id`, `tool_name`, `scope_granted`, `project_id`

### 6.7 Out of scope for MVP, plan for later

- Enterprise-Managed Authorization (ID-JAG) extension
- CIMD — nice-to-have; DCR sufficient for MVP
- SSE polling / long-lived connection tear-down (SEP-1699)
- `tasks` experimental extension for long-running agent operations

---

## 7. Sources

All primary sources on `modelcontextprotocol.io` (protocol version 2025-11-25 unless noted).

**Specification (normative):**
- [Transports — 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)
- [Authorization — 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization)
- [Security Best Practices — 2025-06-18](https://modelcontextprotocol.io/specification/2025-06-18/basic/security_best_practices)
- [Changelog — 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/changelog)
- [Changelog — 2025-06-18](https://modelcontextprotocol.io/specification/2025-06-18/changelog)

**Extensions:**
- [OAuth Client Credentials](https://modelcontextprotocol.io/extensions/auth/oauth-client-credentials)
- [Enterprise-Managed Authorization](https://modelcontextprotocol.io/extensions/auth/enterprise-managed-authorization)

**Developer docs:**
- [Architecture overview](https://modelcontextprotocol.io/docs/learn/architecture)
- [Versioning](https://modelcontextprotocol.io/docs/learn/versioning)
- [Connect to remote MCP servers](https://modelcontextprotocol.io/docs/develop/connect-remote-servers)
- [Example Clients](https://modelcontextprotocol.io/clients)

**Referenced RFCs:**
- [OAuth 2.1 draft-13](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1-13)
- [RFC 8414](https://datatracker.ietf.org/doc/html/rfc8414) — OAuth 2.0 AS Metadata
- [RFC 7591](https://datatracker.ietf.org/doc/html/rfc7591) — Dynamic Client Registration
- [RFC 9728](https://datatracker.ietf.org/doc/html/rfc9728) — Protected Resource Metadata
- [RFC 8707](https://www.rfc-editor.org/rfc/rfc8707.html) — Resource Indicators
- [RFC 9068](https://www.rfc-editor.org/rfc/rfc9068.html) — JWT Profile for OAuth
- [RFC 7523](https://datatracker.ietf.org/doc/html/rfc7523) — JWT Bearer Assertions
- [RFC 6750](https://datatracker.ietf.org/doc/html/rfc6750) — Bearer Token Usage
