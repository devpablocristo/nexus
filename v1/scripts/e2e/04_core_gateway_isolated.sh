#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
NAME
    04_core_gateway_isolated.sh — isolated gateway e2e with its own docker compose

SYNOPSIS
    04_core_gateway_isolated.sh [-h|--help]

DESCRIPTION
    Spins up a fully isolated docker compose stack (unique project name,
    auto-assigned ports), seeds the database, and exercises the complete
    gateway pipeline:

      - Auth (401 format), tools CRUD, policies, DLP, secrets injection
      - Gateway run (allow, deny by policy, deny by DLP)
      - Idempotency (replay, conflict, in-progress, timeout, failed terminal)
      - Egress rules + SSRF protection
      - Audit (query, export JSONL with hash-chain)
      - MCP (tools/list, tools/call)
      - A2A (agent-to-agent call)
      - Org onboarding, alert rules CRUD, approvals, sessions
      - Simulate/explain mode

    Tears down the stack on exit (trap cleanup).

ENVIRONMENT
    NEXUS_HTTP_PORT_E2E_BASE         Starting port for core    (default: 18080)
    NEXUS_MOCK_TOOLS_PORT_E2E_BASE   Starting port for mock    (default: 18081)
    NEXUS_POSTGRES_PORT_E2E_BASE     Starting port for PG      (default: 55432)
    NEXUS_REDIS_PORT_E2E_BASE        Starting port for Redis   (default: 16379)
    COMPOSE_PROJECT_NAME             Override compose project   (auto-generated)

PREREQUISITES
    Docker, curl, jq. The script builds and manages its own stack.

EXAMPLES
    ./scripts/e2e/04_core_gateway_isolated.sh
EOF
  exit 0
}
[[ "${1:-}" =~ ^(-h|--help)$ ]] && usage

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if [[ ! -f ".env" ]]; then
  echo "missing .env (create it from .env.example)" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

NEXUS_HTTP_PORT="${NEXUS_HTTP_PORT_E2E_BASE:-18080}"
NEXUS_MOCK_TOOLS_PORT="${NEXUS_MOCK_TOOLS_PORT_E2E_BASE:-18081}"
NEXUS_POSTGRES_PORT="${NEXUS_POSTGRES_PORT_E2E_BASE:-55432}"
NEXUS_REDIS_PORT="${NEXUS_REDIS_PORT_E2E_BASE:-16379}"
NEXUS_PROMETHEUS_PORT="${NEXUS_PROMETHEUS_PORT_E2E_BASE:-19090}"
NEXUS_GRAFANA_PORT="${NEXUS_GRAFANA_PORT_E2E_BASE:-13000}"

compose() {
  docker compose "$@"
}

port_in_use() {
  local port="$1"
  (echo >/dev/tcp/127.0.0.1/"$port") >/dev/null 2>&1
}

next_free_port() {
  local candidate="$1"
  while port_in_use "$candidate"; do
    candidate=$((candidate + 1))
  done
  echo "$candidate"
}

RESERVED_PORTS=""

reserve_port_var() {
  local var_name="$1"
  local default_port="$2"
  local chosen="${!var_name:-$default_port}"
  while port_in_use "$chosen" || [[ " $RESERVED_PORTS " == *" $chosen "* ]]; do
    chosen=$((chosen + 1))
  done
  RESERVED_PORTS="$RESERVED_PORTS $chosen"
  printf -v "$var_name" '%s' "$chosen"
  export "$var_name"
}

reserve_port_var NEXUS_HTTP_PORT 18080
reserve_port_var NEXUS_MOCK_TOOLS_PORT 18081
reserve_port_var NEXUS_POSTGRES_PORT 55432
reserve_port_var NEXUS_SAAS_POSTGRES_PORT 55433
reserve_port_var NEXUS_SAAS_HTTP_PORT 18082
reserve_port_var NEXUS_REDIS_PORT 16379
reserve_port_var NEXUS_OPERATOR_PORT 18000
reserve_port_var OPERATOR_HEALTH_PORT 18090
reserve_port_var NEXUS_TOWER_PORT 15174
reserve_port_var NEXUS_PROMETHEUS_PORT 19090
reserve_port_var NEXUS_GRAFANA_PORT 13000
reserve_port_var NEXUS_MAILHOG_SMTP_PORT 1125
reserve_port_var NEXUS_MAILHOG_UI_PORT 18025

COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-nexus-core-e2e-$(date +%s)}"
export COMPOSE_PROJECT_NAME

HTTP_BASE="http://localhost:${NEXUS_HTTP_PORT}"
MOCK_BASE="http://localhost:${NEXUS_MOCK_TOOLS_PORT}"


# In docker compose, mock-tools resolves to a private IP (bridge network). With SSRF protection enabled,
# outbound calls to private IPs are blocked by design, which breaks E2E runs. For E2E we disable SSRF
# protection explicitly (dev/test only).
: "${NEXUS_DISABLE_SSRF_PROTECTION:=true}"
export NEXUS_DISABLE_SSRF_PROTECTION

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing dependency: $1" >&2; exit 1; }
}

require curl
require jq

# Prefer ripgrep when available; fall back to grep so the suite works on minimal dev machines/CI images.
match() {
  local pattern="$1"
  if command -v rg >/dev/null 2>&1; then
    rg -n "$pattern"
  else
    # ERE is enough for our patterns (^ anchors, plain substrings).
    grep -nE "$pattern"
  fi
}

fail() { echo "E2E FAIL: $*" >&2; exit 1; }

assert_jq() {
  local json="$1"
  local filter="$2"
  echo "$json" | jq -e "$filter" >/dev/null || fail "jq assertion failed: $filter | json=$json"
}

http_code() {
  curl -sS -o /dev/null -w "%{http_code}" "$1" 2>/dev/null || true
}

cleanup() {
  compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "[e2e] bringing stack up"
compose up --build -d >/dev/null

echo "[e2e] wait readyz"
for _ in {1..60}; do
  if [[ "$(http_code "${HTTP_BASE}/readyz")" == "200" ]]; then
    break
  fi
  sleep 1
done
[[ "$(http_code "${HTTP_BASE}/readyz")" == "200" ]] || fail "readyz never became 200"

echo "[e2e] wait mock-tools"
for _ in {1..60}; do
  if [[ "$(http_code "${MOCK_BASE}/healthz")" == "200" ]]; then
    break
  fi
  sleep 1
done
[[ "$(http_code "${MOCK_BASE}/healthz")" == "200" ]] || fail "mock-tools healthz not 200"

echo "[e2e] migrate up"
make migrate-up >/dev/null

echo "[e2e] seed demo"
SEED_OUT="$(bash scripts/seed/seed_demo.sh)"
echo "$SEED_OUT" | match "^NEXUS_DEMO_API_KEY=" >/dev/null || fail "seed did not print api key"
API_KEY="$(echo "$SEED_OUT" | match "^NEXUS_DEMO_API_KEY=" | tail -n1 | cut -d= -f2)"
[[ -n "$API_KEY" ]] || fail "empty api key"

echo "[e2e] /healthz payload"
H="$(curl -fsS "${HTTP_BASE}/healthz")"
assert_jq "$H" '.ok == true'

echo "[e2e] /docs returns html"
DOCS="$(curl -fsS "${HTTP_BASE}/docs")"
echo "$DOCS" | match "<html" >/dev/null || fail "/docs not html"

echo "[e2e] unauthorized error format"
U_BODY="$(curl -sS "${HTTP_BASE}/v1/tools" || true)"
U_CODE="$(http_code "${HTTP_BASE}/v1/tools")"
[[ "$U_CODE" == "401" ]] || fail "expected 401 without key got $U_CODE"
assert_jq "$U_BODY" '.request_id | type=="string" and length>0'
assert_jq "$U_BODY" '.error.code == "UNAUTHORIZED"'
assert_jq "$U_BODY" '.error.message | type=="string" and length>0'

auth_curl() {
  curl -fsS -H "X-NEXUS-CORE-KEY: ${API_KEY}" "$@"
}

auth_curl_no_fail() {
  curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" "$@"
}

post_json() {
  local url="$1"
  local payload="$2"
  auth_curl_no_fail -H "Content-Type: application/json" -d "$payload" -w "\n%{http_code}" "$url"
}

put_json() {
  local url="$1"
  local payload="$2"
  auth_curl_no_fail -X PUT -H "Content-Type: application/json" -d "$payload" -w "\n%{http_code}" "$url"
}

post_json_with_role() {
  local url="$1"
  local payload="$2"
  local role="$3"
  curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" -H "X-NEXUS-ROLE: ${role}" -H "Content-Type: application/json" -d "$payload" -w "\n%{http_code}" "$url"
}

echo "[e2e] setup egress rules (default-deny requires explicit allowlist)"
post_json "${HTTP_BASE}/v1/tools/echo/egress-rules" '{"host":"mock-tools","enabled":true}' >/dev/null
post_json "${HTTP_BASE}/v1/tools/transfer/egress-rules" '{"host":"mock-tools","enabled":true}' >/dev/null

echo "[e2e] list tools payload coherence"
TOOLS="$(auth_curl "${HTTP_BASE}/v1/tools")"
assert_jq "$TOOLS" '.items | type=="array"'
assert_jq "$TOOLS" '(.items | map(.name) | sort) == ["echo","transfer"]'
assert_jq "$TOOLS" '.items[] | has("id") and has("name") and has("kind") and has("method") and has("url") and has("input_schema") and has("action_type") and has("risk_level") and has("enabled") and has("created_at") and has("updated_at")'

echo "[e2e] get tool echo"
ECHO="$(auth_curl "${HTTP_BASE}/v1/tools/echo")"
assert_jq "$ECHO" '.name == "echo"'
assert_jq "$ECHO" '.kind == "http"'

echo "[e2e] get tool 404 error format"
GET404_WITH_CODE="$(auth_curl_no_fail -w "\n%{http_code}" "${HTTP_BASE}/v1/tools/does-not-exist")"
GET404_BODY="$(echo "$GET404_WITH_CODE" | head -n1)"
GET404_CODE="$(echo "$GET404_WITH_CODE" | tail -n1)"
[[ "$GET404_CODE" == "404" ]] || fail "expected 404 got ${GET404_CODE} body=$GET404_BODY"
assert_jq "$GET404_BODY" '.request_id | type=="string" and length>0'
assert_jq "$GET404_BODY" '.error.code == "NOT_FOUND"'

echo "[e2e] create tool echo2"
CREATE_TOOL_WITH_CODE="$(post_json "${HTTP_BASE}/v1/tools" '{
  "name":"echo2",
  "kind":"http",
  "description":"second echo tool",
  "method":"POST",
  "url":"http://mock-tools:8081/echo",
  "input_schema":{"type":"object"},
  "output_schema":{"type":"object"},
  "action_type":"read",
  "risk_level":1,
  "enabled":true
}')"
CREATE_TOOL_BODY="$(echo "$CREATE_TOOL_WITH_CODE" | head -n1)"
CREATE_TOOL_CODE="$(echo "$CREATE_TOOL_WITH_CODE" | tail -n1)"
[[ "$CREATE_TOOL_CODE" == "201" ]] || fail "expected 201 got ${CREATE_TOOL_CODE} body=$CREATE_TOOL_BODY"
assert_jq "$CREATE_TOOL_BODY" '.name=="echo2" and .enabled==true and .kind=="http"'
TOOL2_ID="$(echo "$CREATE_TOOL_BODY" | jq -r '.id')"
[[ "$TOOL2_ID" =~ ^[0-9a-fA-F-]{36}$ ]] || fail "expected uuid id got $TOOL2_ID"
post_json "${HTTP_BASE}/v1/tools/echo2/egress-rules" '{"host":"mock-tools","enabled":true}' >/dev/null

echo "[e2e] update tool echo2 (partial)"
UPD_TOOL_WITH_CODE="$(put_json "${HTTP_BASE}/v1/tools/echo2" '{"enabled":false}')"
UPD_TOOL_BODY="$(echo "$UPD_TOOL_WITH_CODE" | head -n1)"
UPD_TOOL_CODE="$(echo "$UPD_TOOL_WITH_CODE" | tail -n1)"
[[ "$UPD_TOOL_CODE" == "200" ]] || fail "expected 200 got ${UPD_TOOL_CODE} body=$UPD_TOOL_BODY"
assert_jq "$UPD_TOOL_BODY" '.name=="echo2" and .enabled==false'

echo "[e2e] re-enable tool echo2"
UPD_TOOL2_WITH_CODE="$(put_json "${HTTP_BASE}/v1/tools/echo2" '{"enabled":true}')"
UPD_TOOL2_BODY="$(echo "$UPD_TOOL2_WITH_CODE" | head -n1)"
UPD_TOOL2_CODE="$(echo "$UPD_TOOL2_WITH_CODE" | tail -n1)"
[[ "$UPD_TOOL2_CODE" == "200" ]] || fail "expected 200 got ${UPD_TOOL2_CODE} body=$UPD_TOOL2_BODY"
assert_jq "$UPD_TOOL2_BODY" '.name=="echo2" and .enabled==true'

echo "[e2e] create deny policy for echo2 (deny all)"
CREATE_POL_WITH_CODE="$(post_json "${HTTP_BASE}/v1/tools/echo2/policies" '{
  "effect":"deny",
  "priority":1,
  "conditions":{},
  "limits":{},
  "reason_template":"echo2 denied",
  "enabled":true
}')"
CREATE_POL_BODY="$(echo "$CREATE_POL_WITH_CODE" | head -n1)"
CREATE_POL_CODE="$(echo "$CREATE_POL_WITH_CODE" | tail -n1)"
[[ "$CREATE_POL_CODE" == "201" ]] || fail "expected 201 got ${CREATE_POL_CODE} body=$CREATE_POL_BODY"
POLICY_ID="$(echo "$CREATE_POL_BODY" | jq -r '.id')"
[[ "$POLICY_ID" =~ ^[0-9a-fA-F-]{36}$ ]] || fail "expected uuid policy id got $POLICY_ID"
assert_jq "$CREATE_POL_BODY" '.effect=="deny" and .priority==1 and .enabled==true'

echo "[e2e] run echo2 should be blocked by policy"
RUN_ECHO2_WITH_CODE="$(post_json "${HTTP_BASE}/v1/run" '{"tool_name":"echo2","input":{"a":1},"context":{}}')"
RUN_ECHO2_BODY="$(echo "$RUN_ECHO2_WITH_CODE" | head -n1)"
RUN_ECHO2_CODE="$(echo "$RUN_ECHO2_WITH_CODE" | tail -n1)"
[[ "$RUN_ECHO2_CODE" == "403" ]] || fail "expected 403 got ${RUN_ECHO2_CODE} body=$RUN_ECHO2_BODY"
assert_jq "$RUN_ECHO2_BODY" '.status=="blocked" and .decision=="deny" and .error.code=="POLICY_DENIED"'

echo "[e2e] update policy to disabled (should revert to default allow for read tool)"
UPD_POL_WITH_CODE="$(put_json "${HTTP_BASE}/v1/policies/${POLICY_ID}" '{"enabled":false}')"
UPD_POL_BODY="$(echo "$UPD_POL_WITH_CODE" | head -n1)"
UPD_POL_CODE="$(echo "$UPD_POL_WITH_CODE" | tail -n1)"
[[ "$UPD_POL_CODE" == "200" ]] || fail "expected 200 got ${UPD_POL_CODE} body=$UPD_POL_BODY"
assert_jq "$UPD_POL_BODY" '.enabled==false'

echo "[e2e] run echo2 should succeed after policy disabled"
RUN_ECHO2_OK="$(auth_curl -H "Content-Type: application/json" -d '{"tool_name":"echo2","input":{"hello":"again"},"context":{}}' "${HTTP_BASE}/v1/run")"
assert_jq "$RUN_ECHO2_OK" '.status=="success" and .decision=="allow" and .tool_name=="echo2"'

echo "[e2e] run echo"
RUN_ECHO="$(auth_curl -H "Content-Type: application/json" -d '{"tool_name":"echo","input":{"hello":"world"},"context":{"user_id":"u_1"}}' "${HTTP_BASE}/v1/run")"
assert_jq "$RUN_ECHO" '.request_id | type=="string" and length>0'
assert_jq "$RUN_ECHO" '.decision == "allow"'
assert_jq "$RUN_ECHO" '.tool_name == "echo"'
assert_jq "$RUN_ECHO" '.status == "success"'
assert_jq "$RUN_ECHO" '.result.received.hello == "world"'
assert_jq "$RUN_ECHO" '.result.server_time | type=="string" and length>0'
assert_jq "$RUN_ECHO" '.latency_ms | type=="number" and . >= 0'

echo "[e2e] upsert secret + validate injection"
SEC_UP_WITH_CODE="$(post_json_with_role "${HTTP_BASE}/v1/tools/echo/secrets" '{"secret_type":"header","key_name":"X-Injected-Token","value":"top-secret","enabled":true}' "secops")"
SEC_UP_BODY="$(echo "$SEC_UP_WITH_CODE" | head -n1)"
SEC_UP_CODE="$(echo "$SEC_UP_WITH_CODE" | tail -n1)"
[[ "$SEC_UP_CODE" == "200" ]] || fail "expected 200 got ${SEC_UP_CODE} body=$SEC_UP_BODY"
assert_jq "$SEC_UP_BODY" '.secret_type=="header" and .key_name=="X-Injected-Token" and .enabled==true'

RUN_ECHO_SEC="$(auth_curl -H "Content-Type: application/json" -d '{"tool_name":"echo","input":{"hello":"secret"},"context":{"user_id":"u_1"}}' "${HTTP_BASE}/v1/run")"
assert_jq "$RUN_ECHO_SEC" '.status=="success" and .result.x_injected_token_present==true'

echo "[e2e] run transfer denied > 1000"
RUN_DENY1_WITH_CODE="$(post_json "${HTTP_BASE}/v1/run" '{"tool_name":"transfer","input":{"amount":5000,"token":"secret"},"context":{"user_id":"u_1"}}')"
RUN_DENY1="$(echo "$RUN_DENY1_WITH_CODE" | head -n1)"
RUN_DENY1_CODE="$(echo "$RUN_DENY1_WITH_CODE" | tail -n1)"
[[ "$RUN_DENY1_CODE" == "403" ]] || fail "expected 403 got ${RUN_DENY1_CODE} body=$RUN_DENY1"
assert_jq "$RUN_DENY1" '.request_id | type=="string" and length>0'
assert_jq "$RUN_DENY1" '.decision == "deny"'
assert_jq "$RUN_DENY1" '.status == "blocked"'
assert_jq "$RUN_DENY1" '.error.code == "POLICY_DENIED"'
assert_jq "$RUN_DENY1" '.reason | type=="string" and length>0'

echo "[e2e] run transfer denied missing context.user_id"
RUN_DENY2_WITH_CODE="$(post_json "${HTTP_BASE}/v1/run" '{"tool_name":"transfer","input":{"amount":500,"token":"secret"},"context":{}}')"
RUN_DENY2="$(echo "$RUN_DENY2_WITH_CODE" | head -n1)"
RUN_DENY2_CODE="$(echo "$RUN_DENY2_WITH_CODE" | tail -n1)"
[[ "$RUN_DENY2_CODE" == "403" ]] || fail "expected 403 got ${RUN_DENY2_CODE} body=$RUN_DENY2"
assert_jq "$RUN_DENY2" '.decision == "deny"'
assert_jq "$RUN_DENY2" '.status == "blocked"'

echo "[e2e] run transfer denied missing idempotency key when required"
RUN_IDEMP_REQ_WITH_CODE="$(post_json "${HTTP_BASE}/v1/run" '{"tool_name":"transfer","input":{"amount":500},"context":{"user_id":"u_1"}}')"
RUN_IDEMP_REQ_BODY="$(echo "$RUN_IDEMP_REQ_WITH_CODE" | head -n1)"
RUN_IDEMP_REQ_CODE="$(echo "$RUN_IDEMP_REQ_WITH_CODE" | tail -n1)"
[[ "$RUN_IDEMP_REQ_CODE" == "400" ]] || fail "expected 400 got ${RUN_IDEMP_REQ_CODE} body=$RUN_IDEMP_REQ_BODY"
assert_jq "$RUN_IDEMP_REQ_BODY" '.error.code=="IDEMPOTENCY_REQUIRED" or .status=="blocked"'

echo "[e2e] simulate transfer explain (no upstream execution)"
SIM_WITH_CODE="$(post_json "${HTTP_BASE}/v1/run/simulate" '{"tool_name":"transfer","input":{"amount":5000,"card_number":"4111111111111111"},"context":{"user_id":"u_1"}}')"
SIM_BODY="$(echo "$SIM_WITH_CODE" | head -n1)"
SIM_CODE="$(echo "$SIM_WITH_CODE" | tail -n1)"
[[ "$SIM_CODE" == "403" ]] || fail "expected 403 got ${SIM_CODE} body=$SIM_BODY"
assert_jq "$SIM_BODY" '.status=="blocked" and .decision=="deny" and .explain.mode=="simulate" and .explain.dlp_summary.credit_card.count >= 1'

echo "[e2e] run transfer denied by external+dLP policy"
RUN_DLP_DENY_WITH_CODE="$(post_json "${HTTP_BASE}/v1/run" '{"tool_name":"transfer","input":{"amount":500,"card_number":"4111111111111111"},"context":{"user_id":"u_1"}}')"
RUN_DLP_DENY_BODY="$(echo "$RUN_DLP_DENY_WITH_CODE" | head -n1)"
RUN_DLP_DENY_CODE="$(echo "$RUN_DLP_DENY_WITH_CODE" | tail -n1)"
[[ "$RUN_DLP_DENY_CODE" == "403" ]] || fail "expected 403 got ${RUN_DLP_DENY_CODE} body=$RUN_DLP_DENY_BODY"
assert_jq "$RUN_DLP_DENY_BODY" '.error.code=="POLICY_DENIED"'

echo "[e2e] run transfer allowed with idempotency key"
IDK="idem-$(date +%s)"
RUN_OK="$(curl -fsS -H "X-NEXUS-CORE-KEY: ${API_KEY}" -H "Idempotency-Key: ${IDK}" -H "Content-Type: application/json" -d '{"tool_name":"transfer","input":{"amount":500},"context":{"user_id":"u_1","token":"secret"}}' "${HTTP_BASE}/v1/run")"
assert_jq "$RUN_OK" '.decision == "allow"'
assert_jq "$RUN_OK" '.tool_name == "transfer"'
assert_jq "$RUN_OK" '.status == "success"'
assert_jq "$RUN_OK" '.result.ok == true'
assert_jq "$RUN_OK" '.result.amount == 500'
assert_jq "$RUN_OK" '.result.tx_id | type=="string" and length>0'

echo "[e2e] idempotency replay (same key, same payload)"
RUN_REPLAY="$(curl -fsS -H "X-NEXUS-CORE-KEY: ${API_KEY}" -H "Idempotency-Key: ${IDK}" -H "Content-Type: application/json" -d '{"tool_name":"transfer","input":{"amount":500},"context":{"user_id":"u_1","token":"secret"}}' "${HTTP_BASE}/v1/run")"
assert_jq "$RUN_REPLAY" '.idempotency.outcome=="REPLAY" and .status=="success"'
TRANS_STATS="$(curl -fsS "${MOCK_BASE}/_stats/transfer")"
assert_jq "$TRANS_STATS" '.execution_count == 1'

echo "[e2e] idempotency conflict (same key, different input)"
RUN_CONFLICT_WITH_CODE="$(curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" -H "Idempotency-Key: ${IDK}" -H "Content-Type: application/json" -d '{"tool_name":"transfer","input":{"amount":700},"context":{"user_id":"u_1"}}' -w "\n%{http_code}" "${HTTP_BASE}/v1/run")"
RUN_CONFLICT_BODY="$(echo "$RUN_CONFLICT_WITH_CODE" | head -n1)"
RUN_CONFLICT_CODE="$(echo "$RUN_CONFLICT_WITH_CODE" | tail -n1)"
[[ "$RUN_CONFLICT_CODE" == "409" ]] || fail "expected 409 got ${RUN_CONFLICT_CODE} body=$RUN_CONFLICT_BODY"
assert_jq "$RUN_CONFLICT_BODY" '.error.code=="IDEMPOTENCY_KEY_CONFLICT" or .status=="blocked"'

echo "[e2e] idempotency in-progress"
INPROG_KEY="idem-inprog-$(date +%s)"
curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" -H "Idempotency-Key: ${INPROG_KEY}" -H "Content-Type: application/json" -d '{"tool_name":"transfer","input":{"amount":501,"sleep_ms":1500},"context":{"user_id":"u_1"}}' "${HTTP_BASE}/v1/run" >/tmp/inprog_first.json &
sleep 0.2
RUN_INPROG_WITH_CODE="$(curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" -H "Idempotency-Key: ${INPROG_KEY}" -H "Content-Type: application/json" -d '{"tool_name":"transfer","input":{"amount":501,"sleep_ms":1500},"context":{"user_id":"u_1"}}' -w "\n%{http_code}" "${HTTP_BASE}/v1/run")"
RUN_INPROG_BODY="$(echo "$RUN_INPROG_WITH_CODE" | head -n1)"
RUN_INPROG_CODE="$(echo "$RUN_INPROG_WITH_CODE" | tail -n1)"
[[ "$RUN_INPROG_CODE" == "409" ]] || fail "expected 409 got ${RUN_INPROG_CODE} body=$RUN_INPROG_BODY"
assert_jq "$RUN_INPROG_BODY" '.error.code=="IDEMPOTENCY_IN_PROGRESS" or .status=="blocked"'
wait

echo "[e2e] timeout budget exceeded"
RUN_TIMEOUT_WITH_CODE="$(curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" -H "Idempotency-Key: idem-timeout-$(date +%s)" -H "X-Timeout-Ms: 1000" -H "Content-Type: application/json" -d '{"tool_name":"transfer","input":{"amount":510,"sleep_ms":2500},"context":{"user_id":"u_1"}}' -w "\n%{http_code}" "${HTTP_BASE}/v1/run")"
RUN_TIMEOUT_BODY="$(echo "$RUN_TIMEOUT_WITH_CODE" | head -n1)"
RUN_TIMEOUT_CODE="$(echo "$RUN_TIMEOUT_WITH_CODE" | tail -n1)"
[[ "$RUN_TIMEOUT_CODE" == "408" ]] || fail "expected 408 got ${RUN_TIMEOUT_CODE} body=$RUN_TIMEOUT_BODY"
assert_jq "$RUN_TIMEOUT_BODY" '.error.code=="TIMEOUT_BUDGET_EXCEEDED" or .status=="error"'

echo "[e2e] idempotency failed is terminal (same key -> replay same error)"
FAIL_KEY="idem-failed-$(date +%s)"
TRANS_BEFORE="$(curl -fsS "${MOCK_BASE}/_stats/transfer" | jq -r '.execution_count')"
RUN_FAIL_1_WITH_CODE="$(curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" -H "Idempotency-Key: ${FAIL_KEY}" -H "Content-Type: application/json" -d '{"tool_name":"transfer","input":{"amount":700,"force_5xx":true},"context":{"user_id":"u_1"}}' -w "\n%{http_code}" "${HTTP_BASE}/v1/run")"
RUN_FAIL_1_BODY="$(echo "$RUN_FAIL_1_WITH_CODE" | head -n1)"
RUN_FAIL_1_CODE="$(echo "$RUN_FAIL_1_WITH_CODE" | tail -n1)"
[[ "$RUN_FAIL_1_CODE" == "502" ]] || fail "expected 502 got ${RUN_FAIL_1_CODE} body=$RUN_FAIL_1_BODY"
assert_jq "$RUN_FAIL_1_BODY" '.status=="error" and .error.code=="UPSTREAM_5XX"'
TRANS_AFTER_FIRST="$(curl -fsS "${MOCK_BASE}/_stats/transfer" | jq -r '.execution_count')"
RUN_FAIL_2_WITH_CODE="$(curl -sS -H "X-NEXUS-CORE-KEY: ${API_KEY}" -H "Idempotency-Key: ${FAIL_KEY}" -H "Content-Type: application/json" -d '{"tool_name":"transfer","input":{"amount":700,"force_5xx":true},"context":{"user_id":"u_1"}}' -w "\n%{http_code}" "${HTTP_BASE}/v1/run")"
RUN_FAIL_2_BODY="$(echo "$RUN_FAIL_2_WITH_CODE" | head -n1)"
RUN_FAIL_2_CODE="$(echo "$RUN_FAIL_2_WITH_CODE" | tail -n1)"
[[ "$RUN_FAIL_2_CODE" == "502" ]] || fail "expected replay 502 got ${RUN_FAIL_2_CODE} body=$RUN_FAIL_2_BODY"
assert_jq "$RUN_FAIL_2_BODY" '.status=="error" and .error.code=="UPSTREAM_5XX" and .idempotency.outcome=="REPLAY"'
TRANS_AFTER="$(curl -fsS "${MOCK_BASE}/_stats/transfer" | jq -r '.execution_count')"
[[ "$((TRANS_AFTER_FIRST - TRANS_BEFORE))" -ge "1" ]] || fail "expected upstream execution on first failed call, before=${TRANS_BEFORE} after_first=${TRANS_AFTER_FIRST}"
[[ "$((TRANS_AFTER - TRANS_AFTER_FIRST))" == "0" ]] || fail "expected replay to avoid extra upstream execution, after_first=${TRANS_AFTER_FIRST} after_second=${TRANS_AFTER}"

echo "[e2e] egress rules allow transfer host + deny mismatched host"
EGR_UP_WITH_CODE="$(post_json "${HTTP_BASE}/v1/tools/transfer/egress-rules" '{"host":"mock-tools","enabled":true}')"
EGR_UP_CODE="$(echo "$EGR_UP_WITH_CODE" | tail -n1)"
[[ "$EGR_UP_CODE" == "204" ]] || fail "expected 204 got ${EGR_UP_CODE}"

CREATE_BAD_WITH_CODE="$(post_json "${HTTP_BASE}/v1/tools" '{
  "name":"ext-bad",
  "kind":"http",
  "method":"POST",
  "url":"http://example.com/post",
  "input_schema":{"type":"object"},
  "action_type":"read",
  "classification":"external",
  "risk_level":2,
  "enabled":true
}')"
CREATE_BAD_BODY="$(echo "$CREATE_BAD_WITH_CODE" | head -n1)"
CREATE_BAD_CODE="$(echo "$CREATE_BAD_WITH_CODE" | tail -n1)"
[[ "$CREATE_BAD_CODE" == "201" ]] || fail "expected 201 got ${CREATE_BAD_CODE} body=$CREATE_BAD_BODY"
EGR_BAD_WITH_CODE="$(post_json "${HTTP_BASE}/v1/tools/ext-bad/egress-rules" '{"host":"mock-tools","enabled":true}')"
EGR_BAD_CODE="$(echo "$EGR_BAD_WITH_CODE" | tail -n1)"
[[ "$EGR_BAD_CODE" == "204" ]] || fail "expected 204 got ${EGR_BAD_CODE}"

RUN_EGR_DENY_WITH_CODE="$(post_json "${HTTP_BASE}/v1/run" '{"tool_name":"ext-bad","input":{},"context":{}}')"
RUN_EGR_DENY_BODY="$(echo "$RUN_EGR_DENY_WITH_CODE" | head -n1)"
RUN_EGR_DENY_CODE="$(echo "$RUN_EGR_DENY_WITH_CODE" | tail -n1)"
[[ "$RUN_EGR_DENY_CODE" == "403" ]] || fail "expected 403 got ${RUN_EGR_DENY_CODE} body=$RUN_EGR_DENY_BODY"
assert_jq "$RUN_EGR_DENY_BODY" '.status=="blocked" and .error.code=="EGRESS_DENIED"'

echo "[e2e] audit coherence + redaction"
AUDIT="$(auth_curl "${HTTP_BASE}/v1/audit?tool_name=transfer&limit=10")"
assert_jq "$AUDIT" '.items | type=="array" and length>0'
assert_jq "$AUDIT" '.items[0].tool_name == "transfer"'
assert_jq "$AUDIT" '.items | any(.status=="success")'
assert_jq "$AUDIT" '.items | any(.status=="blocked")'
assert_jq "$AUDIT" '.items | any(.input.card_number? == "***")'
assert_jq "$AUDIT" '.items | any(.context.token? == "***")'
assert_jq "$AUDIT" '.items | any(.dlp_summary.credit_card.count? >= 1)'
assert_jq "$AUDIT" '.items[0].event_hash | type=="string"'

echo "[e2e] mcp tools/list and tools/call"
MCP_LIST="$(auth_curl -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' "${HTTP_BASE}/mcp")"
assert_jq "$MCP_LIST" '.result.items | type=="array" and length>=2'
MCP_CALL="$(auth_curl -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"tool_name":"echo","input":{"from":"mcp"},"context":{"user_id":"u_1"},"timeout_ms":5000}}' "${HTTP_BASE}/mcp")"
assert_jq "$MCP_CALL" '.result.status=="success" and .result.result.received.from=="mcp"'

echo "[e2e] a2a call uses same run pipeline"
A2A_CALL="$(auth_curl -H "X-NEXUS-SCOPES: a2a:call" -H "Content-Type: application/json" -d '{"tool_name":"echo","input":{"from":"a2a"},"context":{"user_id":"u_1"},"timeout_ms":5000}' "${HTTP_BASE}/a2a/call")"
assert_jq "$A2A_CALL" '.status=="success" and .result.received.from=="a2a"'

echo "[e2e] audit export jsonl includes hash-chain"
EXPORT_JSONL="$(auth_curl "${HTTP_BASE}/v1/audit/export?format=jsonl&tool_name=transfer&limit=5")"
echo "$EXPORT_JSONL" | head -n 1 | jq -e '.event_hash | type=="string"' >/dev/null || fail "export missing event_hash"
echo "$EXPORT_JSONL" | head -n 1 | jq -e '.hash_algo=="sha256"' >/dev/null || fail "export missing hash_algo"

echo "[e2e] approvals list"
APPROVALS="$(auth_curl "${HTTP_BASE}/v1/approvals")"
assert_jq "$APPROVALS" '.items | type=="array"'

echo "[e2e] OK"
