# MCP Tool Design for Admin / Management APIs — Research Findings

**Task**: `.maister/tasks/product-design/2026-04-17-mcp-management-tool`
**Research question**: When exposing a REST admin API (CRUD + a handful of non-CRUD
actions) through MCP, how should we design the tool surface for LLM callers — granularity,
safety, idempotency, destructive-action confirmation, resource-vs-tool split, schema,
pagination, and audit?
**Scope**: principles, not a list of AgentLens endpoints. The same heuristics should apply
whether we build a hand-curated server, auto-register against the default project, or
ship a generalised OpenAPI-to-MCP gateway.
**Date**: 2026-04-17

---

## 0. Source availability note

Primary evidence below comes from the current MCP specification (revision `2025-06-18`)
at `modelcontextprotocol.io/specification/2025-06-18/...` and the "server concepts"
learning docs at `modelcontextprotocol.io/docs/concepts/...`. These were fetched directly.

Two source classes could **not** be fetched during this pass and are therefore either
omitted or clearly marked as secondary:

- `github.com` / `raw.githubusercontent.com` and `docs.github.com` (blocked for
  WebFetch/WebSearch in this session). Claims about the GitHub reference server and the
  filesystem reference server are therefore limited to what the MCP docs themselves say.
- `anthropic.com` and `docs.anthropic.com` (blocked). No Anthropic engineering-post
  quotes are cited; where a principle comes from general LLM-tool-use heuristics it is
  labelled `[heuristic]` rather than `[spec]`.

This means the schema-design section cites MCP's own JSON-Schema conventions, not a
specific "Writing effective tools" blog post. If the post's exact wording is required,
fetch it separately and add a citation block.

---

## 1. Primitive selection — Tool vs. Resource vs. Prompt

### 1.1 What the MCP spec says, verbatim

The three server-side primitives are owned by three different actors:

| Primitive | Who controls it | Typical use |
| --- | --- | --- |
| **Tools** | **Model** — "Functions that your LLM can actively call, and decides when to use them based on user requests. Tools can write to databases, call external APIs, modify files, or trigger other logic." | Search flights, send messages, create calendar events |
| **Resources** | **Application** — "Passive data sources that provide read-only access to information for context, such as file contents, database schemas, or API documentation." | Retrieve documents, access knowledge bases, read calendars |
| **Prompts** | **User** — "Pre-built instruction templates that tell the model to work with specific tools and resources." | "Plan a vacation", "Summarize my meetings" |

— `modelcontextprotocol.io/docs/concepts/server-concepts` (Core Server Features table).

The spec is explicit about control flow:

- Tools are **model-controlled**: "the language model can discover and invoke tools
  automatically based on its contextual understanding". (`specification/2025-06-18/server/tools`)
- Resources are **application-driven**: "host applications determining how to
  incorporate context based on their needs" (tree/list views, search, automatic
  inclusion). (`specification/2025-06-18/server/resources`)
- Prompts are **user-controlled**: "exposed from servers to clients with the intention
  of the user being able to explicitly select them for use ... typically triggered
  through user-initiated commands". (`specification/2025-06-18/server/prompts`)

### 1.2 Why the "GET → resource" reflex is mostly wrong for admin APIs

A naïve REST-to-MCP mapping assumes `GET` becomes a resource and everything else
becomes a tool. The spec does **not** require that; it ties the split to *who* should
decide when the data is read:

- A resource is discovered via `resources/list` and fetched via `resources/read` — both
  driven by the **host**, not the model. The UX assumption is a resource picker, tree
  view, or automatic inclusion heuristic (`specification/2025-06-18/server/resources`,
  "User Interaction Model").
- For an admin surface used **by an agent executing a task** ("disable the user who
  leaked the key"), the model needs to decide *when* to read. That's a tool-shaped
  interaction, not a resource-shaped one.

Signals that a read is better as a **tool**:
- The model must decide *when* to read, not the human operator.
- The read takes parameters the user doesn't want to pick from a picker (filters,
  searches, IDs).
- Freshness matters and the host wouldn't pre-load this blob into context.
- The result is small and the agent will consume it immediately, not return to it.

Signals that a read is better as a **resource**:
- The object has a stable URI the user might want to cite, subscribe to, or re-fetch
  (spec: "unique URI (e.g., `file:///path/to/document.md`)", "fixed URIs that point to
  specific data" and "Resource Templates ... with parameters for flexible queries").
- The same item will be referenced repeatedly and benefits from
  `resources/subscribe` + `notifications/resources/updated` for change-tracking (spec:
  "Subscriptions" section).
- The content is large, semi-static, and better streamed as context than as a tool-call
  round-trip.
- You want pickers / auto-completion in the client UI (Resource Templates support
  parameter completion via `completion/complete`).

### 1.3 When **prompts** are the right primitive

Prompts shine for *workflows* — canonical recipes that bundle several tools and a few
pieces of context. The spec's example is a `plan-vacation` prompt that drives
`searchFlights`, `checkWeather`, `bookHotel`, `createCalendarEvent` in sequence
(`docs/concepts/server-concepts` §"Bringing Servers Together").

For an admin server, candidates for prompts are "offboard a user", "rotate a tenant's
keys", "investigate a noisy agent": multi-step operator recipes the user *selects*, not
that the model invents. Prompts are surfaced through slash commands / command palettes
(spec shows a `/plan-vacation` screenshot). They are *not* a substitute for per-call
confirmation (that's the client's job) but they *are* a way to make the "blessed path"
obvious and deterministic.

### 1.4 Decision heuristics

| Situation | Primitive | Reason |
| --- | --- | --- |
| Read that the model decides to issue mid-task (list, filter, search) | **Tool** | Model-controlled; small, bounded, returned into the reasoning loop. |
| Stable object the user picks ("show me this ADR") | **Resource** (template) | URI-addressable, supports subscribe + completion. |
| Mutation with side effects | **Tool** | Tools are the only primitive that can mutate state. |
| Recurring multi-step operator workflow | **Prompt** (that invokes tools) | User-controlled entry point; the spec explicitly models workflows this way. |
| "What configuration keys exist?" (schema) | **Resource** | Large, relatively static, useful as context rather than per-call. |
| "What values does config key X have right now?" | **Tool** | Dynamic, parameterised, agent-driven. |

Rule of thumb: if you wouldn't reasonably put it behind a resource picker, it's a tool.

---

## 2. Tool granularity — coarse vs. fine

### 2.1 What the spec mandates (very little)

The spec only requires that each tool has a unique `name`, a JSON Schema
`inputSchema`, and (optionally) a `title`, `description`, `outputSchema`, and
`annotations` (`specification/2025-06-18/server/tools`, "Data Types → Tool"). The
example tool names in the docs follow a `verb_noun` or `noun_verb` convention:
`get_weather`, `calculator_arithmetic`, `weather_current`, `searchFlights`. The spec
explicitly flags that tool names should be clear enough to disambiguate:

> "A unique identifier for the tool within the server's namespace. This serves as the
> primary key for tool execution and **should follow a clear naming pattern** (e.g.,
> `calculator_arithmetic` rather than just `calculate`)"
>
> — `specification/2025-06-18/server/tools`, "Understanding the Tool Discovery Response"

### 2.2 Three granularity postures — trade-offs

| Posture | Shape | Pros | Cons |
| --- | --- | --- | --- |
| **Fine — one tool per verb** | `user_create`, `user_update`, `user_delete`, `user_get`, `user_list` | Schema exactly mirrors the operation; clear `annotations` (`readOnlyHint`, `destructiveHint`) per tool; easy to toggle off destructive ones. | Tool-list bloat (tools count grows linearly with resources × verbs); more discovery tokens; the model must pick the right verb. |
| **Coarse — one tool per resource** | `user(action: "create"|"update"|"delete"|"get"|"list", ...)` | Fewer tools, smaller tool-list. | One schema must express every verb's params → many optional fields and cross-field conditionals; one `annotations` block must over-warn (anything that contains a destructive branch is treated as destructive); harder for clients to whitelist the safe subset. |
| **Mixed — CRUD split by read/write, plus strategic consolidations** | `user_read` (combines get + list + search), `user_write` (create + update), `user_delete` (separate). | Keeps destructive ops in their own tool (clean annotation); read surface is one tool instead of two; write surface is one tool instead of two; delete stays isolated. | Still need conditional schemas for the write tool; loses the 1:1 mapping reviewers expect. |

### 2.3 What the LLM actually struggles with

These failure modes are the reason the model-facing schema shape matters:

- **Confusable tool names.** The spec's own example warns against shortening
  `calculator_arithmetic` to `calculate`. For admin APIs that means `user_delete` is
  safer than `remove`, and `project_archive` is safer than `retire`. `[spec, heuristic]`
- **Hallucinated parameters.** A tool like `user(action, payload)` with a free-form
  `payload: object` invites the model to invent fields. A narrower schema with
  `additionalProperties: false`, explicit enums, and minimal `required` drops this
  failure mode. `[heuristic, schema-design]`
- **Wrong-verb selection.** With a coarse tool, the model must pick `"delete"` over
  `"archive"` — a single-word decision. With fine tools, that decision is the tool
  name, which is bolder and more prominent in the tool list. `[heuristic]`
- **Tool-list bloat.** The spec supports **listChanged notifications** and
  **pagination** on `tools/list` (`specification/2025-06-18/server/utilities/pagination`),
  which means servers can legitimately expose 50+ tools with dynamic visibility. The
  GitHub MCP server famously groups tools into "toolsets" and supports a dynamic
  discovery mode for this reason (secondary source; see §0).

### 2.4 Recommendation for admin-ish APIs

For a surface that is CRUD-dominated with a handful of non-CRUD actions (rebuild index,
refresh health, bootstrap admin), the best default is:

- **One tool per (resource, verb)** for the CRUD verbs — `user_create`,
  `user_update`, `user_delete`, `user_get`, `user_list` — because this keeps the
  annotation per-tool honest and lets clients display destructive ones with a different
  UI. `[spec — annotations are per-tool]`
- **Merge `get` and `list` into a single read tool** only when the semantics are
  compatible (e.g., `user_find(id?, filter?)` returns one or many). Otherwise keep them
  split so required-field constraints stay simple.
- **Keep non-CRUD actions as their own tools** (`discovery_rebuild`, `health_refresh`,
  `admin_bootstrap`). They have distinct side-effects, usually distinct permissions,
  and their `annotations` differ (idempotent vs. not, destructive vs. not).
- **Namespace every tool.** The spec uses `weather_current` and `calculator_arithmetic`
  to show namespacing by resource. Follow the same pattern so names are unique across
  toolsets and so clients can pattern-match (`user_*`, `project_*`).
- **Support "toolsets" / read-only mode.** Expose a server-side switch that filters
  `tools/list` down to `readOnlyHint: true` tools. MCP allows this natively because
  `tools/list` is dynamic and `notifications/tools/list_changed` advertises reshuffles
  (`specification/2025-06-18/server/tools`, "List Changed Notification"). This is the
  right hook for an "agent mode" vs. "operator mode" toggle.

---

## 3. Safety for destructive actions

### 3.1 The human-in-the-loop rule is SHOULD, not MUST

The spec's tools page carries a Warning that is careful about normative strength:

> "For trust & safety and security, there **SHOULD** always be a human in the loop with
> the ability to deny tool invocations. Applications **SHOULD**:
>
> - Provide UI that makes clear which tools are being exposed to the AI model
> - Insert clear visual indicators when tools are invoked
> - Present confirmation prompts to the user for operations, to ensure a human is in
>   the loop"
>
> — `specification/2025-06-18/server/tools`

The client-side security list expands this:

> "Clients **SHOULD**: Prompt for user confirmation on sensitive operations; Show tool
> inputs to the user before calling the server, to avoid malicious or accidental data
> exfiltration; Validate tool results before passing to LLM; Implement timeouts for
> tool calls; Log tool usage for audit purposes"
>
> — `specification/2025-06-18/server/tools`, §"Security Considerations"

Two consequences for an admin server:

1. **Confirmation-before-execute UI is the client's responsibility, not the server's.**
   Servers cannot rely on it, because not every client renders one.
2. **The server still has to do its own safety work** — the spec's
   "Servers MUST validate all tool inputs, implement proper access controls, rate limit
   tool invocations, sanitize tool outputs" is phrased as server obligations, not
   client obligations.

### 3.2 Tool annotations the client uses to decide UI treatment

MCP `ToolAnnotations` lets the server declare behavioural hints that clients use to
choose UI and policy. The field is part of the Tool type
(`specification/2025-06-18/server/tools`, "Data Types → Tool"):

> "`annotations`: optional properties describing tool behavior"
>
> Warning: "clients **MUST** consider tool annotations to be untrusted unless they come
> from trusted servers."
>
> — `specification/2025-06-18/server/tools`, "Data Types → Tool"

The canonical hint names published in the MCP JSON schema and widely supported across
SDKs are:

- `readOnlyHint` — the tool does not modify state.
- `destructiveHint` — the tool **may** perform destructive operations (defaults to
  true when unset, in the canonical schema, so reads must be explicitly flagged
  read-only).
- `idempotentHint` — repeated identical calls have no additional effect.
- `openWorldHint` — the tool interacts with systems outside the current process
  (network, external services).

Important caveat directly from the spec: these are **hints**. The Warning that clients
"MUST consider tool annotations to be untrusted" means the server cannot *rely* on
annotations to gate invocation — they guide UI only. Anything the server must enforce
(auth, rate-limit, rollback) has to be enforced in the tool handler itself. `[spec]`

### 3.3 Server-side safety patterns that are client-agnostic

Because confirmation UI is optional, robust admin servers typically layer server-side
mechanisms. None are mandated by the spec; they are the accumulated pattern set:

| Pattern | Shape | When to use |
| --- | --- | --- |
| **Dry-run flag** | `"dry_run": { "type": "boolean", "default": true }` on destructive tools. Execution returns the diff / intended mutations without applying them. | Bulk operations, anything that touches >1 record, anything with cascade. |
| **Two-step preview-apply** | Split into `user_delete_preview` (read-only, returns a `plan_id` and effects summary) and `user_delete_apply({ plan_id, confirm_token })`. Server validates the plan still matches current state. | Very high-blast-radius ops (tenant deletion, role revocation across many principals). |
| **Explicit confirm flag** | `"confirm": { "const": true }` or a string token the operator quotes back (`"confirm": "DELETE john@example.com"`). The tool refuses otherwise. | Single-record destructive ops where a preview is overkill but "did the model mean it?" still matters. |
| **Optimistic concurrency** | Require an `if_match` / version / updated-at parameter; server rejects 409 if it moved. | Any update to shared state. |
| **Soft-delete with TTL** | `delete` marks the record; a nightly job reaps. A `restore` tool exists for the TTL window. | User-visible objects where accidental deletion is expensive. |
| **Quota / rate limits on destructive tools** | Server-enforced max N deletes per minute per identity. | Anything an agent could run in a loop. |
| **Scope-gated visibility** | Destructive tools only appear in `tools/list` when the caller's token carries a privileged scope; `notifications/tools/list_changed` advertises the reshuffle after auth. | When different operator roles talk to the same server. |

The spec itself endorses the scope-gated approach in §"Scope Minimization"
(`specification/2025-06-18/basic/security_best_practices`):

> "Implement a progressive, least-privilege scope model: Minimal initial scope set
> (e.g., `mcp:tools-basic`) containing only low-risk discovery/read operations;
> Incremental elevation via targeted `WWW-Authenticate` `scope="..."` challenges when
> privileged operations are first attempted".

### 3.4 Elicitation — a spec-native "ask before executing"

MCP has a client primitive that is underused but exactly fits destructive
confirmations: **elicitation**
(`specification/2025-06-18/client/elicitation`).

A server can mid-tool-call issue `elicitation/create` with a JSON-Schema-described
prompt and wait for `accept` / `decline` / `cancel`. The spec's three-action model is
explicit:

> "1. **Accept** (`action: "accept"`): User explicitly approved and submitted with data
> — Example: User clicked 'Submit', 'OK', 'Confirm', etc.
>
> 2. **Decline** (`action: "decline"`): User explicitly declined the request — Example:
> User clicked 'Reject', 'Decline', 'No', etc.
>
> 3. **Cancel** (`action: "cancel"`): User dismissed without making an explicit choice
> — Example: User closed the dialog, clicked outside, pressed Escape, etc."
>
> — `specification/2025-06-18/client/elicitation`

For a destructive admin tool, that means you can do server-driven confirmation without
depending on whether the specific MCP client renders a "Run tool?" dialog:

```
tool: user_delete(email="john@example.com", confirm=false)
 → server: elicitation/create { message: "Delete john@example.com and their 12
    API keys? Type DELETE to confirm.", requestedSchema: { ... "confirm": { enum: ["DELETE"] } } }
 → client: action=accept | decline | cancel
 → server: applies / aborts
```

Three caveats from the spec:

1. Elicitation requires the client to advertise the capability:
   `"capabilities": { "elicitation": {} }` during `initialize`. Servers must degrade
   gracefully (e.g., fall back to the `confirm` flag) when the client doesn't support
   it.
2. "Servers **MUST NOT** use elicitation to request sensitive information." So don't
   ask for a password or API key that way — ask for an affirmation.
3. The elicitation schema is intentionally limited ("flat objects with primitive
   properties only"). Good enough for a confirm token; not a place to stage a full
   edit form.

### 3.5 Partial-failure behaviour in batch tools

If a tool takes a list (`user_delete_many(emails)`), two spec-aligned behaviours are
valid:

- **All-or-nothing** in a transaction. Return `isError: true` on any failure with a
  structured content block listing which item failed.
- **Best-effort** with per-item results. Return `isError: false`, but
  `structuredContent` carries `{ succeeded: [...], failed: [{ id, reason }] }`, and the
  human-readable `text` block makes that explicit. The spec expects structured output
  to co-exist with a text block "for backwards compatibility"
  (`specification/2025-06-18/server/tools`, "Structured Content").

Prefer all-or-nothing for destructive batches unless the domain actually benefits from
partial success (e.g., "refresh health for these 50 endpoints"). Batch tools should
always carry an `idempotentHint: true` annotation when re-execution after a failure is
safe.

---

## 4. Schema design for LLM callers

### 4.1 What the spec *requires*

- `inputSchema` must be valid JSON Schema (`specification/2025-06-18/server/tools`,
  "Data Types → Tool").
- `outputSchema` is optional but, when provided, "Servers **MUST** provide structured
  results that conform to this schema" and "Clients **SHOULD** validate structured
  results against this schema" (ibid, "Output Schema").
- `name` is the stable key; `title` is optional display.

### 4.2 Schema checklist for admin tools

Each item below is either spec-derived `[spec]` or a widely-applied heuristic from the
JSON-Schema / LLM-tool-use community `[heuristic]`. They compound; skip at your own
risk.

1. **Top-level description tells the model when to use the tool, not just what it
   does.** The spec's example descriptions start with a verb and include domain
   context ("Search for available flights", "Get current weather data for a
   location"). `[spec example]`
2. **Every property has a `description`.** The spec's example
   (`specification/2025-06-18/server/tools`, Listing Tools example) uses
   `"description": "City name or zip code"` — not just `"type": "string"`. Undescribed
   params are a hallucination magnet. `[spec example + heuristic]`
3. **Narrow with `enum` wherever the set is closed.** From the spec's
   `searchFlights`-style examples, `units` uses
   `{ "type": "string", "enum": ["metric", "imperial", "kelvin"], "default": "metric" }`.
   For admin APIs, enumerate role names, status values, sort orders. `[spec example +
   heuristic]`
4. **Minimise `required`.** Every required field is a failure mode if the LLM can't
   deduce the value. For a delete-by-email tool, `email` is required; `reason` is
   optional. `[heuristic]`
5. **Use `default` for ergonomic options.** If 95% of list queries want
   `limit=50, sort="updated_at:desc"`, set those defaults so the LLM can omit them.
   `[heuristic]`
6. **Set `additionalProperties: false`.** Otherwise hallucinated extras are silently
   accepted. `[heuristic]`
7. **Prefer primitives over objects in inputs.** The spec's elicitation schema is
   limited to "flat objects with primitive properties only" — that's because flat
   schemas are easier for both client UIs and LLMs. Inherit the habit even where the
   tool spec doesn't require it. `[spec — elicitation limitation]`
8. **Provide an `outputSchema` for anything the agent will reason over.** The spec's
   rationale: "Enabling strict schema validation of responses; Providing type
   information for better integration with programming languages; Guiding clients and
   LLMs to properly parse and utilize the returned data". Always pair
   `structuredContent` with a `text` block (spec: "SHOULD also return the serialized
   JSON in a TextContent block"). `[spec]`
9. **Descriptions should state side-effects.** e.g., "Deletes the user and revokes all
   of their API keys. This operation is not reversible." This is information the LLM
   uses to decide whether to call the tool at all. `[heuristic, reinforced by
   destructiveHint]`
10. **Descriptions should state constraints and errors.** "Returns 409 Conflict if
    another update happened since the provided `if_match` revision." The LLM can plan
    a retry when it knows the shape of failure. `[heuristic]`

### 4.3 Naming conventions

From the spec's examples (`get_weather`, `calculator_arithmetic`, `weather_current`,
`searchFlights`) and the warning against abbreviating `calculator_arithmetic` to
`calculate`:

- Use `<resource>_<verb>` or `<verb>_<resource>` consistently — pick one and stick.
  Mixed casing/order is a discoverability problem.
- Prefer full words over abbreviations. `user_delete` beats `usr_del`.
- Include the resource, even when "obvious". `delete` is a tool name that will
  collide; `user_delete` will not.
- For actions that cross resources, be explicit: `discovery_rebuild_for_project` is
  clearer than `rebuild` or `project_rebuild`.

---

## 5. Pagination, filtering, search

### 5.1 MCP's own pagination convention

MCP standardises opaque cursor-based pagination for `tools/list`, `resources/list`,
`resources/templates/list`, and `prompts/list`
(`specification/2025-06-18/server/utilities/pagination`):

> "Pagination in MCP uses an opaque cursor-based approach, instead of numbered pages.
>
> - The **cursor** is an opaque string token, representing a position in the result set
> - **Page size** is determined by the server, and clients **MUST NOT** assume a fixed
>   page size
>
> Clients **MUST** treat cursors as opaque tokens:
> - Don't make assumptions about cursor format
> - Don't attempt to parse or modify cursors
> - Don't persist cursors across sessions"
>
> — `specification/2025-06-18/server/utilities/pagination`

### 5.2 Implications for tool-level pagination

The spec's convention applies to the protocol's own list methods, not to your tools'
own list arguments. But adopting the same shape for your admin tools is the simplest
way to stay predictable for LLM callers:

- List tools return a page plus a `next_cursor` in `structuredContent`. The next call
  echoes that cursor unchanged.
- Don't expose offset/limit/page-number in tool inputs if you can help it. The model
  copes well with "call again with this cursor"; it copes badly with "this is page 3
  of 47, set offset to 150". `[heuristic, reinforced by the spec's "opaque" rule]`
- Expose a `limit` hint with a server-enforced maximum, so the model can say "give me
  50" without needing to manage pagination at all for small result sets.
- Return `total_count` only when the server can compute it cheaply. Otherwise return
  a boolean `has_more`. An LLM chasing an inaccurate total will loop.

### 5.3 Filtering / search

Two patterns, both observed across the MCP reference set:

- **Server-side filter tool** — `user_list(filter: { role: "admin", created_after: ... })`.
  Enumerate every filter field; reject extras.
- **Full-text search tool** — `user_search(query: string, limit: int)`. Use when the
  underlying store supports it. The query string is free-form; the tool's *description*
  should say what indices are searched ("email, display name, SSO subject").

Keep these as separate tools when both exist — the LLM chooses the right one based on
name and description, which beats one mega-tool that has a sparse `filter` object
*and* an optional `query` string. `[heuristic + spec naming guidance]`

### 5.4 When to switch to a resource template

If a "list" is really "browse the catalogue" and a human is doing the picking, a
Resource Template is the native fit: `agentlens://users/{role}` with parameter
completion. The spec's template example
(`specification/2025-06-18/server/resources`, "Resource Templates") shows exactly this
shape. But if the list is the input to further *model-driven* decisions, keep it as a
tool.

---

## 6. Audit and logging

### 6.1 What the spec requires of whom

The two obligations landed in different places:

- **Servers MUST** rate-limit tool invocations and implement access controls
  (`specification/2025-06-18/server/tools`, §"Security Considerations").
- **Clients SHOULD** "Log tool usage for audit purposes" (ibid).
- **MCP servers MUST** validate that tokens are issued for them specifically
  ("audience"), per RFC 8707 `resource` parameter
  (`specification/2025-06-18/basic/authorization`, "Token Handling"). Token passthrough
  is "explicitly forbidden"
  (`specification/2025-06-18/basic/security_best_practices`, "Token Passthrough").

So: client-side logs are not enough for an admin server. The server has to carry its
own audit trail, because the spec is explicit that "the MCP Server will be unable to
identify or distinguish between MCP Clients when clients are calling with an
upstream-issued access token which may be opaque to the MCP Server" — i.e., without
proper audience-bound tokens, the server simply doesn't know who it's serving.

### 6.2 What to log

Each audit record should combine server-side identity (from the bearer token the
server validated) with the tool invocation:

- Token subject (`sub` / user ID) and, if present, the acting agent / client ID (`aud`
  or `client_id`).
- Tool name, arguments (redacted for secrets), `request_id` from JSON-RPC.
- Outcome: success, `isError: true` (with category), or protocol error code.
- Duration and rate-limit bucket usage.
- For destructive tools: the preview / plan ID that preceded apply, the `confirm`
  token used, or the elicitation response.

### 6.3 Caller identity — what the server can and cannot know

The spec's authorization model uses OAuth 2.1 bearer tokens with the `resource`
parameter enforcing audience binding
(`specification/2025-06-18/basic/authorization`, "Access Token Usage"). Concretely:

- The server knows **which principal the token was issued for** (the user, via `sub`).
  This is the identity to bind every audit record to.
- The server does **not** natively know **which LLM invocation** triggered the call. A
  tool might be called by an autonomous loop, by a UI "Run tool" button, or by a
  scripted client — the bearer token is the same.
- If that distinction matters, the server has to require it as explicit arguments: a
  `purpose: string` field, a `reason: string` field, or a higher-level workflow ID.
  These are imperfect (the LLM synthesises them) but they're the only signal the
  server has. `[heuristic, motivated by spec audit gaps]`

A clean, deployable audit shape for an admin MCP server is therefore:

```
{
  "ts": "2026-04-17T10:12:33Z",
  "principal": "user:abc-123",          // from validated bearer token
  "client_app": "claude-code/0.4.2",    // from clientInfo on initialize
  "session": "mcp-session-...",         // bound to token; never trusted alone (spec)
  "tool": "user_delete",
  "args": { "email": "john@example.com", "confirm": "DELETE john@example.com" },
  "annotations_declared": { "destructiveHint": true, "idempotentHint": false },
  "outcome": "success",
  "request_id": "jsonrpc-42"
}
```

The `session` field is the transport-level session from Streamable HTTP; the spec
warns explicitly that **"MCP Servers MUST NOT use sessions for authentication"**
(`specification/2025-06-18/basic/security_best_practices`, "Session Hijacking"). Log
it, but never authorise on it alone.

---

## 7. Recommendations for AgentLens

These are direct conclusions from §§1–6 applied to AgentLens's shape (REST admin API,
CRUD + a few non-CRUD actions, multi-tenant, JWT auth, microkernel + plugin).

### 7.1 Primitive mapping

| Category | Primitive | Rationale |
| --- | --- | --- |
| List / get / search of catalog, users, roles, projects, keys (model-driven) | **Tools** (`*_list`, `*_get`, `*_search`) | Agent decides when to read; not browseable via resource picker. |
| Catalog entries as citable objects | **Resource template** `agentlens://catalog/{entry_id}` | Stable URI, fits "reference this entry in a chat". |
| CRUD mutations | **Tools**, one per (resource, verb) | Clean per-tool `annotations`; destructive ops isolated. |
| Non-CRUD actions (rebuild discovery, refresh health, bootstrap) | **Tools**, each its own | Distinct annotations (idempotent vs not), distinct permissions. |
| Operator workflows ("offboard user", "rotate project keys") | **Prompts** | User-selected, bundle the right tool sequence. |

Do **not** build a single generic `agentlens_api(method, path, body)` tool. It violates
every principle in §§2–4: names are opaque, annotations can't be per-endpoint, schemas
are useless, and the LLM has to reason about REST to use it. If an OpenAPI-to-MCP
gateway is on the table (Alternative 3 in the task brief), it still needs to emit one
named, schemaed, annotated MCP tool per (resource, verb), not one omnibus tool.

### 7.2 Granularity

- One tool per CRUD verb per resource: `catalog_create`, `catalog_update`,
  `catalog_delete`, `catalog_get`, `catalog_list`, `catalog_search`. Same for `user`,
  `role`, `project`, `api_key`.
- Bundle `get` + `search` only if the semantics are identical; otherwise keep them
  separate so `get` can require an ID while `search` accepts a free-form query.
- Expose a **read-only mode** (server flag / scope) that filters
  `tools/list` to tools with `readOnlyHint: true`. This is the single most important
  feature for agent deployments where you don't want autonomous mutation.

### 7.3 Safety posture

- Every mutating tool declares `annotations.destructiveHint: true` unless it's
  provably reversible, plus `idempotentHint` set truthfully. Remember: hints are
  untrusted by clients (§3.2) — the server also enforces.
- Destructive tools require an explicit `confirm` argument **or** issue an
  `elicitation/create` (server detects from `initialize` capabilities). Both paths
  exist so clients without elicitation still work.
- Destructive tools that touch >1 record expose a `dry_run: true` default plus a
  paired `*_apply(plan_id)` tool.
- All update tools require an `if_match` / version parameter (optimistic concurrency);
  reject 409 on mismatch.
- Rate-limit tool invocations server-side (spec MUST). Destructive tools get a lower
  bucket.
- Scope-gate destructive tools behind a separate OAuth scope (`agentlens:admin:write`)
  and advertise the visibility change via `notifications/tools/list_changed` on
  scope elevation.

### 7.4 Schema rules

- Adopt §4.2 (1–10) as a hard checklist. Enforce it in code review / in the
  plugin-author guide for any MCP tool published under AgentLens's namespace.
- `additionalProperties: false` is mandatory for all tool inputs.
- `outputSchema` is mandatory for any tool whose result the agent will act on
  programmatically (all list/search/get, all mutations that return the updated
  object). Pair with a `text` block for humans (`spec: "SHOULD also return the
  serialized JSON in a TextContent block"`).
- Errors go through `isError: true` + a `text` block for transient/operational errors;
  JSON-RPC error codes for protocol/schema violations. Match the spec's split exactly.

### 7.5 Pagination and search

- All `*_list` / `*_search` tools use opaque `cursor` + server-chosen page size, with
  a `limit` hint capped server-side. Mirror the MCP spec's own pagination for
  consistency.
- Never return offset/limit/page semantics through the tool surface; keep cursors
  opaque even inside MCP tool results so a future backing-store change doesn't break
  the contract.

### 7.6 Audit

- Every tool handler emits a structured slog record with the fields in §6.3, bound to
  the validated token's `sub`.
- Never trust the MCP session ID for authorization (spec MUST NOT); log it for
  correlation only.
- Propagate `request_id` from the inbound JSON-RPC call into the existing AgentLens
  HTTP audit trail so a chained "LLM → MCP → REST → DB" action stays correlatable.
- Log `annotations_declared` for every invocation — so if destructiveHint flips later,
  we have a historical record of what the client was told at call time.

### 7.7 Which alternative does this favour?

Applying the above:

- **(1) Standalone MCP server** — viable, matches the spec well, but duplicates
  schemas/permissions that already exist on the REST side.
- **(2) Auto-registered plugin on the default project** — same design as (1) but
  benefits from AgentLens's plugin lifecycle (`Register → Init → Start → Stop`) and
  license gating. The §3 "scope-gated visibility" pattern maps cleanly onto
  AgentLens's existing `RequirePermission` middleware.
- **(3) Generalised OpenAPI-to-MCP gateway** — attractive for breadth, but the
  schema-quality rules in §4 (descriptions that state side-effects, enums, safety
  annotations per operation, error semantics) are things *no* OpenAPI spec expresses
  well enough to auto-translate. A gateway would produce low-quality tools unless it
  also runs a curation step (augmenting the OpenAPI with MCP-specific metadata). That
  curation step is then ~80% of the work that option (2) already does.

Net: **option (2)** is the sharpest fit. Option (3) is useful as a *prototype path*
and to keep parity with future AgentLens endpoints, but it has to be gated through a
quality layer that is, in effect, the plugin from option (2).

---

## 8. Open questions for next phase

Things this research did not settle and that the design brief should address:

1. **Elicitation adoption.** Which AgentLens-target MCP clients actually implement
   `elicitation/create` today? If most don't, confirm flags are load-bearing.
2. **Scope/role mapping.** How does AgentLens's existing RBAC (`resource:action`
   permissions) map onto OAuth scopes exposed through `WWW-Authenticate` challenges?
3. **Toolset dynamism.** Do we ship one monolithic tool list or multiple "modes"
   (read, write, admin, plugins) toggled by config/scope? The spec allows both; the
   UX trade-off is real.
4. **Resource-template URIs.** Should catalog entries use `agentlens://` (our own
   scheme) or `https://` pointing at the existing REST URL? Spec permits custom
   schemes; custom hurts generic clients; HTTPS leaks internal URLs.
5. **Batch semantics.** For bulk ops (rebuild discovery, refresh health for N agents),
   all-or-nothing or best-effort?

---

## 9. Citations

All URLs are primary MCP specification / docs (spec revision `2025-06-18`) fetched
2026-04-17. Inline §§ refer back to these.

1. `modelcontextprotocol.io/specification/2025-06-18/server/tools` — Tools primitive:
   data types, `tools/list`, `tools/call`, error model, annotations warning,
   human-in-the-loop SHOULDs, security considerations, output schema semantics.
2. `modelcontextprotocol.io/specification/2025-06-18/server/resources` — Resources
   primitive: URIs, templates, subscriptions, application-driven interaction model,
   common URI schemes.
3. `modelcontextprotocol.io/specification/2025-06-18/server/prompts` — Prompts
   primitive: user-controlled model, slash-command UX, schema.
4. `modelcontextprotocol.io/specification/2025-06-18/client/elicitation` —
   `elicitation/create`, three-action response model (`accept`/`decline`/`cancel`),
   restricted schema, "MUST NOT use elicitation to request sensitive information".
5. `modelcontextprotocol.io/specification/2025-06-18/server/utilities/pagination` —
   Opaque cursors, server-chosen page size, client MUST NOT parse cursors.
6. `modelcontextprotocol.io/specification/2025-06-18/basic/authorization` — OAuth 2.1,
   RFC 8707 `resource` parameter, token audience binding, bearer token usage rules.
7. `modelcontextprotocol.io/specification/2025-06-18/basic/security_best_practices` —
   Confused deputy, token passthrough (forbidden), session hijacking (sessions MUST
   NOT authenticate), scope minimization (progressive least-privilege).
8. `modelcontextprotocol.io/docs/concepts/server-concepts` — Canonical summary of
   the three primitives, who controls each, travel-booking worked example showing
   tool-vs-resource split and prompt orchestration.
9. `modelcontextprotocol.io/docs/concepts/architecture` — Host/client/server
   participants, JSON-RPC data layer, STDIO vs Streamable HTTP transports, sampling
   and elicitation as client-side primitives.
10. `modelcontextprotocol.io/specification/2025-03-26/server/tools` — Prior
    revision; referenced to confirm that `annotations` is an established field whose
    contents are "untrusted unless they come from trusted servers".

### Sources attempted but not retrieved in this pass

(Blocked by fetch/domain policy in this session; cite separately if reconfirmation
needed.)

- `github.com/modelcontextprotocol/servers` tree — reference filesystem, "everything",
  memory, etc. servers. Claims about them are limited to what the MCP docs themselves
  reference (e.g., the `docs/concepts/architecture` page references the filesystem
  server by name).
- `github.com/github/github-mcp-server` — the official GitHub MCP server's toolsets
  and read-only mode. Referenced in §2 as a secondary example; verify wording before
  quoting.
- `anthropic.com/engineering/writing-tools-for-agents` / `docs.anthropic.com` — the
  Anthropic "Writing effective tools" guidance. Not cited above; §§2–4 lean on the MCP
  spec's own naming and schema examples plus `[heuristic]`-labelled claims where the
  community consensus is stable.
