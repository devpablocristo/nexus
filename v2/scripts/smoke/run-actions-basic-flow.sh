#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd curl
require_cmd grep

CONTROL_PLANE_PORT="${CONTROL_PLANE_PORT:-$(find_free_port 18100 18109)}"
DATA_PLANE_PORT="${DATA_PLANE_PORT:-$(find_free_port 18110 18119)}"
CONTROL_PLANE_URL="http://127.0.0.1:${CONTROL_PLANE_PORT}"
BASE_URL="http://127.0.0.1:${DATA_PLANE_PORT}"
READY_URL="${BASE_URL}/readyz"
ACTIONS_URL="${BASE_URL}/v1/actions"
ADMIN_API_KEY="$(admin_api_key)"

cleanup() {
  [[ -n "${CONTROL_PLANE_PID:-}" ]] && kill "${CONTROL_PLANE_PID}" >/dev/null 2>&1 || true
  [[ -n "${DATA_PLANE_PID:-}" ]] && kill "${DATA_PLANE_PID}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

create_resource() {
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${CONTROL_PLANE_URL}/v1/resources" -d @- <<'JSON'
{
  "type": "wallet",
  "name": "wallet hot usdc 1",
  "environment": "prod",
  "chain": "ethereum",
  "labels": {"tier": "hot"},
  "criticality": "critical"
}
JSON
}

create_policy() {
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${CONTROL_PLANE_URL}/v1/policies" -d @- <<'JSON'
{
  "action_type": "withdrawal",
  "resource_type": "wallet",
  "effect": "allow",
  "priority": 10,
  "expression": "action.action_type == \"withdrawal\" && resource.environment == \"prod\"",
  "reason": "trusted production wallet withdrawals can auto-approve",
  "require_approval": false
}
JSON
}

create_action() {
  curl -sS -D "$1" -o "$2" -w '%{http_code}' \
    -H "X-API-Key: ${ADMIN_API_KEY}" \
    -H 'Content-Type: application/json' \
    -X POST "${ACTIONS_URL}" \
    -d @- <<JSON
{
  "action_type": "withdrawal",
  "resource_id": "${resource_id}",
  "resource_type": "wallet",
  "source_system": "treasury-orchestrator",
  "justification": "Daily settlement withdrawal",
  "requested_by": {"type": "system", "id": "treasury-bot"},
  "proposed_by": {"type": "agent", "id": "treasury-agent"},
  "payload": {
    "asset": "USDC",
    "amount": "25000.00",
    "network": "ethereum",
    "destination_address": "0x123"
  },
  "metadata": {"ticket_id": "CHG-1234"}
}
JSON
}

PORT="${CONTROL_PLANE_PORT}" "${SCRIPT_DIR}/../dev/run-control-plane.sh" &
CONTROL_PLANE_PID=$!

wait_for_http "${CONTROL_PLANE_URL}/readyz" 80 0.1

resource_body="$(create_resource)"
resource_id="$(printf '%s' "${resource_body}" | json_get "id")"
if [[ -z "${resource_id}" ]]; then
  echo "failed to create resource" >&2
  echo "${resource_body}" >&2
  exit 1
fi

policy_body="$(create_policy)"
policy_id="$(printf '%s' "${policy_body}" | json_get "id")"
if [[ -z "${policy_id}" ]]; then
  echo "failed to create policy" >&2
  echo "${policy_body}" >&2
  exit 1
fi

PORT="${DATA_PLANE_PORT}" NEXUS_CONTROL_PLANE_URL="${CONTROL_PLANE_URL}" "${SCRIPT_DIR}/../dev/run-data-plane.sh" &
DATA_PLANE_PID=$!

wait_for_http "${READY_URL}" 80 0.1

response_headers="$(mktemp)"
response_body="$(mktemp)"
status="$(create_action "${response_headers}" "${response_body}")"
body="$(cat "${response_body}")"
location="$(grep -i '^Location:' "${response_headers}" | awk '{print $2}' | tr -d '\r')"
rm -f "${response_headers}" "${response_body}"

if [[ "${status}" != "201" ]]; then
  echo "unexpected status: ${status}" >&2
  echo "${body}" >&2
  exit 1
fi

action_id="$(printf '%s' "${body}" | json_get "id")"
decision="$(printf '%s' "${body}" | json_get "decision")"
run_status="$(printf '%s' "${body}" | json_get "status")"
risk_level="$(printf '%s' "${body}" | json_get "risk.level")"
approval_status="$(printf '%s' "${body}" | json_get "approval.status")"
evidence_status="$(printf '%s' "${body}" | json_get "evidence_summary.status")"
checks_total="$(printf '%s' "${body}" | json_get "evidence_summary.checks_total")"
risk_score="$(printf '%s' "${body}" | json_get "risk.score")"
risk_profile="$(printf '%s' "${body}" | json_get "risk.profile.name")"
risk_recommended="$(printf '%s' "${body}" | json_get "risk.recommended_decision")"

if [[ -z "${action_id}" ]]; then
  echo "missing action id" >&2
  echo "${body}" >&2
  exit 1
fi

if [[ "${location}" != "/v1/actions/${action_id}" ]]; then
  echo "unexpected location header: ${location}" >&2
  echo "${body}" >&2
  exit 1
fi

if [[ "${decision}" != "allow" || "${run_status}" != "approved" || "${risk_level}" != "medium" || -n "${approval_status}" || "${evidence_status}" != "passed" || "${checks_total}" != "3" || "${risk_score}" != "30" || "${risk_profile}" != "balanced" || "${risk_recommended}" != "enhanced_log" ]]; then
  echo "unexpected response body" >&2
  echo "${body}" >&2
  exit 1
fi

metrics_body="$(fetch_metrics "${BASE_URL}" "${ADMIN_API_KEY}")"
assert_metrics_contains "${metrics_body}" 'nexus_http_requests_total{method="POST",route="/v1/actions",status_code="201"} 1'
assert_metrics_contains "${metrics_body}" 'nexus_actions_total{action_type="withdrawal",event="created"} 1'

echo "actions smoke ok"
