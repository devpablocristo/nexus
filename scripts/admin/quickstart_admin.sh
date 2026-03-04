#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BASE="http://localhost:${NEXUS_HTTP_PORT:-8080}"

if [[ ! -f .env ]]; then
  cp .env.example .env
fi

if ! grep -q '^NEXUS_DISABLE_SSRF_PROTECTION=' .env; then
  echo 'NEXUS_DISABLE_SSRF_PROTECTION=true' >> .env
else
  sed -i 's/^NEXUS_DISABLE_SSRF_PROTECTION=.*/NEXUS_DISABLE_SSRF_PROTECTION=true/' .env
fi

echo "[1/5] Starting stack"
docker compose up --build -d

echo "[2/5] Running migrations"
make migrate-up

echo "[3/5] Seeding demo tenant/key/scopes"
SEED_OUTPUT=$(make seed)
echo "$SEED_OUTPUT"
API_KEY=$(echo "$SEED_OUTPUT" | sed -n 's/^NEXUS_DEMO_API_KEY=//p' | tail -n1)
if [[ -z "${API_KEY}" ]]; then
  echo "Failed to extract NEXUS_DEMO_API_KEY from seed output" >&2
  exit 1
fi

echo "[4/5] Configure egress for demo tools"
for tool in echo transfer; do
  curl -sS -o /dev/null -w "${tool} -> HTTP %{http_code}\n" \
    -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"host":"mock-tools","enabled":true}' \
    "${BASE}/v1/tools/${tool}/egress-rules"
done

echo "[5/5] Validate REST + MCP + A2A + Admin bootstrap"
echo "RBAC check: /v1/tools without scope must be 403"
RBAC_CODE=$(curl -sS -o /dev/null -w "%{http_code}" \
  -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  "${BASE}/v1/tools")
if [[ "$RBAC_CODE" != "403" ]]; then
  echo "Expected 403 without scope, got ${RBAC_CODE}" >&2
  exit 1
fi

curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "X-NEXUS-ROLE: admin" \
  -H "X-NEXUS-SCOPES: admin:console:read,admin:console:write,admin:secrets" \
  "${BASE}/v1/admin/bootstrap" | jq

curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "X-NEXUS-SCOPES: gateway:run" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"echo","input":{"hello":"world"},"context":{"user_id":"u_1"}}' \
  "${BASE}/v1/run" | jq

curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "X-NEXUS-SCOPES: mcp:read" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' \
  "${BASE}/mcp" | jq

curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "X-NEXUS-SCOPES: a2a:call" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"echo","input":{"hello":"a2a"},"context":{"user_id":"u_1"}}' \
  "${BASE}/a2a/call" | jq

echo

echo "Quickstart complete"
echo "API key: ${API_KEY}"
echo "Admin console: ${BASE}/admin"
echo "Metrics: ${BASE}/metrics"
