#!/usr/bin/env bash
# =============================================================================
# Full E2E test for nexus-core — exercises every feature against the live stack.
#
# Prerequisites: docker compose up (stack running)
# Usage: ./scripts/e2e/03_full_core_e2e.sh
# =============================================================================
set -euo pipefail

HTTP_PORT="${NEXUS_HTTP_PORT:-8080}"
API_KEY="${NEXUS_DEMO_API_KEY:-nexus-core-local-key}"
BASE="http://localhost:${HTTP_PORT}/v1"
ALL_SCOPES="tools:read,tools:write,policy:read,policy:write,egress:read,egress:write,audit:read,gateway:run,gateway:simulate,admin:secrets,admin:console:read,admin:console:write"

PASS=0
FAIL=0
TOOL_NAME="e2e-test-tool-$$"

AUTH_HEADERS=(
  -H "X-NEXUS-CORE-KEY: ${API_KEY}"
  -H "X-NEXUS-SCOPES: ${ALL_SCOPES}"
  -H "X-NEXUS-ACTOR: e2e/script"
  -H "Content-Type: application/json"
)

# ── Helpers ──────────────────────────────────────────────────────────────────

fail() { echo "  ✗ $*" >&2; FAIL=$((FAIL+1)); }
ok()   { echo "  ✓ $*"; PASS=$((PASS+1)); }

api() {
  local method="$1" path="$2"; shift 2
  curl -sS -X "$method" "${AUTH_HEADERS[@]}" "$@" "${BASE}${path}"
}

api_status() {
  local method="$1" path="$2"; shift 2
  curl -sS -o /dev/null -w "%{http_code}" -X "$method" "${AUTH_HEADERS[@]}" "$@" "${BASE}${path}"
}

assert_eq() {
  local actual="$1" expected="$2" label="$3"
  if [[ "$actual" == "$expected" ]]; then ok "$label"; else fail "$label: expected '$expected', got '$actual'"; fi
}

assert_jq() {
  local json="$1" filter="$2" label="${3:-$2}"
  if echo "$json" | jq -e "$filter" >/dev/null 2>&1; then ok "$label"; else fail "$label (filter: $filter)"; fi
}

assert_contains() {
  local haystack="$1" needle="$2" label="$3"
  if echo "$haystack" | grep -q "$needle"; then ok "$label"; else fail "$label: '$needle' not found"; fi
}

section() { echo ""; echo "═══════════════════════════════════════════════════════════"; echo " $1"; echo "═══════════════════════════════════════════════════════════"; }

cleanup() {
  echo ""
  echo "▸ Cleanup..."
  api DELETE "/tools/${TOOL_NAME}" 2>/dev/null || true
  echo "  done."
}
trap cleanup EXIT

# ═════════════════════════════════════════════════════════════════════════════
section "1. TOOLS CRUD"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 1.1 Create tool"
CREATE_RESP=$(api POST "/tools" -d "{
  \"name\": \"${TOOL_NAME}\",
  \"kind\": \"http\",
  \"method\": \"POST\",
  \"url\": \"http://mock-tools:8081/echo\",
  \"action_type\": \"read\",
  \"input_schema\": {\"type\":\"object\",\"properties\":{\"msg\":{\"type\":\"string\"}},\"required\":[\"msg\"]},
  \"classification\": \"internal\",
  \"sensitivity\": \"low\",
  \"risk_level\": 1,
  \"enabled\": true
}")
assert_jq "$CREATE_RESP" ".name == \"${TOOL_NAME}\"" "tool created with correct name"
assert_jq "$CREATE_RESP" '.enabled == true' "tool created enabled"
assert_jq "$CREATE_RESP" '.id | length > 0' "tool has UUID"
TOOL_ID=$(echo "$CREATE_RESP" | jq -r '.id')
echo "  → id=${TOOL_ID}"

echo "▸ 1.2 List tools"
LIST_RESP=$(api GET "/tools")
assert_jq "$LIST_RESP" ".items | map(select(.name == \"${TOOL_NAME}\")) | length == 1" "tool appears in list"

echo "▸ 1.3 Get tool by name"
GET_NAME_RESP=$(api GET "/tools/${TOOL_NAME}")
assert_jq "$GET_NAME_RESP" ".id == \"${TOOL_ID}\"" "get by name returns correct id"

echo "▸ 1.4 Get tool by UUID"
GET_ID_RESP=$(api GET "/tools/${TOOL_ID}")
assert_jq "$GET_ID_RESP" ".name == \"${TOOL_NAME}\"" "get by UUID returns correct name"

echo "▸ 1.5 Update tool"
UPDATE_RESP=$(api PUT "/tools/${TOOL_NAME}" -d '{"description":"updated by e2e","sensitivity":"medium"}')
assert_jq "$UPDATE_RESP" '.sensitivity == "medium"' "sensitivity updated"
assert_jq "$UPDATE_RESP" '.description == "updated by e2e"' "description updated"

echo "▸ 1.6 Update by UUID"
UPDATE2_RESP=$(api PUT "/tools/${TOOL_ID}" -d '{"risk_level":3}')
assert_jq "$UPDATE2_RESP" '.risk_level == 3' "update by UUID works"

echo "▸ 1.7 Name immutability"
IMMUT_STATUS=$(api_status PUT "/tools/${TOOL_NAME}" -d '{"name":"hacked"}')
assert_eq "$IMMUT_STATUS" "400" "reject name change returns 400"

echo "▸ 1.8 Duplicate name"
DUP_STATUS=$(api_status POST "/tools" -d "{
  \"name\": \"${TOOL_NAME}\",\"kind\":\"http\",\"method\":\"POST\",\"url\":\"http://x\",
  \"action_type\":\"read\",\"input_schema\":{\"type\":\"object\"},\"risk_level\":1,\"enabled\":true
}")
assert_eq "$DUP_STATUS" "409" "duplicate name returns 409"

# ═════════════════════════════════════════════════════════════════════════════
section "2. EGRESS RULES"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 2.1 Create egress rule"
EGRESS_STATUS=$(api_status POST "/tools/${TOOL_NAME}/egress-rules" -d '{"host":"mock-tools"}')
assert_eq "$EGRESS_STATUS" "204" "egress rule created"

echo "▸ 2.2 List egress rules"
EGRESS_LIST=$(api GET "/tools/${TOOL_NAME}/egress-rules")
assert_contains "$EGRESS_LIST" "mock-tools" "egress list contains mock-tools"

echo "▸ 2.3 Run WITHOUT egress (should fail)"
api DELETE "/tools/${TOOL_NAME}/egress-rules?host=mock-tools" >/dev/null 2>&1 || true
NO_EGRESS_RESP=$(api POST "/run" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"test\"}}")
assert_jq "$NO_EGRESS_RESP" '.status == "blocked"' "blocked without egress rule"
assert_contains "$NO_EGRESS_RESP" "egress" "error mentions egress"

echo "▸ 2.4 Re-add egress rule"
api POST "/tools/${TOOL_NAME}/egress-rules" -d '{"host":"mock-tools"}' >/dev/null 2>&1

# ═════════════════════════════════════════════════════════════════════════════
section "3. GATEWAY — RUN"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 3.1 Run by tool_name"
RUN_NAME=$(api POST "/run" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"by-name\"}}")
assert_jq "$RUN_NAME" '.status == "success"' "run by name: success"
assert_jq "$RUN_NAME" ".tool_name == \"${TOOL_NAME}\"" "run by name: correct tool"
assert_jq "$RUN_NAME" '.result.received.msg == "by-name"' "run by name: upstream echoed input"
assert_jq "$RUN_NAME" '.latency_ms >= 0' "run by name: latency present"

echo "▸ 3.2 Run by tool_id"
RUN_ID=$(api POST "/run" -d "{\"tool_id\":\"${TOOL_ID}\",\"input\":{\"msg\":\"by-id\"}}")
assert_jq "$RUN_ID" '.status == "success"' "run by id: success"
assert_jq "$RUN_ID" ".tool_name == \"${TOOL_NAME}\"" "run by id: resolved name"

echo "▸ 3.3 Run with both (tool_id wins)"
RUN_BOTH=$(api POST "/run" -d "{\"tool_id\":\"${TOOL_ID}\",\"tool_name\":\"wrong\",\"input\":{\"msg\":\"both\"}}")
assert_jq "$RUN_BOTH" '.status == "success"' "run with both: tool_id takes precedence"

echo "▸ 3.4 Run with invalid tool_id"
RUN_BAD_ID=$(api POST "/run" -d '{"tool_id":"not-a-uuid","input":{"msg":"x"}}')
assert_contains "$RUN_BAD_ID" "not a valid UUID" "invalid tool_id rejected"

echo "▸ 3.5 Run with neither"
RUN_NONE=$(api POST "/run" -d '{"input":{"msg":"x"}}')
assert_contains "$RUN_NONE" "tool_name or tool_id required" "missing both rejected"

echo "▸ 3.6 Run with nonexistent tool"
RUN_404=$(api POST "/run" -d '{"tool_name":"nonexistent-xyz","input":{"msg":"x"}}')
assert_jq "$RUN_404" '.status == "blocked"' "nonexistent tool: blocked"

# ═════════════════════════════════════════════════════════════════════════════
section "4. SCHEMA VALIDATION"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 4.1 Valid input"
SCHEMA_OK=$(api POST "/run" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"valid\"}}")
assert_jq "$SCHEMA_OK" '.status == "success"' "valid input passes schema"

echo "▸ 4.2 Invalid input (missing required field)"
SCHEMA_FAIL=$(api POST "/run" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"foo\":123}}")
assert_jq "$SCHEMA_FAIL" '.status == "blocked"' "invalid input blocked"
assert_contains "$SCHEMA_FAIL" "schema" "error mentions schema"

# ═════════════════════════════════════════════════════════════════════════════
section "5. TOOL DISABLED"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 5.1 Disable tool"
api PUT "/tools/${TOOL_NAME}" -d '{"enabled":false}' >/dev/null

echo "▸ 5.2 Run disabled tool"
RUN_DISABLED=$(api POST "/run" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"disabled\"}}")
assert_jq "$RUN_DISABLED" '.status == "blocked"' "disabled tool is blocked"
assert_contains "$RUN_DISABLED" "disabled" "error mentions disabled"

echo "▸ 5.3 Re-enable tool"
api PUT "/tools/${TOOL_NAME}" -d '{"enabled":true}' >/dev/null
RUN_REENABLED=$(api POST "/run" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"re-enabled\"}}")
assert_jq "$RUN_REENABLED" '.status == "success"' "re-enabled tool works"

# ═════════════════════════════════════════════════════════════════════════════
section "6. POLICIES"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 6.1 Create deny policy"
POLICY_RESP=$(api POST "/tools/${TOOL_NAME}/policies" -d '{
  "effect": "deny",
  "priority": 1,
  "conditions": {},
  "limits": {},
  "reason_template": "e2e deny policy",
  "enabled": true
}')
assert_jq "$POLICY_RESP" '.effect == "deny"' "deny policy created"
POLICY_ID=$(echo "$POLICY_RESP" | jq -r '.id')

echo "▸ 6.2 List policies"
POLICY_LIST=$(api GET "/tools/${TOOL_NAME}/policies")
assert_jq "$POLICY_LIST" ".items | length >= 1" "policy list has items"

echo "▸ 6.3 Run with deny policy"
RUN_DENIED=$(api POST "/run" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"denied\"}}")
assert_jq "$RUN_DENIED" '.decision == "deny"' "request denied by policy"
assert_contains "$RUN_DENIED" "e2e deny policy" "deny reason matches"

echo "▸ 6.4 Update policy (disable)"
api PUT "/policies/${POLICY_ID}" -d '{"enabled":false}' >/dev/null
RUN_AFTER_DISABLE=$(api POST "/run" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"after-disable\"}}")
assert_jq "$RUN_AFTER_DISABLE" '.status == "success"' "runs after policy disabled"

echo "▸ 6.5 Update policy (re-enable + change to allow)"
api PUT "/policies/${POLICY_ID}" -d '{"enabled":true,"effect":"allow","reason_template":"e2e allow policy"}' >/dev/null
RUN_ALLOW=$(api POST "/run" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"allowed\"}}")
assert_jq "$RUN_ALLOW" '.decision == "allow"' "allow policy works"
assert_jq "$RUN_ALLOW" '.status == "success"' "request succeeds with allow policy"

# ═════════════════════════════════════════════════════════════════════════════
section "7. SECRETS"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 7.1 Create secret (bearer token)"
SECRET_RESP=$(api POST "/tools/${TOOL_NAME}/secrets" -d '{
  "secret_type": "bearer",
  "key_name": "Authorization",
  "value": "Bearer e2e-test-token-123"
}')
assert_jq "$SECRET_RESP" '.secret_type == "bearer"' "secret created"
assert_jq "$SECRET_RESP" '.key_name == "Authorization"' "secret key_name correct"

echo "▸ 7.2 List secrets"
SECRET_LIST=$(api GET "/tools/${TOOL_NAME}/secrets")
assert_jq "$SECRET_LIST" '.items | length >= 1' "secrets list has items"

echo "▸ 7.3 Run with secret injection"
RUN_SECRET=$(api POST "/run" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"with-secret\"}}")
assert_jq "$RUN_SECRET" '.status == "success"' "run with secret succeeds"
assert_jq "$RUN_SECRET" '.result.auth_present == true' "upstream received auth header"

echo "▸ 7.4 Delete secret"
DEL_SECRET=$(api_status DELETE "/tools/${TOOL_NAME}/secrets?key_name=Authorization")
assert_eq "$DEL_SECRET" "204" "secret deleted"

echo "▸ 7.5 Run after secret deleted"
RUN_NO_SECRET=$(api POST "/run" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"no-secret\"}}")
assert_jq "$RUN_NO_SECRET" '.result.auth_present == false' "upstream no longer receives auth"

# ═════════════════════════════════════════════════════════════════════════════
section "8. SIMULATE (dry run)"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 8.1 Simulate existing tool"
SIM_RESP=$(api POST "/run/simulate" -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"sim\"}}")
assert_jq "$SIM_RESP" '.decision == "allow"' "simulate: allow"
assert_jq "$SIM_RESP" '.explain | length > 0' "simulate: has explain"
assert_jq "$SIM_RESP" '.explain.would_execute == true' "simulate: would execute"

echo "▸ 8.2 Simulate nonexistent tool"
SIM_404=$(api POST "/run/simulate" -d '{"tool_name":"no-such-tool","input":{"msg":"x"}}')
assert_jq "$SIM_404" '.decision == "deny"' "simulate nonexistent: deny"

# ═════════════════════════════════════════════════════════════════════════════
section "9. IDEMPOTENCY (write tool)"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 9.0 Switch tool to write"
api PUT "/tools/${TOOL_NAME}" -d '{"action_type":"write"}' >/dev/null

IDEM_KEY="e2e-idem-$$"

echo "▸ 9.1 First call (NEW)"
IDEM1=$(curl -sS -D - -X POST "${AUTH_HEADERS[@]}" \
  -H "Idempotency-Key: ${IDEM_KEY}" \
  -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"idem\"}}" \
  "${BASE}/run" 2>&1)
IDEM1_OUTCOME=$(echo "$IDEM1" | grep -i "X-Idempotency-Outcome" | tr -d '\r' | awk '{print $2}')
assert_eq "$IDEM1_OUTCOME" "NEW" "idempotency: NEW"

echo "▸ 9.2 Same key, same payload (REPLAY)"
IDEM2=$(curl -sS -D - -X POST "${AUTH_HEADERS[@]}" \
  -H "Idempotency-Key: ${IDEM_KEY}" \
  -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"idem\"}}" \
  "${BASE}/run" 2>&1)
IDEM2_OUTCOME=$(echo "$IDEM2" | grep -i "X-Idempotency-Outcome" | tr -d '\r' | awk '{print $2}')
assert_eq "$IDEM2_OUTCOME" "REPLAY" "idempotency: REPLAY"

echo "▸ 9.3 Same key, different payload (CONFLICT)"
IDEM3=$(curl -sS -D - -X POST "${AUTH_HEADERS[@]}" \
  -H "Idempotency-Key: ${IDEM_KEY}" \
  -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"different\"}}" \
  "${BASE}/run" 2>&1)
IDEM3_OUTCOME=$(echo "$IDEM3" | grep -i "X-Idempotency-Outcome" | tr -d '\r' | awk '{print $2}')
assert_eq "$IDEM3_OUTCOME" "CONFLICT" "idempotency: CONFLICT"

echo "▸ 9.4 Restore to read"
api PUT "/tools/${TOOL_NAME}" -d '{"action_type":"read"}' >/dev/null

# ═════════════════════════════════════════════════════════════════════════════
section "10. AUDIT LOG"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 10.1 Query audit events"
AUDIT_RESP=$(api GET "/audit?tool_name=${TOOL_NAME}&limit=5")
assert_jq "$AUDIT_RESP" '.items | length > 0' "audit has events for our tool"
assert_jq "$AUDIT_RESP" '.items[0].request_id | length > 0' "audit events have request_id"
assert_jq "$AUDIT_RESP" '.items[0].event_hash | length > 0' "audit events are hash-chained"

echo "▸ 10.2 Export JSONL"
EXPORT_JSONL=$(api GET "/audit/export?tool_name=${TOOL_NAME}&format=jsonl&limit=3")
EXPORT_LINES=$(echo "$EXPORT_JSONL" | wc -l)
if [[ "$EXPORT_LINES" -ge 1 ]]; then ok "JSONL export has data ($EXPORT_LINES lines)"; else fail "JSONL export empty"; fi

echo "▸ 10.3 Export CSV"
EXPORT_CSV=$(api GET "/audit/export?tool_name=${TOOL_NAME}&format=csv&limit=3")
assert_contains "$EXPORT_CSV" "created_at" "CSV export has header"
assert_contains "$EXPORT_CSV" "${TOOL_NAME}" "CSV export has our tool"

# ═════════════════════════════════════════════════════════════════════════════
section "11. AUTHZ (scope enforcement)"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 11.1 tools:read without scope"
AUTHZ_STATUS=$(curl -sS -o /dev/null -w "%{http_code}" \
  -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "X-NEXUS-SCOPES: gateway:run" \
  "${BASE}/tools")
assert_eq "$AUTHZ_STATUS" "403" "tools list without tools:read scope → 403"

echo "▸ 11.2 gateway:run without scope"
AUTHZ_RUN=$(curl -sS -o /dev/null -w "%{http_code}" \
  -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "X-NEXUS-SCOPES: tools:read" \
  -H "Content-Type: application/json" \
  -d "{\"tool_name\":\"${TOOL_NAME}\",\"input\":{\"msg\":\"x\"}}" \
  "${BASE}/run")
assert_eq "$AUTHZ_RUN" "403" "run without gateway:run scope → 403"

# ═════════════════════════════════════════════════════════════════════════════
section "12. DELETE TOOL"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 12.1 Delete by name"
DEL_STATUS=$(api_status DELETE "/tools/${TOOL_NAME}")
assert_eq "$DEL_STATUS" "204" "tool deleted"

echo "▸ 12.2 Verify deleted"
GET_DELETED=$(api_status GET "/tools/${TOOL_NAME}")
assert_eq "$GET_DELETED" "404" "deleted tool returns 404"

trap - EXIT

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
