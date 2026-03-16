#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd curl

CONTROL_PLANE_PORT="${CONTROL_PLANE_PORT:-$(find_free_port 18140 18149)}"
DATA_PLANE_PORT="${DATA_PLANE_PORT:-$(find_free_port 18150 18159)}"
CONTROL_PLANE_URL="http://127.0.0.1:${CONTROL_PLANE_PORT}"
BASE_URL="http://127.0.0.1:${DATA_PLANE_PORT}"
READY_URL="${BASE_URL}/readyz"
ACTIONS_URL="${BASE_URL}/v1/actions"
AUDIT_URL="${CONTROL_PLANE_URL}/v1/audit"
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
  "reason": "production wallet withdrawals need approval",
  "require_approval": true,
  "approval_ttl_seconds": 600
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

create_action() {
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${ACTIONS_URL}" -d @- <<JSON
{
  "action_type": "withdrawal",
  "resource_id": "${resource_id}",
  "resource_type": "wallet",
  "source_system": "treasury-orchestrator",
  "justification": "Daily treasury control flow",
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

approve_action() {
  local action_id="$1"

  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${ACTIONS_URL}/${action_id}/approve" -d @- <<'JSON'
{
  "decided_by": {"type": "user", "id": "alice"},
  "comment": "approved after treasury review"
}
JSON
}

issue_lease() {
  local action_id="$1"
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -X POST "${ACTIONS_URL}/${action_id}/lease"
}

execute_action() {
  local action_id="$1"
  local lease_id="$2"

  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${ACTIONS_URL}/${action_id}/execute" -d @- <<JSON
{
  "lease_id": "${lease_id}",
  "executed_by": {"type": "system", "id": "wallet-orchestrator"}
}
JSON
}

created_action="$(create_action)"
action_id="$(printf '%s' "${created_action}" | json_get "id")"

if [[ -z "${action_id}" ]]; then
  echo "failed to create action" >&2
  echo "${created_action}" >&2
  exit 1
fi

list_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${ACTIONS_URL}?action_type=withdrawal&status=pending_approval&limit=10")"
list_count="$(printf '%s' "${list_body}" | json_len "items")"
listed_id="$(printf '%s' "${list_body}" | json_get "items.0.id")"

if [[ "${list_count}" != "1" || "${listed_id}" != "${action_id}" ]]; then
  echo "unexpected list response" >&2
  echo "${list_body}" >&2
  exit 1
fi

get_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${ACTIONS_URL}/${action_id}")"
get_status="$(printf '%s' "${get_body}" | json_get "status")"
get_decision="$(printf '%s' "${get_body}" | json_get "decision")"

if [[ "${get_status}" != "pending_approval" || "${get_decision}" != "require_approval" ]]; then
  echo "unexpected get response" >&2
  echo "${get_body}" >&2
  exit 1
fi

risk_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${ACTIONS_URL}/${action_id}/risk")"
risk_level="$(printf '%s' "${risk_body}" | json_get "level")"
risk_score="$(printf '%s' "${risk_body}" | json_get "score")"
risk_factor_count="$(printf '%s' "${risk_body}" | json_len "factors")"
risk_profile="$(printf '%s' "${risk_body}" | json_get "profile.name")"
risk_recommended="$(printf '%s' "${risk_body}" | json_get "recommended_decision")"

if [[ "${risk_level}" != "medium" || "${risk_score}" != "30" || "${risk_factor_count}" != "2" || "${risk_profile}" != "balanced" || "${risk_recommended}" != "enhanced_log" ]]; then
  echo "unexpected risk response" >&2
  echo "${risk_body}" >&2
  exit 1
fi

evidence_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${ACTIONS_URL}/${action_id}/evidence")"
evidence_count="$(printf '%s' "${evidence_body}" | json_len "items")"
evidence_kind="$(printf '%s' "${evidence_body}" | json_get "items.2.kind")"
matched_policy_id="$(printf '%s' "${evidence_body}" | json_get "items.2.details.matched_policy_id")"

if [[ "${evidence_count}" != "3" || "${evidence_kind}" != "policy_decision" || "${matched_policy_id}" != "${policy_id}" ]]; then
  echo "unexpected evidence response" >&2
  echo "${evidence_body}" >&2
  exit 1
fi

approved_body="$(approve_action "${action_id}")"
approved_status="$(printf '%s' "${approved_body}" | json_get "status")"
approved_decision="$(printf '%s' "${approved_body}" | json_get "decision")"

if [[ "${approved_status}" != "approved" || "${approved_decision}" != "allow" ]]; then
  echo "unexpected approve response" >&2
  echo "${approved_body}" >&2
  exit 1
fi

leased_body="$(issue_lease "${action_id}")"
leased_status="$(printf '%s' "${leased_body}" | json_get "status")"
lease_id="$(printf '%s' "${leased_body}" | json_get "lease.id")"
lease_status="$(printf '%s' "${leased_body}" | json_get "lease.status")"

if [[ "${leased_status}" != "leased" || -z "${lease_id}" || "${lease_status}" != "active" ]]; then
  echo "unexpected lease response" >&2
  echo "${leased_body}" >&2
  exit 1
fi

executed_body="$(execute_action "${action_id}" "${lease_id}")"
executed_status="$(printf '%s' "${executed_body}" | json_get "status")"
execution_status="$(printf '%s' "${executed_body}" | json_get "execution.status")"
used_lease_status="$(printf '%s' "${executed_body}" | json_get "lease.status")"
execution_id="$(printf '%s' "${executed_body}" | json_get "execution.result.execution_id")"

if [[ "${executed_status}" != "executed" || "${execution_status}" != "success" || "${used_lease_status}" != "used" || -z "${execution_id}" ]]; then
  echo "unexpected execute response" >&2
  echo "${executed_body}" >&2
  exit 1
fi

audit_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${AUDIT_URL}?action_id=${action_id}&limit=10")"
audit_count="$(printf '%s' "${audit_body}" | json_len "items")"
latest_event="$(printf '%s' "${audit_body}" | json_get "items.0.event_type")"
second_event="$(printf '%s' "${audit_body}" | json_get "items.1.event_type")"

if [[ "${audit_count}" != "4" || "${latest_event}" != "action_executed" || "${second_event}" != "action_leased" ]]; then
  echo "unexpected audit response" >&2
  echo "${audit_body}" >&2
  exit 1
fi

echo "actions e2e lifecycle ok"
