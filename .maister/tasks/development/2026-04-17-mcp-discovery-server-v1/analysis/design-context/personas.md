# Personas — MCP Discovery Server for AgentLens

Approved 2026-04-17.

---

## Persona 1 — Anya, Backend LLM App Developer (PRIMARY)

**Role**: Senior engineer building a production LLM service (e.g., document-processing agent orchestrator). Works at a company that deploys AgentLens as internal infrastructure.

### Goals
- LLM service discovers agents at runtime (e.g., "find a PDF-extraction agent for scanned docs") without REST-client glue.
- Tool-call p95 < 100ms (AgentLens calls are on the critical path of user-facing responses).
- Adjust reachable agent pool by changing service-account project membership, not code deploys.

### Pain points (status quo)
- Writes + maintains AgentLens REST-client wrappers in her service's language.
- Translates REST responses → LLM tool-call return shapes manually.
- No first-class Claude/MCP tool integration.
- JWT issued for human users is awkward for a headless backend service.

### Key journey
1. Operator (Priya) creates service account, grants membership in Project A + C, hands Anya an API key.
2. Anya sets `AGENTLENS_MCP_URL` + `AGENTLENS_API_KEY` env vars.
3. LLM framework (LangChain / Vercel AI SDK / Anthropic SDK) connects AgentLens as an MCP server with bearer header.
4. LLM calls `agent_search("pdf extraction")` → list of matching agents across Projects A + C + default.
5. LLM calls `agent_get(id)` for a chosen agent, invokes it directly via its endpoint (outside MCP scope).
6. Anya observes OTel metrics/traces in existing Grafana — no new dashboards.

---

## Persona 2 — Karol, Human AI Developer in the IDE (SECONDARY)

**Role**: Developer prototyping LLM apps in Claude Code / Cursor / VS Code+Copilot / Claude.ai. Same company as Anya or separate.

### Goals
- Explore team's AgentLens catalog while coding or chatting with Claude.
- Zero-config integration: paste URL → works.
- Discover-then-code: "what agents can do X?" → copy ID → use in code.

### Pain points (status quo)
- No integration with coding flow — opens AgentLens web UI, searches manually, copy-pastes IDs.
- Can't easily share access with teammates — UI is per-user, MCP connectors are shareable.

### Key journey
1. Reads "AgentLens MCP quickstart" in docs.
2. Claude.ai → Settings → Connectors → Add → pastes `https://agentlens.example.com/api/mcp`.
3. Browser redirect → AgentLens OAuth login → grants `mcp:discovery` scope → redirected back.
4. Asks Claude "what agents can extract tables from PDFs?" → Claude calls `agent_search`, shows results.
5. Claude fetches raw card; Karol reviews schema; uses in code.

---

## Persona 3 — Priya, AgentLens Platform Operator (ENABLING)

**Role**: Internal platform/devops engineer responsible for AgentLens deployment. Not a primary MCP user but enables Anya and Karol.

### Goals
- Create service accounts for backend apps; revoke when teams move on.
- Project-based access boundaries so apps only see their own projects' catalogs.
- Monitor MCP usage — which apps hit which tools, rate, failures.
- No new auth system to maintain.

### Pain points (status quo) & needs
- No service-account concept today. Either hands out human users (bad) or scripts with admin tokens (worse).
- No MCP usage/audit view for "who called X last week" incident reviews.
- Needs instant API-key revocation when a service is retired.

### Key journey
1. Anya requests: "need AgentLens service account with read access to Project A + C."
2. Priya: AgentLens UI → Service Accounts → Create → names `doc-pipeline-prod`.
3. UI shows API key once; Priya puts it in team secrets manager, hands name to Anya.
4. UI toggles project memberships; she grants A + C.
5. Month later: service decommissioned. Priya clicks "Revoke" — key stops instantly.
6. Audit query: Priya opens Grafana panel showing MCP invocations keyed by `principal_id` + `tool_name`.

---

## Cross-Persona Observations

- **Anya drives scope**: 95% of tool-call volume. Design decisions default to Anya's ergonomics.
- **Karol is ergonomic validation**: if Claude.ai Custom Connector "just works" for Karol, OAuth layer is right.
- **Priya drives operational surface**: service-account model, audit log, project membership UI. Separate from MCP tool surface itself, but gated on the same data model.
- **Default project as public tier**: all three personas benefit from the rule that `default` project is readable by everyone — minimum-friction onboarding for shared agents.
