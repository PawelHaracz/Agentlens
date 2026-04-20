# MCP Discovery Server — Quickstart (5 minutes)

This guide gets a backend LLM app querying AgentLens agents via MCP in 5 minutes using a service-account API key.

---

## Prerequisites

- AgentLens running (see [README](../README.md))
- Admin credentials

---

## Step 1 — Enable MCP in config

Add to `agentlens.yaml` (or set env vars):

```yaml
mcp_server:
  enabled: true
  public_url: "https://agentlens.example.com/api/mcp"
  audit_enabled: true
```

Or via environment:

```bash
AGENTLENS_MCP_ENABLED=true
AGENTLENS_MCP_PUBLIC_URL=https://agentlens.example.com/api/mcp
```

Restart AgentLens. Verify:

```bash
curl https://agentlens.example.com/api/mcp/status
# {"enabled":true,"active_sessions":0,...}
```

---

## Step 2 — Create a service account

```bash
curl -X POST https://agentlens.example.com/api/v1/service-accounts \
  -H "Authorization: Bearer <admin-jwt>" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-llm-app"}'
```

Response includes a **one-time secret** — copy it now:

```json
{
  "party": {"id": "...", "name": "my-llm-app"},
  "client_id": "abc123",
  "secret": "agentlens_sk_abc123def456.supersecretvalue..."
}
```

Store the secret securely (e.g., as `AGENTLENS_API_KEY` in your app's environment).

---

## Step 3 — Call the MCP endpoint

### MCP initialize handshake

```bash
curl -X POST https://agentlens.example.com/api/mcp \
  -H "Authorization: Bearer agentlens_sk_abc123def456.supersecretvalue..." \
  -H "Content-Type: application/json" \
  -H "Origin: https://your-app.example.com" \
  -d '{"jsonrpc":"2.0","id":"1","method":"initialize","params":{}}'
```

Note the `MCP-Session-Id` response header — include it in subsequent calls.

### Search for agents

```bash
curl -X POST https://agentlens.example.com/api/mcp \
  -H "Authorization: Bearer agentlens_sk_abc123def456.supersecretvalue..." \
  -H "MCP-Session-Id: <session-id>" \
  -H "Origin: https://your-app.example.com" \
  -d '{"jsonrpc":"2.0","id":"2","method":"tools/call","params":{"name":"agent_search","arguments":{"query":"pdf extraction"}}}'
```

---

## Step 4 — Configure allowed origins (production)

Add your app's origin to the allowlist:

```yaml
mcp_server:
  allowed_origins:
    - "https://your-app.example.com"
    - "https://claude.ai"
```

Empty allowlist → all requests 403 (strict default).

---

## Available Tools

| Tool | Description |
|---|---|
| `agent_search` | Free-text search across agents + capabilities |
| `agent_get` | Get full agent details by catalog ID |
| `capabilities_list` | List capabilities offered by an agent |
| `agent_card` | Fetch raw protocol card (A2A/MCP) |

---

## Claude.ai / Cursor — OAuth Setup

For interactive IDE use, enable Dex federation and add AgentLens as a Custom Connector:

1. Paste MCP URL: `https://agentlens.example.com/api/mcp`
2. The IDE fetches `/.well-known/oauth-protected-resource` → discovers Dex.
3. Complete OAuth flow in browser.
4. Tools appear in the IDE.

See [auth.md](auth.md#federation-dex-oauth-21) for full federation setup.
