#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd curl

CONTROL_WORKERS_PORT="${CONTROL_WORKERS_PORT:-$(find_free_port 18130 18139)}"
BASE_URL="http://127.0.0.1:${CONTROL_WORKERS_PORT}"
READY_URL="${BASE_URL}/readyz"
INCIDENTS_URL="${BASE_URL}/v1/incidents"
ALERTS_URL="${BASE_URL}/v1/alerts"
ADMIN_API_KEY="$(admin_api_key)"

cleanup() {
  [[ -n "${CONTROL_WORKERS_PID:-}" ]] && kill "${CONTROL_WORKERS_PID}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

PORT="${CONTROL_WORKERS_PORT}" "${SCRIPT_DIR}/../dev/run-control-workers.sh" &
CONTROL_WORKERS_PID=$!

wait_for_http "${READY_URL}" 80 0.1

create_body="$(
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X POST "${INCIDENTS_URL}" -d @- <<'JSON'
{
  "source_kind": "action",
  "source_id": "action-1",
  "action_type": "withdrawal",
  "resource_id": "wallet_hot_usdc_1",
  "resource_type": "wallet",
  "trigger": "execution_failed",
  "risk_level": "critical",
  "reason": "executor could not reach signer",
  "details": {"attempt": 1}
}
JSON
)"

incident_id="$(printf '%s' "${create_body}" | json_get "id")"
severity="$(printf '%s' "${create_body}" | json_get "severity")"
status="$(printf '%s' "${create_body}" | json_get "status")"

if [[ -z "${incident_id}" || "${severity}" != "critical" || "${status}" != "open" ]]; then
  echo "unexpected create response" >&2
  echo "${create_body}" >&2
  exit 1
fi

alerts_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${ALERTS_URL}?severity=critical&status=pending&limit=10")"
alert_count="$(printf '%s' "${alerts_body}" | json_len "items")"
alert_source_id="$(printf '%s' "${alerts_body}" | json_get "items.0.source_id")"
alert_channel="$(printf '%s' "${alerts_body}" | json_get "items.0.channel")"
alert_route="$(printf '%s' "${alerts_body}" | json_get "items.0.route")"

if [[ "${alert_count}" != "1" || "${alert_source_id}" != "${incident_id}" || "${alert_channel}" != "pagerduty" || "${alert_route}" != "ops-p1" ]]; then
  echo "unexpected alerts response" >&2
  echo "${alerts_body}" >&2
  exit 1
fi

list_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" "${INCIDENTS_URL}?trigger=execution_failed&status=open&limit=10")"
list_count="$(printf '%s' "${list_body}" | json_len "items")"
listed_id="$(printf '%s' "${list_body}" | json_get "items.0.id")"

if [[ "${list_count}" != "1" || "${listed_id}" != "${incident_id}" ]]; then
  echo "unexpected list response" >&2
  echo "${list_body}" >&2
  exit 1
fi

update_body="$(
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' -X PATCH "${INCIDENTS_URL}/${incident_id}" -d @- <<'JSON'
{
  "status": "resolved",
  "summary": "withdrawal incident resolved after manual review"
}
JSON
)"

resolved_status="$(printf '%s' "${update_body}" | json_get "status")"
resolved_at="$(printf '%s' "${update_body}" | json_get "resolved_at")"

if [[ "${resolved_status}" != "resolved" || -z "${resolved_at}" ]]; then
  echo "unexpected update response" >&2
  echo "${update_body}" >&2
  exit 1
fi

archive_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -X POST "${INCIDENTS_URL}/${incident_id}/archive")"
archived_at="$(printf '%s' "${archive_body}" | json_get "archived_at")"
if [[ -z "${archived_at}" ]]; then
  echo "unexpected archive response" >&2
  echo "${archive_body}" >&2
  exit 1
fi

restore_body="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -X POST "${INCIDENTS_URL}/${incident_id}/restore")"
restored_archived_at="$(printf '%s' "${restore_body}" | json_get "archived_at")"
if [[ -n "${restored_archived_at}" ]]; then
  echo "unexpected restore response" >&2
  echo "${restore_body}" >&2
  exit 1
fi

delete_status="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -o /dev/null -w '%{http_code}' -X DELETE "${INCIDENTS_URL}/${incident_id}")"
if [[ "${delete_status}" != "204" ]]; then
  echo "unexpected delete status: ${delete_status}" >&2
  exit 1
fi

metrics_body="$(fetch_metrics "${BASE_URL}" "${ADMIN_API_KEY}")"
assert_metrics_contains "${metrics_body}" 'nexus_http_requests_total{method="POST",route="/v1/incidents",status_code="201"} 1'
assert_metrics_contains "${metrics_body}" 'nexus_incidents_created_total{severity="critical",trigger="execution_failed"} 1'
assert_metrics_contains "${metrics_body}" 'nexus_alerts_created_total{channel="pagerduty",severity="critical"} 1'

echo "control-workers incidents and alerts smoke ok"
