#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────────
#  run-e2e.sh — build the AgentLens server, start it with a fresh
#  SQLite database, capture the auto-generated admin password, and
#  run Playwright E2E tests against it.
# ──────────────────────────────────────────────────────────────────
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
E2E_DIR="$ROOT/e2e"
PORT="${AGENTLENS_PORT:-18080}"
DATA_DIR=$(mktemp -d)
BINARY="$ROOT/bin/agentlens"
PID_FILE="$DATA_DIR/server.pid"

cleanup() {
  if [ -f "$PID_FILE" ]; then
    kill "$(cat "$PID_FILE")" 2>/dev/null || true
  fi
  rm -rf "$DATA_DIR"
}
trap cleanup EXIT

echo "▶ Building Go binary …"
cd "$ROOT"
CGO_ENABLED=1 go build -o "$BINARY" ./cmd/agentlens/

echo "▶ Building frontend …"
cd "$ROOT/web"
bun install --silent
bun run build

echo "▶ Starting AgentLens on :${PORT} (data: $DATA_DIR) …"
export AGENTLENS_PORT="$PORT"
export AGENTLENS_DATA_DIR="$DATA_DIR"
export AGENTLENS_DB_DIALECT=sqlite
export AGENTLENS_DB_SQLITE_PATH="$DATA_DIR/agentlens.db"
export AGENTLENS_JWT_SECRET=e2e-test-secret-key-for-jwt-signing
export AGENTLENS_LOG_LEVEL=warn
export AGENTLENS_HEALTH_CHECK_ENABLED=false

# Start the server and capture output to extract the admin password.
SERVER_OUT="$DATA_DIR/server.out"
"$BINARY" -port "$PORT" >"$SERVER_OUT" 2>&1 &
echo $! >"$PID_FILE"

echo "▶ Waiting for server to become ready …"
for i in $(seq 1 30); do
  if curl -sf "http://localhost:${PORT}/healthz" >/dev/null 2>&1; then
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "✘ Server did not start in time. Output:"
    cat "$SERVER_OUT"
    exit 1
  fi
  sleep 1
done
echo "  Server ready."

# Extract the admin password from the server's stdout.
ADMIN_PW=$(awk -F 'Password: ' '/Password: / { print $2; exit }' "$SERVER_OUT" || true)
if [ -z "$ADMIN_PW" ]; then
  echo "✘ Could not extract admin password from server output."
  cat "$SERVER_OUT"
  exit 1
fi
export AGENTLENS_ADMIN_PASSWORD="$ADMIN_PW"
echo "  Admin password captured (not shown for security)."

echo "▶ Installing Playwright dependencies …"
cd "$E2E_DIR"
bun install --silent
bunx playwright install chromium --with-deps 2>/dev/null || bunx playwright install chromium

echo "▶ Running Playwright tests …"
bunx playwright test "$@"
echo "✔ All E2E tests passed."
