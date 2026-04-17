# Frontend: Build & Tooling

Bun + Vite for package management, build, and dev server. Versions pinned.

### Bun 1.3.11 as Package Manager and Runner
Bun is the canonical package manager and script runner. Version is pinned via `.bun-version` and the Dockerfile frontend stage. Installs use `--frozen-lockfile`. Source: .bun-version, Dockerfile, Makefile.

### Vite 8 + React 18 + Tailwind 3 Stack
Frontend uses Vite 8 with `@vitejs/plugin-react`, React 18, Tailwind CSS 3 (via postcss + autoprefixer), shadcn/ui (Radix primitives), React Query, React Router v6. Dev server proxies `/api` and `/healthz` to `localhost:8080`. Source: web/package.json, web/vite.config.ts, web/postcss.config.js.

### Canonical npm Scripts
`dev` (vite), `build` (tsc && vite build — typecheck must pass before bundling), `preview`, `test` (vitest run), `test:watch`, `type-check` (tsc --noEmit). Run via `bun run`. Source: web/package.json.
