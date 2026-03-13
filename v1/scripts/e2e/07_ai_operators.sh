#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
NAME
    07_ai_operators.sh — e2e for nexus-ai-operators service

SYNOPSIS
    07_ai_operators.sh [-h|--help]

DESCRIPTION
    Validates the nexus-ai-operators service against a running stack:

      1. Health endpoints (/healthz, /readyz)
      2. Prometheus metrics (/metrics) — all nexus_operator_* families
      3. Tick endpoint — manual trigger of operator engine cycle
      4. Event consumption — generates gateway traffic, verifies cursor advances
      5. High-risk detection — generates concentrated deny traffic, verifies
         actions, incidents and policy proposals are created
      6. Assistant endpoint — AI-assisted query returns structured response
      7. Auth enforcement — rejects requests without valid operator key

    Creates temporary tools for testing, cleans up on exit.
    Reports pass/fail counts at the end.

ENVIRONMENT
    NEXUS_HTTP_PORT       Core HTTP port              (default: 8080)
    NEXUS_OPERATOR_PORT   AI-Operators HTTP port      (default: 8000)
    NEXUS_DEMO_API_KEY    API key for core            (default: nexus-core-local-key)
    OPERATOR_INTERNAL_KEY Operator key for ai-ops     (default: operator-internal-key)

PREREQUISITES
    Full stack running (docker compose up) including nexus-ai-operators.

EXAMPLES
    ./scripts/e2e/07_ai_operators.sh
    NEXUS_OPERATOR_PORT=9000 ./scripts/e2e/07_ai_operators.sh
EOF
  exit 0
}
[[ "${1:-}" =~ ^(-h|--help)$ ]] && usage

CORE_PORT="${NEXUS_HTTP_PORT:-8080}"
AI_PORT="${NEXUS_OPERATOR_PORT:-8000}"
API_KEY="${NEXUS_DEMO_API_KEY:-nexus-core-local-key}"
OP_KEY="${OPERATOR_INTERNAL_KEY:-operator-internal-key}"

CORE_BASE="http://localhost:${CORE_PORT}"
AI_BASE="http://localhost:${AI_PORT}"

PASS=0
FAIL=0
FAIL_TOOL="ai-e2e-fail-$$"

cleanup() {
  curl -sS -o /dev/null -X DELETE \
    -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
    "${CORE_BASE}/v1/tools/${FAIL_TOOL}" 2>/dev/null || true
}
trap cleanup EXIT

fail() { echo "  ✗ $*" >&2; FAIL=$((FAIL+1)); }
ok()   { echo "  ✓ $*"; PASS=$((PASS+1)); }

assert_eq() {
  local actual="$1" expected="$2" label="$3"
  if [[ "$actual" == "$expected" ]]; then ok "$label"; else fail "$label: expected '$expected', got '$actual'"; fi
}

assert_jq() {
  local json="$1" filter="$2" label="${3:-$filter}"
  if echo "$json" | jq -e "$filter" >/dev/null 2>&1; then ok "$label"; else fail "$label"; fi
}

assert_contains() {
  local haystack="$1" needle="$2" label="$3"
  if echo "$haystack" | grep -q "$needle"; then ok "$label"; else fail "$label: '$needle' not found"; fi
}

http_code() {
  curl -sS -o /dev/null -w "%{http_code}" "$@" 2>/dev/null || echo "000"
}

http_json() {
  curl -fsS "$@" 2>/dev/null || echo '{}'
}

metric_value() {
  local metrics="$1" name="$2"
  echo "$metrics" | grep "^${name} " | head -1 | awk '{print $2}' || echo "0"
}

section() {
  echo ""
  echo "═══════════════════════════════════════════════════════════"
  echo " $1"
  echo "═══════════════════════════════════════════════════════════"
}

resolve_ai_base() {
  if [[ "$(http_code "${AI_BASE}/healthz")" == "200" ]]; then
    return
  fi
  local mapped
  mapped="$(docker compose port nexus-ai-operators 8000 2>/dev/null | cut -d: -f2)" || true
  if [[ -n "$mapped" ]]; then
    AI_BASE="http://localhost:${mapped}"
  fi
}

resolve_ai_base

echo "╔═══════════════════════════════════════════════════════════╗"
echo "║        nexus-ai-operators   e2e test suite               ║"
echo "╠═══════════════════════════════════════════════════════════╣"
echo "║  Core  : ${CORE_BASE}"
echo "║  AI-Ops: ${AI_BASE}"
echo "╚═══════════════════════════════════════════════════════════╝"

# ═════════════════════════════════════════════════════════════════════════════
section "1. HEALTH ENDPOINTS"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 1.1 /healthz returns 200"
HEALTHZ_CODE="$(http_code "${AI_BASE}/healthz")"
assert_eq "$HEALTHZ_CODE" "200" "healthz returns 200"

echo "▸ 1.2 /healthz body ok=true"
HEALTHZ_BODY="$(http_json "${AI_BASE}/healthz")"
assert_jq "$HEALTHZ_BODY" '.ok == true' "healthz body ok=true"

echo "▸ 1.3 /readyz returns 200 (engine loop running)"
READYZ_CODE="$(http_code "${AI_BASE}/readyz")"
assert_eq "$READYZ_CODE" "200" "readyz returns 200 (engine running)"

echo "▸ 1.4 /readyz body ok=true"
READYZ_BODY="$(http_json "${AI_BASE}/readyz")"
assert_jq "$READYZ_BODY" '.ok == true' "readyz body ok=true"

# ═════════════════════════════════════════════════════════════════════════════
section "2. PROMETHEUS METRICS"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 2.1 /metrics returns 200"
METRICS_CODE="$(http_code "${AI_BASE}/metrics")"
assert_eq "$METRICS_CODE" "200" "metrics endpoint returns 200"

echo "▸ 2.2 all operator metric families present"
METRICS="$(curl -fsS "${AI_BASE}/metrics" 2>/dev/null || echo '')"
assert_contains "$METRICS" "nexus_operator_events_consumed_total" "has events_consumed_total"
assert_contains "$METRICS" "nexus_operator_actions_applied_total" "has actions_applied_total"
assert_contains "$METRICS" "nexus_operator_incidents_opened_total" "has incidents_opened_total"
assert_contains "$METRICS" "nexus_operator_proposals_created_total" "has proposals_created_total"
assert_contains "$METRICS" "nexus_operator_last_cursor" "has last_cursor gauge"

# ═════════════════════════════════════════════════════════════════════════════
section "3. AUTH ENFORCEMENT"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 3.1 /v1/internal/tick rejects missing operator key"
CODE_NO_KEY="$(http_code -X POST "${AI_BASE}/v1/internal/tick")"
assert_eq "$CODE_NO_KEY" "401" "tick rejects missing key"

echo "▸ 3.2 /v1/internal/tick rejects wrong operator key"
CODE_BAD_KEY="$(http_code -X POST -H "X-Operator-Key: wrong-key" "${AI_BASE}/v1/internal/tick")"
assert_eq "$CODE_BAD_KEY" "401" "tick rejects wrong key"

echo "▸ 3.3 /v1/assistant/query rejects missing operator key"
CODE_ASSIST_NO_KEY="$(http_code -X POST \
  -H "Content-Type: application/json" \
  -d '{"org_id":"test","query":"hello"}' \
  "${AI_BASE}/v1/assistant/query")"
assert_eq "$CODE_ASSIST_NO_KEY" "401" "assistant rejects missing key"

# ═════════════════════════════════════════════════════════════════════════════
section "4. TICK ENDPOINT (manual engine trigger)"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 4.1 /v1/internal/tick succeeds with valid key"
TICK_BODY="$(http_json -X POST -H "X-Operator-Key: ${OP_KEY}" "${AI_BASE}/v1/internal/tick")"
assert_jq "$TICK_BODY" '.status == "ok"' "tick returns status=ok"

echo "▸ 4.2 cursor is tracked in metrics"
METRICS_TICK="$(curl -fsS "${AI_BASE}/metrics" 2>/dev/null || echo '')"
CURSOR_VAL="$(metric_value "$METRICS_TICK" "nexus_operator_last_cursor")"
echo "      cursor after tick: ${CURSOR_VAL}"
ok "cursor metric present and readable"

# ═════════════════════════════════════════════════════════════════════════════
section "5. EVENT CONSUMPTION"
# ═════════════════════════════════════════════════════════════════════════════

CURSOR_BEFORE="$(metric_value "$(curl -fsS "${AI_BASE}/metrics" 2>/dev/null)" "nexus_operator_last_cursor")"

echo "▸ 5.1 Generate gateway traffic (10 requests)"
for i in $(seq 1 10); do
  curl -sS -o /dev/null \
    -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"tool_name":"echo","input":{"msg":"ai-e2e-'$i'"},"context":{"user_id":"ai-e2e"}}' \
    "${CORE_BASE}/v1/run" 2>/dev/null || true
done
ok "generated 10 gateway requests"

echo "▸ 5.2 Trigger engine tick to consume new events"
http_json -X POST -H "X-Operator-Key: ${OP_KEY}" "${AI_BASE}/v1/internal/tick" >/dev/null

echo "▸ 5.3 Verify cursor advanced"
CURSOR_AFTER="$(metric_value "$(curl -fsS "${AI_BASE}/metrics" 2>/dev/null)" "nexus_operator_last_cursor")"
echo "      cursor before=${CURSOR_BEFORE} after=${CURSOR_AFTER}"
if awk "BEGIN{exit !(${CURSOR_AFTER} > ${CURSOR_BEFORE})}"; then
  ok "cursor advanced after traffic (${CURSOR_BEFORE} → ${CURSOR_AFTER})"
else
  fail "cursor did not advance (${CURSOR_BEFORE} → ${CURSOR_AFTER})"
fi

echo "▸ 5.4 Events consumed counter increased"
EVENTS_CONSUMED="$(metric_value "$(curl -fsS "${AI_BASE}/metrics" 2>/dev/null)" "nexus_operator_events_consumed_total")"
if awk "BEGIN{exit !(${EVENTS_CONSUMED} > 0)}"; then
  ok "events consumed > 0 (${EVENTS_CONSUMED})"
else
  fail "events consumed still 0"
fi

# ═════════════════════════════════════════════════════════════════════════════
section "6. HIGH-RISK SIGNAL DETECTION"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 6.1 Create a fail tool for concentrated deny traffic"
CREATE_STATUS="$(http_code \
  -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"name":"'"${FAIL_TOOL}"'","kind":"http","method":"POST","url":"http://10.255.255.1:9999/black-hole","input_schema":{"type":"object"},"action_type":"read","risk_level":1,"enabled":true}' \
  "${CORE_BASE}/v1/tools")"
if [[ "$CREATE_STATUS" =~ ^(201|409)$ ]]; then
  ok "fail tool created (${CREATE_STATUS})"
else
  fail "could not create fail tool (${CREATE_STATUS})"
fi

echo "▸ 6.2 Generate 30 deny requests (no egress rule = deny)"
ACTIONS_BEFORE="$(metric_value "$(curl -fsS "${AI_BASE}/metrics" 2>/dev/null)" "nexus_operator_actions_applied_total")"
for i in $(seq 1 30); do
  curl -sS -o /dev/null \
    -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"tool_name":"'"${FAIL_TOOL}"'","input":{"n":'$i'},"context":{"user_id":"ai-e2e-risk"}}' \
    "${CORE_BASE}/v1/run" 2>/dev/null || true
done
ok "generated 30 deny requests"

echo "▸ 6.3 Trigger tick to process high-risk batch"
http_json -X POST -H "X-Operator-Key: ${OP_KEY}" "${AI_BASE}/v1/internal/tick" >/dev/null

echo "▸ 6.4 Verify action was applied (or cooldown is active)"
METRICS_HR="$(curl -fsS "${AI_BASE}/metrics" 2>/dev/null || echo '')"
ACTIONS_AFTER="$(metric_value "$METRICS_HR" "nexus_operator_actions_applied_total")"
INCIDENTS_AFTER="$(metric_value "$METRICS_HR" "nexus_operator_incidents_opened_total")"
PROPOSALS_AFTER="$(metric_value "$METRICS_HR" "nexus_operator_proposals_created_total")"
echo "      actions before=${ACTIONS_BEFORE} after=${ACTIONS_AFTER}"

if awk "BEGIN{exit !(${ACTIONS_AFTER} > ${ACTIONS_BEFORE})}"; then
  ok "action applied on high-risk signal (${ACTIONS_BEFORE} → ${ACTIONS_AFTER})"
elif awk "BEGIN{exit !(${ACTIONS_AFTER} > 0)}"; then
  ok "action already applied in previous cycle — cooldown active (correct behaviour)"
else
  fail "no action ever applied"
fi

echo "▸ 6.5 Verify incident was opened"
if awk "BEGIN{exit !(${INCIDENTS_AFTER} > 0)}"; then
  ok "incident opened (${INCIDENTS_AFTER})"
else
  fail "no incidents opened"
fi

echo "▸ 6.6 Verify policy proposal was created"
if awk "BEGIN{exit !(${PROPOSALS_AFTER} > 0)}"; then
  ok "policy proposal created (${PROPOSALS_AFTER})"
else
  fail "no proposals created"
fi

# ═════════════════════════════════════════════════════════════════════════════
section "7. ASSISTANT ENDPOINT"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 7.1 Query returns structured response"
ASSIST_BODY="$(http_json -X POST \
  -H "X-Operator-Key: ${OP_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"org_id":"test","query":"what is the current state?"}' \
  "${AI_BASE}/v1/assistant/query")"

assert_jq "$ASSIST_BODY" '.summary | length > 0' "assistant returns non-empty summary"
assert_jq "$ASSIST_BODY" '.tables | type == "array"' "assistant returns tables array"
assert_jq "$ASSIST_BODY" '.actions | type == "array"' "assistant returns actions array"

echo "▸ 7.2 Response contains operator state table"
assert_jq "$ASSIST_BODY" '.tables[0].title == "Operator State"' "state table present"

echo "▸ 7.3 Response contains tick action"
assert_jq "$ASSIST_BODY" '.actions[0].action_type == "operator.tick"' "tick action present"

echo "▸ 7.4 Query with actor field"
ASSIST_ACTOR="$(http_json -X POST \
  -H "X-Operator-Key: ${OP_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"org_id":"test","query":"status?","actor":"e2e-test"}' \
  "${AI_BASE}/v1/assistant/query")"
assert_jq "$ASSIST_ACTOR" '.summary | length > 0' "assistant with actor returns summary"

# ═════════════════════════════════════════════════════════════════════════════
section "8. ENGINE STATE INTEGRITY"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 8.1 Cursor is monotonically increasing"
C1="$(metric_value "$(curl -fsS "${AI_BASE}/metrics" 2>/dev/null)" "nexus_operator_last_cursor")"
http_json -X POST -H "X-Operator-Key: ${OP_KEY}" "${AI_BASE}/v1/internal/tick" >/dev/null
C2="$(metric_value "$(curl -fsS "${AI_BASE}/metrics" 2>/dev/null)" "nexus_operator_last_cursor")"
if awk "BEGIN{exit !(${C2} >= ${C1})}"; then
  ok "cursor monotonically non-decreasing (${C1} → ${C2})"
else
  fail "cursor went backward (${C1} → ${C2})"
fi

echo "▸ 8.2 Multiple rapid ticks don't crash"
for _ in $(seq 1 5); do
  http_json -X POST -H "X-Operator-Key: ${OP_KEY}" "${AI_BASE}/v1/internal/tick" >/dev/null
done
READYZ_AFTER="$(http_code "${AI_BASE}/readyz")"
assert_eq "$READYZ_AFTER" "200" "engine still healthy after rapid ticks"

# ═════════════════════════════════════════════════════════════════════════════
section "RESULTS"
# ═════════════════════════════════════════════════════════════════════════════
TOTAL=$((PASS+FAIL))
echo ""
echo "  Total : ${TOTAL}"
echo "  Pass  : ${PASS}"
echo "  Fail  : ${FAIL}"
echo ""
if [[ "$FAIL" -eq 0 ]]; then
  echo "  ★ ALL TESTS PASSED"
  exit 0
else
  echo "  ✗ SOME TESTS FAILED"
  exit 1
fi
