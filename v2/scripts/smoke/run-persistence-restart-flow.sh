#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd curl
require_cmd docker

BASE_URL="http://127.0.0.1:${NEXUS_HTTP_PORT:-18080}"
CONTROL_PLANE_URL="http://127.0.0.1:${NEXUS_CONTROL_PLANE_PORT:-18082}"
CONTROL_WORKERS_URL="http://127.0.0.1:${NEXUS_CONTROL_WORKERS_PORT:-18083}"
ADMIN_API_KEY="$(admin_api_key)"

wait_for_http "${BASE_URL}/readyz" 80 0.2
wait_for_http "${CONTROL_PLANE_URL}/readyz" 80 0.2
wait_for_http "${CONTROL_WORKERS_URL}/readyz" 80 0.2

resource_body="$(
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' \
    -H 'X-Nexus-Actor-Type: user' -H 'X-Nexus-Actor-Id: alice' \
    -X POST "${CONTROL_PLANE_URL}/v1/resources" -d @- <<'JSON'
{
  "type": "wallet",
  "name": "pre-prod persistence wallet",
  "environment": "prod",
  "chain": "ethereum",
  "labels": {"tier": "hot"},
  "criticality": "critical"
}
JSON
)"
resource_id="$(printf '%s' "${resource_body}" | json_get "id")"

policy_body="$(
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' \
    -H 'X-Nexus-Actor-Type: user' -H 'X-Nexus-Actor-Id: alice' \
    -X POST "${CONTROL_PLANE_URL}/v1/policies" -d @- <<'JSON'
{
  "action_type": "withdrawal",
  "resource_type": "wallet",
  "effect": "allow",
  "priority": 10,
  "expression": "action.action_type == \"withdrawal\" && resource.environment == \"prod\"",
  "reason": "pre-prod persistence policy",
  "require_approval": true,
  "approval_ttl_seconds": 600
}
JSON
)"
policy_id="$(printf '%s' "${policy_body}" | json_get "id")"

action_body="$(
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' \
    -X POST "${BASE_URL}/v1/actions" -d @- <<JSON
{
  "action_type": "withdrawal",
  "resource_id": "${resource_id}",
  "resource_type": "wallet",
  "source_system": "treasury-orchestrator",
  "justification": "restart persistence validation",
  "requested_by": {"type": "system", "id": "treasury-bot"},
  "proposed_by": {"type": "agent", "id": "treasury-agent"},
  "payload": {
    "asset": "USDC",
    "amount": "42.00",
    "network": "ethereum",
    "destination_address": "0x999"
  }
}
JSON
)"
action_id="$(printf '%s' "${action_body}" | json_get "id")"

incident_body="$(
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' \
    -X POST "${CONTROL_WORKERS_URL}/v1/incidents" -d @- <<JSON
{
  "source_kind": "action",
  "source_id": "${action_id}",
  "action_type": "withdrawal",
  "resource_id": "${resource_id}",
  "resource_type": "wallet",
  "trigger": "execution_failed",
  "risk_level": "high",
  "reason": "restart persistence incident",
  "details": {"restart_check": true}
}
JSON
)"
incident_id="$(printf '%s' "${incident_body}" | json_get "id")"

alerts_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${CONTROL_WORKERS_URL}/v1/alerts?limit=100")"
alert_id="$(python3 - "${alerts_body}" "${incident_id}" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
incident_id = sys.argv[2]
for item in payload.get("items", []):
    if item.get("incident_id") == incident_id:
        print(item["id"])
        raise SystemExit(0)
raise SystemExit(1)
PY
)"

if [[ -z "${resource_id}" || -z "${policy_id}" || -z "${action_id}" || -z "${incident_id}" || -z "${alert_id}" ]]; then
  echo "failed to create persistence fixtures" >&2
  exit 1
fi

docker compose restart nexus-control-plane nexus-control-workers nexus-data-plane >/dev/null

wait_for_http "${BASE_URL}/readyz" 80 0.2
wait_for_http "${CONTROL_PLANE_URL}/readyz" 80 0.2
wait_for_http "${CONTROL_WORKERS_URL}/readyz" 80 0.2

curl -fsS -H "X-API-Key: ${ADMIN_API_KEY}" "${CONTROL_PLANE_URL}/v1/resources/${resource_id}" >/dev/null
curl -fsS -H "X-API-Key: ${ADMIN_API_KEY}" "${CONTROL_PLANE_URL}/v1/policies/${policy_id}" >/dev/null
curl -fsS -H "X-API-Key: ${ADMIN_API_KEY}" "${BASE_URL}/v1/actions/${action_id}" >/dev/null
curl -fsS -H "X-API-Key: ${ADMIN_API_KEY}" "${CONTROL_WORKERS_URL}/v1/incidents/${incident_id}" >/dev/null
curl -fsS -H "X-API-Key: ${ADMIN_API_KEY}" "${CONTROL_WORKERS_URL}/v1/alerts/${alert_id}" >/dev/null

audit_body="$(curl -fsS -H "X-API-Key: ${ADMIN_API_KEY}" "${CONTROL_PLANE_URL}/v1/audit?action_id=${action_id}&limit=20")"
audit_count="$(printf '%s' "${audit_body}" | json_len "items")"
if [[ "${audit_count}" -lt 1 ]]; then
  echo "expected audit records after restart" >&2
  echo "${audit_body}" >&2
  exit 1
fi

echo "postgres persistence restart smoke ok"
