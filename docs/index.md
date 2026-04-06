# AgentLens

**Real-time AI agent catalog for Kubernetes — discover, track, and inspect A2A, MCP, and A2UI agents across your cluster.**

AgentLens automatically discovers AI agents running in Kubernetes (via Service annotations), polls static endpoints, and accepts push registrations. It exposes a REST API and a web dashboard for browsing the catalog, filtering by protocol/status, and inspecting agent cards and capabilities.

---

## Quickstart

### Docker Compose

```bash
git clone https://github.com/PawelHaracz/agentlens
cd agentlens/examples
docker compose up
```

Open [http://localhost:8080](http://localhost:8080) in your browser.

### Helm (Kubernetes)

```bash
helm install agentlens ./deploy/helm/agentlens \
  --namespace agentlens --create-namespace
```

AgentLens will start watching Services across all namespaces for agent annotations.

---

## Main Sections

- [Architecture](architecture.md) — System design, components, and data flow
- [API Reference](api.md) — REST API endpoints and schemas
- [User Guide](user-guide.md) — Using the web dashboard
- [End User Guide](end-user-guide.md) — End-to-end usage walkthrough
- [Developer Guide](developer-guide.md) — Contributing and extending AgentLens
- [DevOps Guide](devops-guide.md) — Deployment, Helm, and Kubernetes
- [Authentication](auth.md) — Auth configuration and JWT
- [Database](database.md) — Database setup and migrations
- [Settings](settings.md) — Configuration reference
