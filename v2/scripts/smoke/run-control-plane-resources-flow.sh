#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd curl

CONTROL_PLANE_PORT="${CONTROL_PLANE_PORT:-18120}"
BASE_URL="http://127.0.0.1:${CONTROL_PLANE_PORT}"
READY_URL="${BASE_URL}/healthz"
RESOURCES_URL="${BASE_URL}/v1/resources"
POLICIES_URL="${BASE_URL}/v1/policies"
AUDIT_URL="${BASE_URL}/v1/audit"
ADMIN_API_KEY="$(admin_api_key)"

cleanup() {
  [[ -n "${CONTROL_PLANE_PID:-}" ]] && kill "${CONTROL_PLANE_PID}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

PORT="${CONTROL_PLANE_PORT}" "${SCRIPT_DIR}/../dev/run-control-plane.sh" &
CONTROL_PLANE_PID=$!

wait_for_http "${READY_URL}" 80 0.1

create_body="$(
  curl -sS \
    -H "X-API-Key: ${ADMIN_API_KEY}" \
    -H 'Content-Type: application/json' \
    -H 'X-Nexus-Actor-Type: user' \
    -H 'X-Nexus-Actor-Id: alice' \
    -X POST "${RESOURCES_URL}" -d @- <<'JSON'
{
  "type": "wallet",
  "name": "wallet hot usdc 1",
  "environment": "prod",
  "chain": "ethereum",
  "labels": {"tier": "hot"},
  "criticality": "critical"
}
JSON
)"

resource_id="$(printf '%s' "${create_body}" | json_get "id")"
resource_type="$(printf '%s' "${create_body}" | json_get "type")"

if [[ -z "${resource_id}" || "${resource_type}" != "wallet" ]]; then
  echo "unexpected create response" >&2
  echo "${create_body}" >&2
  exit 1
fi

list_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${RESOURCES_URL}?type=wallet&environment=prod&archived=false&limit=10")"
list_count="$(printf '%s' "${list_body}" | json_len "items")"
listed_id="$(printf '%s' "${list_body}" | json_get "items.0.id")"

if [[ "${list_count}" != "1" || "${listed_id}" != "${resource_id}" ]]; then
  echo "unexpected list response" >&2
  echo "${list_body}" >&2
  exit 1
fi

archive_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -X POST "${RESOURCES_URL}/${resource_id}/archive")"
archived_at="$(printf '%s' "${archive_body}" | json_get "archived_at")"
if [[ -z "${archived_at}" ]]; then
  echo "unexpected archive response" >&2
  echo "${archive_body}" >&2
  exit 1
fi

restore_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -X POST "${RESOURCES_URL}/${resource_id}/restore")"
restored_archived_at="$(printf '%s' "${restore_body}" | json_get "archived_at")"
if [[ -n "${restored_archived_at}" ]]; then
  echo "unexpected restore response" >&2
  echo "${restore_body}" >&2
  exit 1
fi

create_policy_body="$(
  curl -sS \
    -H "X-API-Key: ${ADMIN_API_KEY}" \
    -H 'Content-Type: application/json' \
    -H 'X-Nexus-Actor-Type: user' \
    -H 'X-Nexus-Actor-Id: alice' \
    -X POST "${POLICIES_URL}" -d @- <<'JSON'
{
  "action_type": "withdrawal",
  "resource_type": "wallet",
  "effect": "allow",
  "priority": 10,
  "expression": "action.action_type == \"withdrawal\" && resource.type == \"wallet\"",
  "reason": "wallet withdrawals require approval",
  "require_approval": true,
  "approval_ttl_seconds": 600
}
JSON
)"

policy_id="$(printf '%s' "${create_policy_body}" | json_get "id")"
policy_effect="$(printf '%s' "${create_policy_body}" | json_get "effect")"

if [[ -z "${policy_id}" || "${policy_effect}" != "allow" ]]; then
  echo "unexpected policy create response" >&2
  echo "${create_policy_body}" >&2
  exit 1
fi

policy_list_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${POLICIES_URL}?action_type=withdrawal&resource_type=wallet")"
policy_count="$(printf '%s' "${policy_list_body}" | json_len "items")"
listed_policy_id="$(printf '%s' "${policy_list_body}" | json_get "items.0.id")"

if [[ "${policy_count}" != "1" || "${listed_policy_id}" != "${policy_id}" ]]; then
  echo "unexpected policy list response" >&2
  echo "${policy_list_body}" >&2
  exit 1
fi

policy_archive_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -X POST "${POLICIES_URL}/${policy_id}/archive")"
policy_archived_at="$(printf '%s' "${policy_archive_body}" | json_get "archived_at")"
if [[ -z "${policy_archived_at}" ]]; then
  echo "unexpected policy archive response" >&2
  echo "${policy_archive_body}" >&2
  exit 1
fi

policy_restore_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -X POST "${POLICIES_URL}/${policy_id}/restore")"
policy_restored_archived_at="$(printf '%s' "${policy_restore_body}" | json_get "archived_at")"
if [[ -n "${policy_restored_archived_at}" ]]; then
  echo "unexpected policy restore response" >&2
  echo "${policy_restore_body}" >&2
  exit 1
fi

policy_delete_status="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -o /dev/null -w '%{http_code}' -X DELETE "${POLICIES_URL}/${policy_id}")"
if [[ "${policy_delete_status}" != "204" ]]; then
  echo "unexpected policy delete status: ${policy_delete_status}" >&2
  exit 1
fi

delete_status="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -o /dev/null -w '%{http_code}' -X DELETE "${RESOURCES_URL}/${resource_id}")"
if [[ "${delete_status}" != "204" ]]; then
  echo "unexpected delete status: ${delete_status}" >&2
  exit 1
fi

resource_audit_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${AUDIT_URL}?resource_id=${resource_id}&event_type=resource_created&actor_id=alice")"
resource_audit_count="$(printf '%s' "${resource_audit_body}" | json_len "items")"
if [[ "${resource_audit_count}" != "1" ]]; then
  echo "unexpected resource audit response" >&2
  echo "${resource_audit_body}" >&2
  exit 1
fi

policy_audit_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${AUDIT_URL}?event_type=policy_created&actor_id=alice")"
policy_audit_count="$(printf '%s' "${policy_audit_body}" | json_len "items")"
if [[ "${policy_audit_count}" != "1" ]]; then
  echo "unexpected policy audit response" >&2
  echo "${policy_audit_body}" >&2
  exit 1
fi

echo "control-plane resources and policies smoke ok"
