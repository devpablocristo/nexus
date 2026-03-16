#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd curl

CONTROL_PLANE_PORT="${CONTROL_PLANE_PORT:-$(find_free_port 18160 18169)}"
CONTROL_WORKERS_PORT="${CONTROL_WORKERS_PORT:-$(find_free_port 18170 18179)}"
DATA_PLANE_PORT="${DATA_PLANE_PORT:-$(find_free_port 18180 18189)}"
CONTROL_PLANE_URL="http://127.0.0.1:${CONTROL_PLANE_PORT}"
CONTROL_WORKERS_URL="http://127.0.0.1:${CONTROL_WORKERS_PORT}"
DATA_PLANE_URL="http://127.0.0.1:${DATA_PLANE_PORT}"
ALERTS_URL="${CONTROL_WORKERS_URL}/v1/alerts"
AUDIT_URL="${CONTROL_PLANE_URL}/v1/audit"
ADMIN_API_KEY="$(admin_api_key)"

cleanup() {
  [[ -n "${CONTROL_PLANE_PID:-}" ]] && kill "${CONTROL_PLANE_PID}" >/dev/null 2>&1 || true
  [[ -n "${CONTROL_WORKERS_PID:-}" ]] && kill "${CONTROL_WORKERS_PID}" >/dev/null 2>&1 || true
  [[ -n "${DATA_PLANE_PID:-}" ]] && kill "${DATA_PLANE_PID}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

create_resource() {
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${CONTROL_PLANE_URL}/v1/resources" -d @- <<'JSON'
{
  "type": "wallet",
  "name": "wallet hot ops 1",
  "environment": "prod",
  "chain": "ethereum",
  "labels": {"tier": "hot"},
  "criticality": "critical"
}
JSON
}

create_canary_resource() {
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${CONTROL_PLANE_URL}/v1/resources" -d @- <<'JSON'
{
  "type": "wallet",
  "name": "wallet canary 1",
  "environment": "prod",
  "chain": "ethereum",
  "labels": {"tier": "trap"},
  "criticality": "low",
  "is_canary": true
}
JSON
}

create_deny_policy() {
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${CONTROL_PLANE_URL}/v1/policies" -d @- <<'JSON'
{
  "action_type": "withdrawal",
  "resource_type": "wallet",
  "effect": "deny",
  "priority": 1,
  "expression": "resource.environment == \"prod\"",
  "reason": "production withdrawals blocked by operator policy",
  "require_approval": false
}
JSON
}

create_review_policy() {
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${CONTROL_PLANE_URL}/v1/policies" -d @- <<'JSON'
{
  "action_type": "treasury_transfer",
  "resource_type": "wallet",
  "effect": "allow",
  "priority": 10,
  "expression": "action.action_type == \"treasury_transfer\"",
  "reason": "treasury transfers need manual review",
  "require_approval": true,
  "approval_ttl_seconds": 600
}
JSON
}

create_blocked_action() {
  local resource_id="$1"

  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${DATA_PLANE_URL}/v1/actions" -d @- <<JSON
{
  "action_type": "withdrawal",
  "resource_id": "${resource_id}",
  "resource_type": "wallet",
  "source_system": "treasury-orchestrator",
  "justification": "blocked action test",
  "requested_by": {"type": "system", "id": "treasury-bot"},
  "proposed_by": {"type": "agent", "id": "treasury-agent"},
  "payload": {
    "asset": "USDC",
    "amount": "1000.00",
    "network": "ethereum",
    "destination_address": "0xabc"
  }
}
JSON
}

create_review_action() {
  local resource_id="$1"

  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${DATA_PLANE_URL}/v1/actions" -d @- <<JSON
{
  "action_type": "treasury_transfer",
  "resource_id": "${resource_id}",
  "resource_type": "wallet",
  "source_system": "treasury-orchestrator",
  "justification": "rejected action test",
  "requested_by": {"type": "system", "id": "treasury-bot"},
  "proposed_by": {"type": "agent", "id": "treasury-agent"},
  "payload": {
    "asset": "USDC",
    "amount": "5000.00",
    "from_account": "hot-wallet",
    "to_account": "cold-wallet"
  }
}
JSON
}

create_canary_action() {
  local resource_id="$1"

  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${DATA_PLANE_URL}/v1/actions" -d @- <<JSON
{
  "action_type": "withdrawal",
  "resource_id": "${resource_id}",
  "resource_type": "wallet",
  "source_system": "treasury-orchestrator",
  "justification": "canary trap test",
  "requested_by": {"type": "system", "id": "treasury-bot"},
  "proposed_by": {"type": "agent", "id": "treasury-agent"},
  "payload": {
    "asset": "USDC",
    "amount": "1.00",
    "network": "ethereum",
    "destination_address": "0xcanary"
  }
}
JSON
}

reject_action() {
  local action_id="$1"

  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${DATA_PLANE_URL}/v1/actions/${action_id}/reject" -d @- <<'JSON'
{
  "decided_by": {"type": "user", "id": "alice"},
  "comment": "manual rejection after treasury review"
}
JSON
}

PORT="${CONTROL_PLANE_PORT}" "${SCRIPT_DIR}/../dev/run-control-plane.sh" &
CONTROL_PLANE_PID=$!
PORT="${CONTROL_WORKERS_PORT}" NEXUS_CONTROL_PLANE_URL="${CONTROL_PLANE_URL}" "${SCRIPT_DIR}/../dev/run-control-workers.sh" &
CONTROL_WORKERS_PID=$!

wait_for_http "${CONTROL_PLANE_URL}/readyz" 80 0.1
wait_for_http "${CONTROL_WORKERS_URL}/readyz" 80 0.1

resource_body="$(create_resource)"
resource_id="$(printf '%s' "${resource_body}" | json_get "id")"
if [[ -z "${resource_id}" ]]; then
  echo "failed to create resource" >&2
  echo "${resource_body}" >&2
  exit 1
fi

canary_resource_body="$(create_canary_resource)"
canary_resource_id="$(printf '%s' "${canary_resource_body}" | json_get "id")"
canary_is_flagged="$(printf '%s' "${canary_resource_body}" | json_get "is_canary")"
if [[ -z "${canary_resource_id}" || "${canary_is_flagged}" != "true" ]]; then
  echo "failed to create canary resource" >&2
  echo "${canary_resource_body}" >&2
  exit 1
fi

deny_policy_body="$(create_deny_policy)"
deny_policy_id="$(printf '%s' "${deny_policy_body}" | json_get "id")"
if [[ -z "${deny_policy_id}" ]]; then
  echo "failed to create deny policy" >&2
  echo "${deny_policy_body}" >&2
  exit 1
fi

review_policy_body="$(create_review_policy)"
review_policy_id="$(printf '%s' "${review_policy_body}" | json_get "id")"
if [[ -z "${review_policy_id}" ]]; then
  echo "failed to create review policy" >&2
  echo "${review_policy_body}" >&2
  exit 1
fi

PORT="${DATA_PLANE_PORT}" NEXUS_CONTROL_PLANE_URL="${CONTROL_PLANE_URL}" NEXUS_CONTROL_WORKERS_URL="${CONTROL_WORKERS_URL}" "${SCRIPT_DIR}/../dev/run-data-plane.sh" &
DATA_PLANE_PID=$!

wait_for_http "${DATA_PLANE_URL}/readyz" 80 0.1

blocked_action_body="$(create_blocked_action "${resource_id}")"
blocked_action_id="$(printf '%s' "${blocked_action_body}" | json_get "id")"
blocked_status="$(printf '%s' "${blocked_action_body}" | json_get "status")"
blocked_decision="$(printf '%s' "${blocked_action_body}" | json_get "decision")"
if [[ -z "${blocked_action_id}" || "${blocked_status}" != "blocked" || "${blocked_decision}" != "deny" ]]; then
  echo "unexpected blocked action response" >&2
  echo "${blocked_action_body}" >&2
  exit 1
fi

blocked_incidents="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${CONTROL_WORKERS_URL}/v1/incidents?trigger=blocked_action&status=open")"
blocked_count="$(printf '%s' "${blocked_incidents}" | json_len "items")"
blocked_incident_id="$(printf '%s' "${blocked_incidents}" | json_get "items.0.id")"
blocked_source_id="$(printf '%s' "${blocked_incidents}" | json_get "items.0.source_id")"
blocked_incident_action_id="$(printf '%s' "${blocked_incidents}" | json_get "items.0.action_id")"
blocked_reason="$(printf '%s' "${blocked_incidents}" | json_get "items.0.reason")"
if [[ "${blocked_count}" != "1" || "${blocked_source_id}" != "${blocked_action_id}" || "${blocked_incident_action_id}" != "${blocked_action_id}" || "${blocked_reason}" != "production withdrawals blocked by operator policy" ]]; then
  echo "unexpected blocked incidents response" >&2
  echo "${blocked_incidents}" >&2
  exit 1
fi

blocked_alerts="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${ALERTS_URL}?channel=slack&status=pending&limit=10")"
blocked_alert_count="$(printf '%s' "${blocked_alerts}" | json_len "items")"
if [[ "${blocked_alert_count}" != "0" ]]; then
  echo "unexpected blocked alerts response" >&2
  echo "${blocked_alerts}" >&2
  exit 1
fi

blocked_audit="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${AUDIT_URL}?action_id=${blocked_action_id}&event_type=incident_created")"
blocked_audit_count="$(printf '%s' "${blocked_audit}" | json_len "items")"
blocked_audit_incident_id="$(printf '%s' "${blocked_audit}" | json_get "items.0.incident_id")"
if [[ "${blocked_audit_count}" != "1" || "${blocked_audit_incident_id}" != "${blocked_incident_id}" ]]; then
  echo "unexpected blocked audit response" >&2
  echo "${blocked_audit}" >&2
  exit 1
fi

blocked_alert_audit="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${AUDIT_URL}?incident_id=${blocked_incident_id}&event_type=alert_created")"
blocked_alert_audit_count="$(printf '%s' "${blocked_alert_audit}" | json_len "items")"
if [[ "${blocked_alert_audit_count}" != "0" ]]; then
  echo "unexpected blocked alert audit response" >&2
  echo "${blocked_alert_audit}" >&2
  exit 1
fi

review_action_body="$(create_review_action "${resource_id}")"
review_action_id="$(printf '%s' "${review_action_body}" | json_get "id")"
review_status="$(printf '%s' "${review_action_body}" | json_get "status")"
review_decision="$(printf '%s' "${review_action_body}" | json_get "decision")"
if [[ -z "${review_action_id}" || "${review_status}" != "pending_approval" || "${review_decision}" != "require_approval" ]]; then
  echo "unexpected review action response" >&2
  echo "${review_action_body}" >&2
  exit 1
fi

rejected_body="$(reject_action "${review_action_id}")"
rejected_status="$(printf '%s' "${rejected_body}" | json_get "status")"
if [[ "${rejected_status}" != "rejected" ]]; then
  echo "unexpected reject response" >&2
  echo "${rejected_body}" >&2
  exit 1
fi

rejected_incidents="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${CONTROL_WORKERS_URL}/v1/incidents?trigger=approval_rejected&status=open")"
rejected_count="$(printf '%s' "${rejected_incidents}" | json_len "items")"
rejected_incident_id="$(printf '%s' "${rejected_incidents}" | json_get "items.0.id")"
rejected_source_id="$(printf '%s' "${rejected_incidents}" | json_get "items.0.source_id")"
rejected_incident_action_id="$(printf '%s' "${rejected_incidents}" | json_get "items.0.action_id")"
rejected_reason="$(printf '%s' "${rejected_incidents}" | json_get "items.0.reason")"
if [[ "${rejected_count}" != "1" || "${rejected_source_id}" != "${review_action_id}" || "${rejected_incident_action_id}" != "${review_action_id}" || "${rejected_reason}" != "manual rejection after treasury review" ]]; then
  echo "unexpected rejected incidents response" >&2
  echo "${rejected_incidents}" >&2
  exit 1
fi

rejected_audit="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${AUDIT_URL}?action_id=${review_action_id}&event_type=alert_created")"
rejected_audit_count="$(printf '%s' "${rejected_audit}" | json_len "items")"
rejected_audit_route="$(printf '%s' "${rejected_audit}" | json_get "items.0.data.route")"
if [[ "${rejected_audit_count}" != "1" || "${rejected_audit_route}" != "ops-p1" ]]; then
  echo "unexpected rejected audit response" >&2
  echo "${rejected_audit}" >&2
  exit 1
fi

canary_action_body="$(create_canary_action "${canary_resource_id}")"
canary_action_id="$(printf '%s' "${canary_action_body}" | json_get "id")"
canary_status="$(printf '%s' "${canary_action_body}" | json_get "status")"
canary_decision="$(printf '%s' "${canary_action_body}" | json_get "decision")"
canary_trap="$(printf '%s' "${canary_action_body}" | json_get "risk.recommended_decision")"
if [[ -z "${canary_action_id}" || "${canary_status}" != "blocked" || "${canary_decision}" != "deny" || -z "${canary_trap}" ]]; then
  echo "unexpected canary action response" >&2
  echo "${canary_action_body}" >&2
  exit 1
fi

canary_incidents="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${CONTROL_WORKERS_URL}/v1/incidents?trigger=canary_triggered&resource_id=${canary_resource_id}&status=open")"
canary_count="$(printf '%s' "${canary_incidents}" | json_len "items")"
canary_incident_id="$(printf '%s' "${canary_incidents}" | json_get "items.0.id")"
canary_severity="$(printf '%s' "${canary_incidents}" | json_get "items.0.severity")"
canary_incident_action_id="$(printf '%s' "${canary_incidents}" | json_get "items.0.action_id")"
if [[ "${canary_count}" != "1" || "${canary_severity}" != "critical" || "${canary_incident_action_id}" != "${canary_action_id}" ]]; then
  echo "unexpected canary incidents response" >&2
  echo "${canary_incidents}" >&2
  exit 1
fi

canary_audit="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${AUDIT_URL}?action_id=${canary_action_id}&event_type=alert_created")"
canary_audit_count="$(printf '%s' "${canary_audit}" | json_len "items")"
canary_audit_incident_id="$(printf '%s' "${canary_audit}" | json_get "items.0.incident_id")"
canary_audit_route="$(printf '%s' "${canary_audit}" | json_get "items.0.data.route")"
if [[ "${canary_audit_count}" != "1" || "${canary_audit_incident_id}" != "${canary_incident_id}" || "${canary_audit_route}" != "ops-p1" ]]; then
  echo "unexpected canary audit response" >&2
  echo "${canary_audit}" >&2
  exit 1
fi

echo "actions incidents and alerts e2e ok"
