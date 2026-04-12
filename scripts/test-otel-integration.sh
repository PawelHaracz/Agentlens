#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== OTel Integration Test ==="
echo "Starting AgentLens + Jaeger..."
docker compose -f "$ROOT_DIR/docker-compose.otel.yml" up -d --build

cleanup() {
  echo "Cleaning up..."
  docker compose -f "$ROOT_DIR/docker-compose.otel.yml" down
}
trap cleanup EXIT

# Wait for AgentLens to be healthy
echo "Waiting for AgentLens..."
for i in $(seq 1 60); do
  if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
    echo "AgentLens is up"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: AgentLens did not start in time"
    docker compose -f "$ROOT_DIR/docker-compose.otel.yml" logs agentlens
    exit 1
  fi
  sleep 3
done

# Generate traffic to produce traces
echo "Generating traces..."
curl -sf http://localhost:8080/healthz > /dev/null
curl -sf http://localhost:8080/readyz > /dev/null
curl -sf http://localhost:8080/api/v1/catalog > /dev/null || true
curl -sf http://localhost:8080/metrics | head -3

# Wait for traces to be exported (flush interval + processing)
echo "Waiting for trace export (10s)..."
sleep 10

# Query Jaeger for traces
echo "Checking Jaeger for traces..."
TRACES_RESPONSE=$(curl -sf "http://localhost:16686/api/traces?service=agentlens&limit=5" 2>/dev/null)

if [ -z "$TRACES_RESPONSE" ]; then
  echo "FAIL: No response from Jaeger API"
  exit 1
fi

COUNT=$(echo "$TRACES_RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(len(data.get('data', [])))
" 2>/dev/null || echo "0")

echo "Found $COUNT traces in Jaeger"

if [ "$COUNT" -gt 0 ]; then
  echo "PASS: OTel integration test — traces visible in Jaeger"
  exit 0
else
  echo "FAIL: No traces found in Jaeger"
  echo "Jaeger response: $TRACES_RESPONSE"
  exit 1
fi
