# ADR-007: Embedded SPA Frontend

Date: 2026-04-11
Status: Accepted
Related: [ADR-005](005-authentication-and-session-management.md) (cookie auth for SPA)

## Context

AgentLens needs a web UI for interactive catalog browsing, search, filtering, and agent management. The deployment model targets on-prem and edge environments where minimizing operational complexity matters — fewer moving parts, fewer things to configure and monitor. The UI must support client-side routing, real-time filtering, and rich interactive components (search, capability views, agent detail pages).

Key tensions:

1. **Distribution simplicity** — operators want a single artifact to deploy, not a Go binary plus a separately hosted frontend.
2. **Developer experience** — frontend developers need hot module replacement and fast iteration without rebuilding the Go binary on every change.
3. **Build pipeline** — the Go toolchain's `//go:embed` directive fails if the embedded directory is missing, but frontend builds are slow and shouldn't block `go vet` or `golangci-lint`.

## Decision

Ship the frontend as a **React 18 SPA embedded into the Go binary** via `//go:embed`.

### Embedding

Vite builds the SPA into `web/dist/`. The `web/embed.go` file uses `//go:embed dist` to include the entire directory. `web.FS()` returns `fs.Sub(distFS, "dist")` to strip the prefix, giving the API layer a clean `fs.FS` rooted at the SPA output.

### SPA fallback routing

`spaHandler()` in `api/router.go` wraps `http.FileServer` with fallback logic:

- Path starts with `/api/` → return 404 (never swallow API routes).
- `fs.Stat()` finds a real file in embedded dist → serve it (JS, CSS, assets).
- Otherwise → rewrite to `/` → serve `index.html` → React Router handles client-side routing.

### Stack

React 18, react-router-dom v6, TypeScript, Vite 8, TailwindCSS 3, Radix UI, shadcn-style components (cva + tailwind-merge + clsx), TanStack React Query for server state. Bun as JS package manager.

### Dev mode

Vite dev server proxies `/api` and `/healthz` to `localhost:8080` (Go backend). Frontend development does not require the Go binary — `bun run dev` is sufficient.

### Build placeholder

`make lint` creates a stub `web/dist/index.html` via the `add-html-placeholder` target so `//go:embed` doesn't fail when the frontend hasn't been built. Real frontend output requires `make web-build`.

### No SSR

Pure client-side rendering. The Go server serves static files only — no Node.js runtime in production.

## Consequences

### Positive

- Single binary distribution — `agentlens` binary contains the full UI. No CDN, no separate static file server, no CORS configuration.
- No Node.js runtime dependency in production — the Go binary is the only process.
- Dev mode decouples frontend and backend iteration — Vite proxy gives HMR without rebuilding Go.
- SPA fallback handler gives clean deep-linking support — bookmarkable URLs like `/agents/abc-123` work without server-side route registration per page.

### Negative / Trade-offs

- Two-step build — frontend changes require `make web-build` before `make build`. Forgetting the first step produces a binary with stale (or stub) UI.
- Embedded `dist/` increases binary size by ~2-5MB for a typical React app with assets.
- No hot module replacement in production — frontend is static once compiled into the binary.
- SPA fallback means server-side routes cannot overlap with frontend route paths — `/api/` prefix convention enforces this, but new server routes must respect it.
- `//go:embed` fails if `web/dist/` is missing entirely — the placeholder target in `make lint` is a workaround, not a clean solution.

### Neutral

- No server-side rendering means initial page load shows a blank screen until JS loads. Acceptable for an internal operations tool, not suitable for public-facing SEO-dependent pages.
- Frontend framework choice (React) is independent of the embedding decision — switching to Svelte or Vue would require changing `web/` contents but not the Go embedding or routing logic.

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| **Separate frontend deployment (CDN + API)** | Adds deployment pipeline, CORS configuration, and infrastructure. Defeats single-binary distribution goal for on-prem/edge. |
| **Server-side rendering (Next.js, Remix)** | Requires Node.js runtime in production. Defeats single-binary goal. Adds process management complexity. |
| **Go `html/template` rendering** | Poor UX for interactive catalog browsing — no client-side routing, page reloads on every interaction, limited component ecosystem. |
| **HTMX / Alpine.js** | Smaller ecosystem for complex UI patterns (real-time filtering, search-as-you-type, capability tree views). Would require more custom JS anyway. |
| **Separate API gateway + static file server** | More infrastructure, more ops burden. Unnecessary when Go can serve both API and static files from a single process. |
