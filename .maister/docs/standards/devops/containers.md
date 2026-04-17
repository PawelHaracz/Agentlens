# DevOps: Containers & Kubernetes

Production image and Helm chart both enforce strict security defaults.

### Multi-Stage Distroless Nonroot Image
Production Docker image is a 3-stage build: (1) `oven/bun:1.3.11-alpine` compiles the frontend (`bun install --frozen-lockfile` → `bun run build`); (2) `golang:1.26.1` cross-compiles with `CGO_ENABLED=1` (gcc, libc6-dev installed); (3) runtime is `gcr.io/distroless/base-debian12:nonroot` running as UID/GID 65532 and exposing port 8080. Version is injected via `-ldflags "-X main.version=$(VERSION)"`; default `VERSION=dev`. Source: Dockerfile.

### Hardened Pod Security Defaults (Helm)
`deploy/helm/agentlens/values.yaml` ships: `runAsNonRoot=true`, `runAsUser/Group/fsGroup=65532`, `seccompProfile: RuntimeDefault`, `readOnlyRootFilesystem=true`, `allowPrivilegeEscalation=false`, `capabilities.drop: [ALL]`. ServiceAccount `automountServiceAccountToken=false`. Default resource requests 100m CPU / 128Mi, limits 500m / 512Mi. Liveness on `/healthz`, readiness on `/readyz`. Source: deploy/helm/agentlens/values.yaml.

### Strict Helm Lint + Matrix Template Test
Chart: `apiVersion v2`, `version 0.2.0`, `appVersion 0.2.0`. CI runs `helm lint --strict` with both default values and `ci/ci-values.yaml`, then `helm template agentlens <chart> --debug > /dev/null`, then `./scripts/test-helm-templates.sh`. PostgreSQL subchart is `~16.x` from bitnami, gated by `postgresql.enabled`. Source: Makefile, deploy/helm/agentlens/Chart.yaml.

### CGO_ENABLED=1 Required Everywhere
Builds and tests must set `CGO_ENABLED=1` because the SQLite driver (mattn/go-sqlite3) is a cgo package. The Makefile sets it for `build`, `test`, `test-coverage`, `test-race`. Dockerfile installs gcc/libc6-dev in the builder stage. Source: Makefile, Dockerfile.
