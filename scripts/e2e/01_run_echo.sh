#!/usr/bin/env bash
# Primer caso e2e: POST /v1/run con tool echo
# Valida el flujo mínimo del gateway (consumer -> gateway -> upstream -> respuesta)
#
# Prerrequisitos: make up, make migrate-up, make seed
# Uso: ./scripts/e2e/01_run_echo.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

HTTP_PORT="${NEXUS_HTTP_PORT:-8080}"
API_KEY="${NEXUS_DEMO_API_KEY:-nexus-core-local-key}"

HTTP_BASE="http://localhost:${HTTP_PORT}"

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing dependency: $1" >&2; exit 1; }
}
require curl
require jq

fail() { echo "E2E FAIL: $*" >&2; exit 1; }
assert_jq() {
  local json="$1"
  local filter="$2"
  echo "$json" | jq -e "$filter" >/dev/null || fail "jq assertion failed: $filter | json=$json"
}

# 1. Setup egress (echo debe poder llamar a mock-tools)
echo "[e2e/01] setup egress para echo"
EGRESS_CODE="$(curl -sS -o /dev/null -w "%{http_code}" \
  -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"host":"mock-tools","enabled":true}' \
  "${HTTP_BASE}/v1/tools/echo/egress-rules")"
[[ "$EGRESS_CODE" =~ ^(200|204)$ ]] || fail "egress setup failed: $EGRESS_CODE"

# 2. POST /v1/run con echo
echo "[e2e/01] POST /v1/run tool=echo"
RUN_RESP="$(curl -fsS \
  -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"echo","input":{"hello":"e2e"},"context":{"user_id":"u_1"}}' \
  "${HTTP_BASE}/v1/run")"

# 3. Assertions
assert_jq "$RUN_RESP" '.request_id | type=="string" and length>0'
assert_jq "$RUN_RESP" '.decision == "allow"'
assert_jq "$RUN_RESP" '.tool_name == "echo"'
assert_jq "$RUN_RESP" '.status == "success"'
assert_jq "$RUN_RESP" '.result.received.hello == "e2e"'
assert_jq "$RUN_RESP" '.latency_ms | type=="number" and . >= 0'

echo "[e2e/01] OK — run echo success"
