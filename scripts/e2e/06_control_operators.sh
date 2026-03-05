#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
NAME
    06_control_operators.sh — e2e for nexus-control-operators service

SYNOPSIS
    06_control_operators.sh [-h|--help]

DESCRIPTION
    Validates the nexus-control-operators service against a running stack:

      1. Health endpoints (/healthz, /readyz)
      2. Prometheus metrics (/metrics) — all 4 nexus_operators_* families
      3. Event consumption — generates traffic, verifies offsets advance
      4. Core proxy HTTP metrics — verifies core request tracking
      5. Persistence — offset, sentry state files in container
      6. Anomaly detection — generates error traffic, checks sentry baselines

    Reports pass/fail counts at the end.

ENVIRONMENT
    NEXUS_HTTP_PORT       Core HTTP port              (default: 8080)
    OPERATOR_HEALTH_PORT  Operators health port       (default: 8090)
    NEXUS_DEMO_API_KEY    API key from seed           (default: nexus-core-local-key)

PREREQUISITES
    Full stack running (docker compose up) including nexus-control-operators.

EXAMPLES
    ./scripts/e2e/06_control_operators.sh
    OPERATOR_HEALTH_PORT=9090 ./scripts/e2e/06_control_operators.sh
EOF
  exit 0
}
[[ "${1:-}" =~ ^(-h|--help)$ ]] && usage

CORE_PORT="${NEXUS_HTTP_PORT:-8080}"
OPS_PORT="${OPERATOR_HEALTH_PORT:-8090}"
API_KEY="${NEXUS_DEMO_API_KEY:-nexus-core-local-key}"

CORE_BASE="http://localhost:${CORE_PORT}"
OPS_BASE="http://localhost:${OPS_PORT}"

PASS=0
FAIL=0

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
  curl -sS -o /dev/null -w "%{http_code}" "$1" 2>/dev/null || echo "000"
}

section() {
  echo ""
  echo "═══════════════════════════════════════════════════════════"
  echo " $1"
  echo "═══════════════════════════════════════════════════════════"
}

# Resolve the actual ops port: if running inside docker, use docker compose port
resolve_ops_base() {
  if [[ "$(http_code "${OPS_BASE}/healthz")" == "200" ]]; then
    return
  fi
  # Try finding the mapped port via docker
  local mapped
  mapped="$(docker compose port nexus-control-operators 8090 2>/dev/null | cut -d: -f2)" || true
  if [[ -n "$mapped" ]]; then
    OPS_BASE="http://localhost:${mapped}"
  fi
}

resolve_ops_base

# ═════════════════════════════════════════════════════════════════════════════
section "1. HEALTH ENDPOINTS"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 1.1 /healthz returns 200"
HEALTHZ_CODE="$(http_code "${OPS_BASE}/healthz")"
assert_eq "$HEALTHZ_CODE" "200" "healthz returns 200"

echo "▸ 1.2 /healthz body is valid JSON"
HEALTHZ_BODY="$(curl -fsS "${OPS_BASE}/healthz" 2>/dev/null || echo '{}')"
assert_jq "$HEALTHZ_BODY" '.ok == true' "healthz body ok=true"

echo "▸ 1.3 /readyz returns 200 (nexus-core reachable)"
READYZ_CODE="$(http_code "${OPS_BASE}/readyz")"
assert_eq "$READYZ_CODE" "200" "readyz returns 200 (core is up)"

echo "▸ 1.4 /readyz body is valid JSON"
READYZ_BODY="$(curl -fsS "${OPS_BASE}/readyz" 2>/dev/null || echo '{}')"
assert_jq "$READYZ_BODY" '.ok == true' "readyz body ok=true"

# ═════════════════════════════════════════════════════════════════════════════
section "2. PROMETHEUS METRICS (endpoint)"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 2.1 /metrics returns 200"
METRICS_CODE="$(http_code "${OPS_BASE}/metrics")"
assert_eq "$METRICS_CODE" "200" "metrics endpoint returns 200"

echo "▸ 2.2 /metrics contains core_requests (always registered)"
METRICS="$(curl -fsS "${OPS_BASE}/metrics" 2>/dev/null || echo '')"
assert_contains "$METRICS" "nexus_operators_core_requests_total" "metrics has core_requests_total"

# ═════════════════════════════════════════════════════════════════════════════
section "3. EVENT CONSUMPTION"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 3.1 Generate gateway traffic (5 successful requests)"
for i in $(seq 1 5); do
  curl -sS -o /dev/null \
    -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"tool_name":"echo","input":{"msg":"ops-e2e-'$i'"},"context":{"user_id":"e2e"}}' \
    "${CORE_BASE}/v1/run" 2>/dev/null || true
done
ok "generated 5 successful requests"

echo "▸ 3.2 Wait for operators to consume events"
sleep 3

echo "▸ 3.3 Verify consumer offsets are advancing"
METRICS_AFTER="$(curl -fsS "${OPS_BASE}/metrics" 2>/dev/null || echo '')"
SENTRY_OFFSET="$(echo "$METRICS_AFTER" | grep 'nexus_operators_consumer_offset{consumer_group="agents.sentry.v1"}' | awk '{print $2}' || echo '0')"
COORD_OFFSET="$(echo "$METRICS_AFTER" | grep 'nexus_operators_consumer_offset{consumer_group="agents.coordinator.v1"}' | awk '{print $2}' || echo '0')"

if [[ -n "$SENTRY_OFFSET" ]] && (( $(echo "$SENTRY_OFFSET > 0" | bc -l 2>/dev/null || echo 0) )); then
  ok "sentry consumer offset > 0 (${SENTRY_OFFSET})"
else
  fail "sentry consumer offset not advancing (${SENTRY_OFFSET})"
fi

if [[ -n "$COORD_OFFSET" ]] && (( $(echo "$COORD_OFFSET > 0" | bc -l 2>/dev/null || echo 0) )); then
  ok "coordinator consumer offset > 0 (${COORD_OFFSET})"
else
  fail "coordinator consumer offset not advancing (${COORD_OFFSET})"
fi

echo "▸ 3.4 Verify events have been processed"
EVENTS_OK="$(echo "$METRICS_AFTER" | grep 'nexus_operators_events_processed_total.*status="ok"' | head -1 | awk '{print $2}' || echo '0')"
if [[ -n "$EVENTS_OK" ]] && (( $(echo "$EVENTS_OK > 0" | bc -l 2>/dev/null || echo 0) )); then
  ok "events processed (ok) > 0 (${EVENTS_OK})"
else
  fail "no events processed yet (${EVENTS_OK})"
fi

echo "▸ 3.5 Verify labeled metrics appeared after processing"
assert_contains "$METRICS_AFTER" "nexus_operators_events_processed_total" "metrics has events_processed_total"
assert_contains "$METRICS_AFTER" "nexus_operators_event_processing_duration_seconds" "metrics has event_processing_duration"
assert_contains "$METRICS_AFTER" "nexus_operators_consumer_offset" "metrics has consumer_offset"

# ═════════════════════════════════════════════════════════════════════════════
section "4. CORE PROXY HTTP METRICS"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 4.1 Verify core HTTP requests are tracked"
CORE_REQS="$(echo "$METRICS_AFTER" | grep 'nexus_operators_core_requests_total.*status="200"' | head -1 | awk '{print $2}' || echo '0')"
if [[ -n "$CORE_REQS" ]] && (( $(echo "$CORE_REQS > 0" | bc -l 2>/dev/null || echo 0) )); then
  ok "core HTTP requests tracked (${CORE_REQS})"
else
  fail "no core HTTP requests tracked"
fi

# ═════════════════════════════════════════════════════════════════════════════
section "5. PERSISTENCE"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 5.1 Verify offset file exists in container"
OFFSET_EXISTS="$(docker compose exec -T nexus-control-operators test -f /app/data/offsets.json && echo "yes" || echo "no")"
assert_eq "$OFFSET_EXISTS" "yes" "offsets.json exists in /app/data"

echo "▸ 5.2 Verify offset file has content"
OFFSET_CONTENT="$(docker compose exec -T nexus-control-operators cat /app/data/offsets.json 2>/dev/null || echo '{}')"
assert_jq "$OFFSET_CONTENT" 'keys | length > 0' "offsets.json has consumer groups"

echo "▸ 5.3 Verify sentry state file exists"
SENTRY_EXISTS="$(docker compose exec -T nexus-control-operators test -f /app/data/sentry_state.json && echo "yes" || echo "no")"
assert_eq "$SENTRY_EXISTS" "yes" "sentry_state.json exists"

# ═════════════════════════════════════════════════════════════════════════════
section "6. ANOMALY DETECTION (sentry)"
# ═════════════════════════════════════════════════════════════════════════════

echo "▸ 6.1 Generate error traffic to trigger sentry"
echo "      (sending requests to a tool that will fail)"

FAIL_TOOL_CREATED="no"
CREATE_STATUS="$(curl -sS -o /dev/null -w "%{http_code}" \
  -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"name":"ops-e2e-fail","kind":"http","method":"POST","url":"http://mock-tools:8081/error","input_schema":{"type":"object"},"action_type":"read","risk_level":1,"enabled":true}' \
  "${CORE_BASE}/v1/tools" 2>/dev/null || echo "000")"
if [[ "$CREATE_STATUS" =~ ^(201|409)$ ]]; then
  FAIL_TOOL_CREATED="yes"
  curl -sS -o /dev/null \
    -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"host":"mock-tools","enabled":true}' \
    "${CORE_BASE}/v1/tools/ops-e2e-fail/egress-rules" 2>/dev/null || true
fi

if [[ "$FAIL_TOOL_CREATED" == "yes" ]]; then
  for i in $(seq 1 10); do
    curl -sS -o /dev/null \
      -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
      -H "Content-Type: application/json" \
      -d '{"tool_name":"ops-e2e-fail","input":{"trigger":"error"},"context":{"user_id":"e2e"}}' \
      "${CORE_BASE}/v1/run" 2>/dev/null || true
  done
  ok "generated 10 error requests for ops-e2e-fail"

  echo "▸ 6.2 Wait for sentry to detect anomaly"
  sleep 5

  echo "▸ 6.3 Check sentry state for new baselines"
  SENTRY_STATE="$(docker compose exec -T nexus-control-operators cat /app/data/sentry_state.json 2>/dev/null || echo '{}')"
  BASELINE_COUNT="$(echo "$SENTRY_STATE" | jq '.baselines | length' 2>/dev/null || echo '0')"
  if [[ "$BASELINE_COUNT" -gt 0 ]]; then
    ok "sentry baselines populated (${BASELINE_COUNT} entries)"
  else
    fail "sentry baselines empty after error traffic"
  fi
else
  fail "could not create fail tool for anomaly test"
fi

# Cleanup fail tool
if [[ "$FAIL_TOOL_CREATED" == "yes" ]]; then
  curl -sS -o /dev/null -X DELETE \
    -H "X-NEXUS-CORE-KEY: ${API_KEY}" \
    "${CORE_BASE}/v1/tools/ops-e2e-fail" 2>/dev/null || true
fi

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
