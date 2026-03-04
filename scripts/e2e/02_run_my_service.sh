#!/usr/bin/env bash
# E2E: llama a la tool "my-service" registrada desde la UI (nexus-tower).
# Valida el flujo completo: consumer → gateway → mock-tools:8081/echo → respuesta.
#
# Prerrequisitos: stack levantado (docker compose up), tool "my-service" registrada.
# Uso: ./scripts/e2e/02_run_my_service.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

HTTP_PORT="${NEXUS_HTTP_PORT:-8080}"
API_KEY="${NEXUS_DEMO_API_KEY:-nexus-core-local-key}"
TOOL_NAME="${1:-my-service}"

HTTP_BASE="http://localhost:${HTTP_PORT}"

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing dependency: $1" >&2; exit 1; }
}
require curl
require jq

fail() { echo "FAIL: $*" >&2; exit 1; }
ok()   { echo "  ✓ $*"; }

assert_eq() {
  local actual="$1" expected="$2" label="$3"
  [[ "$actual" == "$expected" ]] || fail "$label: expected '$expected', got '$actual'"
  ok "$label"
}

assert_jq() {
  local json="$1" filter="$2" label="${3:-$2}"
  echo "$json" | jq -e "$filter" >/dev/null 2>&1 || fail "assertion: $label"
  ok "$label"
}

echo "═══════════════════════════════════════════════════════════"
echo " E2E: call tool '${TOOL_NAME}' via /v1/run"
echo "═══════════════════════════════════════════════════════════"
echo ""

# ── 1. Verify tool exists ────────────────────────────────────────────────────
echo "▸ Step 1: verify tool '${TOOL_NAME}' is registered"
TOOL_RESP="$(curl -fsS \
  -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "X-NEXUS-SCOPES: tools:read" \
  "${HTTP_BASE}/v1/tools/${TOOL_NAME}" 2>&1)" || fail "tool '${TOOL_NAME}' not found (GET /v1/tools/${TOOL_NAME} failed)"
assert_jq "$TOOL_RESP" ".name == \"${TOOL_NAME}\"" "tool name matches"
assert_jq "$TOOL_RESP" '.enabled == true'           "tool is enabled"

TOOL_URL="$(echo "$TOOL_RESP" | jq -r '.url')"
TOOL_METHOD="$(echo "$TOOL_RESP" | jq -r '.method')"
echo "  → ${TOOL_METHOD} ${TOOL_URL}"
echo ""

# ── 2. Call the tool via /v1/run ─────────────────────────────────────────────
echo "▸ Step 2: POST /v1/run (tool=${TOOL_NAME})"
RUN_RESP="$(curl -sS -w "\n%{http_code}" \
  -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "X-NEXUS-SCOPES: gateway:run" \
  -H "X-NEXUS-ACTOR: e2e/script" \
  -H "Content-Type: application/json" \
  -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"hello from e2e script\"},\"context\":{\"source\":\"e2e\"}}" \
  "${HTTP_BASE}/v1/run")"

HTTP_CODE="$(echo "$RUN_RESP" | tail -1)"
BODY="$(echo "$RUN_RESP" | sed '$d')"

assert_eq "$HTTP_CODE" "200" "HTTP status is 200"
echo ""

# ── 3. Validate response ────────────────────────────────────────────────────
echo "▸ Step 3: validate response"
assert_jq "$BODY" '.request_id | type=="string" and length>0' "request_id present"
assert_jq "$BODY" ".decision == \"allow\""                     "decision = allow"
assert_jq "$BODY" ".tool_name == \"${TOOL_NAME}\""            "tool_name matches"
assert_jq "$BODY" '.status == "success"'                       "status = success"
assert_jq "$BODY" '.result.received.msg == "hello from e2e script"' "upstream echoed our input"
assert_jq "$BODY" '.latency_ms >= 0'                           "latency_ms present"
echo ""

# ── 4. Print summary ────────────────────────────────────────────────────────
REQ_ID="$(echo "$BODY" | jq -r '.request_id')"
LATENCY="$(echo "$BODY" | jq -r '.latency_ms')"
DECISION="$(echo "$BODY" | jq -r '.decision')"

echo "═══════════════════════════════════════════════════════════"
echo " PASS — tool '${TOOL_NAME}' called successfully"
echo ""
echo "   request_id : ${REQ_ID}"
echo "   decision   : ${DECISION}"
echo "   latency    : ${LATENCY}ms"
echo "   input sent : {\"msg\":\"hello from e2e script\"}"
echo "   upstream   : echoed back correctly"
echo "═══════════════════════════════════════════════════════════"
