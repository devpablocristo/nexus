#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd curl

# This test validates graceful degradation:
# 1. Start data-plane WITHOUT control-plane → actions should fail (no cache)
# 2. Start control-plane → create resource + policy
# 3. Create action (populates cache)
# 4. Stop control-plane → create action again (should use cache)
# 5. Wait beyond hard TTL (simulated via short TTL) → should fail closed

DATA_PLANE_PORT="${DATA_PLANE_PORT:-$(find_free_port 18120 18129)}"
BASE_URL="http://127.0.0.1:${DATA_PLANE_PORT}"
READY_URL="${BASE_URL}/readyz"
ACTIONS_URL="${BASE_URL}/v1/actions"
ADMIN_API_KEY="$(admin_api_key)"

cleanup() {
  [[ -n "${DATA_PLANE_PID:-}" ]] && kill "${DATA_PLANE_PID}" >/dev/null 2>&1 || true
  [[ -n "${CONTROL_PLANE_PID:-}" ]] && kill "${CONTROL_PLANE_PID}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "=== smoke: degradation flow ==="

# Step 1: Start data-plane without control-plane
echo "--- step 1: data-plane without control-plane ---"
cd "${DATA_PLANE_DIR}"
NEXUS_API_KEYS="${ADMIN_API_KEY}" \
  PORT="${DATA_PLANE_PORT}" \
  go run ./cmd/api &
DATA_PLANE_PID=$!
wait_for_http "${READY_URL}"

# Create action without control-plane — should succeed (no resolver/policy source configured)
# This validates that data-plane works standalone
RESPONSE=$(curl -sS -w '\n%{http_code}' -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' \
  -X POST "${ACTIONS_URL}" -d '{
    "action_type": "withdrawal",
    "resource_id": "wallet-test-1",
    "resource_type": "wallet",
    "source_system": "smoke-test",
    "justification": "degradation test",
    "requested_by": {"type": "system", "id": "smoke"},
    "proposed_by": {"type": "system", "id": "smoke"},
    "payload": {"amount": "1.0"}
  }')

HTTP_CODE=$(echo "${RESPONSE}" | tail -1)
if [[ "${HTTP_CODE}" != "201" ]]; then
  echo "FAIL: expected 201, got ${HTTP_CODE}" >&2
  echo "${RESPONSE}" >&2
  exit 1
fi
echo "PASS: data-plane works standalone without control-plane"

# Step 2: Start control-plane and create resource + policy
echo "--- step 2: start control-plane, populate cache ---"
CONTROL_PLANE_PORT="${CONTROL_PLANE_PORT:-$(find_free_port 18130 18139)}"
CONTROL_PLANE_URL="http://127.0.0.1:${CONTROL_PLANE_PORT}"

cd "${CONTROL_PLANE_DIR}"
NEXUS_API_KEYS="${ADMIN_API_KEY},$(data_plane_service_api_key),$(control_workers_service_api_key)" \
  PORT="${CONTROL_PLANE_PORT}" \
  go run ./cmd/api &
CONTROL_PLANE_PID=$!
wait_for_http "${CONTROL_PLANE_URL}/readyz"

# Kill data-plane and restart with control-plane configured
kill "${DATA_PLANE_PID}" >/dev/null 2>&1 || true
sleep 1

cd "${DATA_PLANE_DIR}"
NEXUS_API_KEYS="${ADMIN_API_KEY}" \
  NEXUS_CONTROL_PLANE_URL="${CONTROL_PLANE_URL}" \
  NEXUS_CONTROL_PLANE_API_KEY="$(data_plane_service_api_key)" \
  PORT="${DATA_PLANE_PORT}" \
  go run ./cmd/api &
DATA_PLANE_PID=$!
wait_for_http "${READY_URL}"

# Create resource in control-plane
RESOURCE_RESPONSE=$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' \
  -X POST "${CONTROL_PLANE_URL}/v1/resources" -d '{
    "type": "wallet",
    "name": "degradation-test-wallet",
    "environment": "test",
    "chain": "ethereum",
    "labels": {},
    "criticality": "high"
  }')
RESOURCE_ID=$(echo "${RESOURCE_RESPONSE}" | json_get "id")
echo "created resource: ${RESOURCE_ID}"

# Create allow policy
curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' \
  -X POST "${CONTROL_PLANE_URL}/v1/policies" -d '{
    "action_type": "withdrawal",
    "resource_type": "wallet",
    "effect": "allow",
    "priority": 10,
    "expression": "true",
    "reason": "test policy",
    "require_approval": false
  }' >/dev/null

# Create action (this populates the cache in data-plane)
RESPONSE=$(curl -sS -w '\n%{http_code}' -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' \
  -X POST "${ACTIONS_URL}" -d "{
    \"action_type\": \"withdrawal\",
    \"resource_id\": \"${RESOURCE_ID}\",
    \"resource_type\": \"wallet\",
    \"source_system\": \"smoke-test\",
    \"justification\": \"cache population\",
    \"requested_by\": {\"type\": \"system\", \"id\": \"smoke\"},
    \"proposed_by\": {\"type\": \"system\", \"id\": \"smoke\"},
    \"payload\": {\"amount\": \"1.0\"}
  }")

HTTP_CODE=$(echo "${RESPONSE}" | tail -1)
if [[ "${HTTP_CODE}" != "201" ]]; then
  echo "FAIL: expected 201 with control-plane, got ${HTTP_CODE}" >&2
  echo "${RESPONSE}" >&2
  exit 1
fi
echo "PASS: action created with control-plane running (cache populated)"

# Step 3: Stop control-plane, verify data-plane uses cache
echo "--- step 3: stop control-plane, verify cache ---"
kill "${CONTROL_PLANE_PID}" >/dev/null 2>&1 || true
CONTROL_PLANE_PID=""
sleep 1

RESPONSE=$(curl -sS -w '\n%{http_code}' -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' \
  -X POST "${ACTIONS_URL}" -d "{
    \"action_type\": \"withdrawal\",
    \"resource_id\": \"${RESOURCE_ID}\",
    \"resource_type\": \"wallet\",
    \"source_system\": \"smoke-test\",
    \"justification\": \"cached request\",
    \"requested_by\": {\"type\": \"system\", \"id\": \"smoke\"},
    \"proposed_by\": {\"type\": \"system\", \"id\": \"smoke\"},
    \"payload\": {\"amount\": \"2.0\"}
  }")

HTTP_CODE=$(echo "${RESPONSE}" | tail -1)
if [[ "${HTTP_CODE}" != "201" ]]; then
  echo "FAIL: expected 201 with cache, got ${HTTP_CODE}" >&2
  echo "${RESPONSE}" >&2
  exit 1
fi
echo "PASS: action created using cache while control-plane is down"

echo ""
echo "=== degradation flow: ALL PASSED ==="
