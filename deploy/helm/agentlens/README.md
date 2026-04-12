# AgentLens Helm Chart

Production-grade Helm chart for [AgentLens](https://github.com/PawelHaracz/agentlens) — AI agent service discovery.

## Installation

### SQLite (development/single-instance)

```bash
helm install agentlens ./deploy/helm/agentlens \
  --namespace agentlens --create-namespace
```

### PostgreSQL (production/multi-replica)

```bash
helm install agentlens ./deploy/helm/agentlens \
  --namespace agentlens --create-namespace \
  --set database.dialect=postgres \
  --set postgresql.enabled=true \
  --set postgresql.auth.password="pg-password"
```

### With OpenTelemetry

```bash
helm install agentlens ./deploy/helm/agentlens \
  --set telemetry.enabled=true \
  --set telemetry.endpoint=otel-collector.monitoring:4317 \
  --set metrics.serviceMonitor.enabled=true
```

## Key Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas (SQLite: must be 1) | `1` |
| `database.dialect` | `sqlite` or `postgres` | `sqlite` |
| `postgresql.enabled` | Enable Bitnami PostgreSQL subchart | `false` |
| `telemetry.enabled` | Enable OTel instrumentation | `false` |
| `telemetry.endpoint` | OTLP collector endpoint | `""` |
| `telemetry.frontendEndpoint` | Optional browser OTLP/HTTP endpoint override | `""` |
| `metrics.serviceMonitor.enabled` | Enable Prometheus ServiceMonitor | `false` |
| `metrics.prometheus.enabled` | Enable /metrics endpoint | `false` |
| `serviceAccount.automountServiceAccountToken` | Mount service account token into pod | `false` |
| `ingress.enabled` | Enable Kubernetes Ingress | `false` |
| `gateway.enabled` | Enable Gateway API HTTPRoute | `false` |
| `autoscaling.enabled` | Enable HPA (requires postgres) | `false` |
| `pdb.enabled` | Enable PodDisruptionBudget | `true` |

## Testing

```bash
# After install:
helm test agentlens -n agentlens
```

## Probes

- **Liveness** (`/healthz`): process alive check, no DB dependency
- **Readiness** (`/readyz`): DB reachability check
- **Startup** (`/healthz`): 150s window for slow DB migrations

## Security

- Runs as non-root (UID 65532)
- Read-only root filesystem (`/tmp` emptyDir for Go temp files)
- All capabilities dropped
- seccomp RuntimeDefault profile
