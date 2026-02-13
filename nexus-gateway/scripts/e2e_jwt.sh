#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [[ ! -f ".env" ]]; then
  echo "missing .env (create it from .env.example)" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

HTTP_BASE="http://localhost:${NEXUS_HTTP_PORT}"
MOCK_BASE="http://localhost:${NEXUS_MOCK_TOOLS_PORT}"

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing dependency: $1" >&2; exit 1; }
}

require curl
require jq
require rg
require sha256sum

fail() { echo "E2E JWT FAIL: $*" >&2; exit 1; }

assert_jq() {
  local json="$1"
  local filter="$2"
  echo "$json" | jq -e "$filter" >/dev/null || fail "jq assertion failed: $filter | json=$json"
}

http_code() {
  curl -sS -o /dev/null -w "%{http_code}" "$1" 2>/dev/null || true
}

cleanup() {
  docker compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "[e2e-jwt] bring stack up (JWT only)"
export NEXUS_AUTH_ENABLE_JWT=true
export NEXUS_AUTH_ALLOW_API_KEY=false
docker compose up --build -d >/dev/null

for _ in {1..60}; do
  [[ "$(http_code "${HTTP_BASE}/readyz")" == "200" ]] && break
  sleep 1
done
[[ "$(http_code "${HTTP_BASE}/readyz")" == "200" ]] || fail "readyz not 200"
[[ "$(http_code "${MOCK_BASE}/healthz")" == "200" ]] || fail "mock-tools healthz not 200"

echo "[e2e-jwt] migrate + seed"
make migrate-up >/dev/null
SEED_OUT="$(bash scripts/seed_demo.sh)"
API_KEY="$(echo "$SEED_OUT" | rg -n "^NEXUS_DEMO_API_KEY=" | tail -n1 | cut -d= -f2)"
[[ -n "$API_KEY" ]] || fail "seed key not found"
API_HASH="$(printf "%s" "$API_KEY" | sha256sum | awk '{print $1}')"

ORG_ID="$(docker compose exec -T postgres psql -U postgres -d nexus -At -c "select org_id from org_api_keys where api_key_hash='${API_HASH}' limit 1;")"
[[ "$ORG_ID" =~ ^[0-9a-fA-F-]{36}$ ]] || fail "org id not found"

TOKEN_RESP="$(curl -fsS "${MOCK_BASE}/_jwt/issue?org_id=${ORG_ID}&sub=e2e-jwt&role=secops")"
TOKEN="$(echo "$TOKEN_RESP" | jq -r '.token')"
[[ -n "$TOKEN" && "$TOKEN" != "null" ]] || fail "token not returned"

echo "[e2e-jwt] api-key disabled check"
NO_KEY="$(curl -sS "${HTTP_BASE}/v1/tools" || true)"
assert_jq "$NO_KEY" '.error.code=="UNAUTHORIZED"'

echo "[e2e-jwt] bearer run echo"
RUN_ECHO="$(curl -fsS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d '{"tool_name":"echo","input":{"jwt":"ok"},"context":{"user_id":"u_1"}}' \
  "${HTTP_BASE}/v1/run")"
assert_jq "$RUN_ECHO" '.status=="success" and .decision=="allow" and .result.received.jwt=="ok"'

echo "[e2e-jwt] bearer list tools"
TOOLS="$(curl -fsS -H "Authorization: Bearer ${TOKEN}" "${HTTP_BASE}/v1/tools")"
assert_jq "$TOOLS" '.items | type=="array" and length>=2'

echo "[e2e-jwt] bearer MCP call"
MCP_CALL="$(curl -fsS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool_name":"echo","input":{"from":"jwt-mcp"},"context":{"user_id":"u_1"}}}' \
  "${HTTP_BASE}/mcp")"
assert_jq "$MCP_CALL" '.result.status=="success" and .result.result.received.from=="jwt-mcp"'

echo "[e2e-jwt] OK"

