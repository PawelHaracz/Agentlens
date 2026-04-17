# DevOps: CI/CD Gates

GitHub Actions workflows enforce lint, test, coverage, security scanning, and release automation. Every PR must pass these gates.

### Lint + Test + Build Gate
The `ci.yml` workflow runs `make vet`, `make lint` (golangci-lint), and `make web-lint` (`tsc --noEmit` via bunx) in a `lint` job. The `build` job declares `needs: [lint, test, test-frontend]` so it only runs after lint and tests pass. Source: .github/workflows/ci.yml.

### Go + Frontend Coverage Gates
Backend: `make test-coverage` uploads `coverage.out`. Frontend: `make web-test-coverage` via Vitest uploads `web/coverage/`. Vitest thresholds are enforced: **lines 80, functions 80, branches 75, statements 80** on `src/**/*.{ts,tsx}` (excluding `main.tsx`, `test-setup.ts`, `.d.ts`, `.test.*`, `components/ui/**`, `types.ts`). Source: web/vitest.config.ts, ci.yml.

### Security Scanning Gate
`code-scanning.yml` runs CodeQL (go + javascript-typescript matrix), `govulncheck` on Go dependencies, Trivy against the built Docker image (CRITICAL + HIGH severity, SARIF uploaded to GitHub Security), and `helm lint deploy/helm/agentlens` + `helm template` rendering validation. Weekly schedule: Monday 06:00 UTC. Source: .github/workflows/code-scanning.yml.

### Playwright E2E Gate (Real SQLite Backend)
`e2e.yml` builds the Go binary with `CGO_ENABLED=1`, starts `agentlens` on port 18080 with an ephemeral `DATA_DIR`, waits for `/healthz`, extracts and masks the bootstrap admin password using `::add-mask::$ADMIN_PW`, then runs `bunx playwright test`. 20-minute workflow timeout. Source: .github/workflows/e2e.yml.

### Semantic Release Automation
`release.yml` auto-bumps versions (or accepts overrides), pushes Docker images to `ghcr.io/<owner>/agentlens:<ver>` and `:latest`, packages the Helm chart and pushes to `oci://ghcr.io/<owner>/charts`, creates GitHub Releases with tags `v<app>` and `helm/v<chart>`. Concurrency lock `release-${{ github.ref }}` prevents parallel releases. Source: .github/workflows/release.yml.

### Documentation Auto-Deploy (MkDocs Material)
`docs.yml` deploys on push to `main` when `docs/**`, `mkdocs.yml`, or `requirements-docs.txt` changes. Uses `actions/setup-python@v5` with pip cache keyed to `requirements-docs.txt`. Source: .github/workflows/docs.yml.
