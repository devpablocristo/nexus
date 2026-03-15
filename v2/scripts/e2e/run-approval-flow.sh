#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd curl
require_cmd python3

UPSTREAM_PORT="${UPSTREAM_PORT:-18112}"
DATA_PLANE_PORT="${DATA_PLANE_PORT:-18113}"
UPSTREAM_URL="http://127.0.0.1:${UPSTREAM_PORT}/echo"
BASE_URL="http://127.0.0.1:${DATA_PLANE_PORT}"
READY_URL="${BASE_URL}/v1/run/intents"

cleanup() {
  [[ -n "${UPSTREAM_PID:-}" ]] && kill "${UPSTREAM_PID}" >/dev/null 2>&1 || true
  [[ -n "${DATA_PLANE_PID:-}" ]] && kill "${DATA_PLANE_PID}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

PORT="${UPSTREAM_PORT}" "${SCRIPT_DIR}/../dev/run-echo-upstream.sh" &
UPSTREAM_PID=$!

PORT="${DATA_PLANE_PORT}" NEXUS_TOOL_ECHO_URL="${UPSTREAM_URL}" "${SCRIPT_DIR}/../dev/run-data-plane.sh" &
DATA_PLANE_PID=$!

wait_for_http "${READY_URL}" 80 0.1

create_policy() {
  curl -sS -H 'Content-Type: application/json' -X POST "${BASE_URL}/v1/policies" -d @- <<'JSON'
{
  "tool_name": "echo",
  "effect": "allow",
  "expression": "input.hello == \"review\"",
  "reason": "operator approval required",
  "require_approval": true,
  "approval_ttl_seconds": 300
}
JSON
}

run_request() {
  curl -sS -H 'Content-Type: application/json' -X POST "${BASE_URL}/v1/run" -d @- <<'JSON'
{
  "tool_name": "echo",
  "input": {
    "hello": "review"
  }
}
JSON
}

approve_request() {
  local approval_id="$1"
  curl -sS -H 'Content-Type: application/json' -X POST "${BASE_URL}/v1/approvals/${approval_id}/approve" -d '{"decided_by":"alice"}'
}

issue_lease() {
  local intent_id="$1"
  curl -sS -X POST "${BASE_URL}/v1/run/intents/${intent_id}/lease"
}

execute_intent() {
  local intent_id="$1"
  local lease_id="$2"
  curl -sS -H 'Content-Type: application/json' -X POST "${BASE_URL}/v1/run/intents/${intent_id}/execute" -d "{\"lease_id\":\"${lease_id}\"}"
}

extract_field() {
  local field="$1"
  python3 -c "import json,sys; print(json.load(sys.stdin).get('${field}', ''))"
}

policy_body="$(create_policy)"
if [[ "${policy_body}" != *'"require_approval":true'* ]]; then
  echo "failed to create policy" >&2
  echo "${policy_body}" >&2
  exit 1
fi

run_body="$(run_request)"
intent_id="$(printf '%s' "${run_body}" | extract_field "intent_id")"
approval_id="$(printf '%s' "${run_body}" | extract_field "approval_id")"
run_status="$(printf '%s' "${run_body}" | extract_field "status")"
run_reason="$(printf '%s' "${run_body}" | extract_field "reason")"

if [[ -z "${intent_id}" || -z "${approval_id}" || "${run_status}" != "blocked" || "${run_reason}" != pending\ human\ approval* ]]; then
  echo "unexpected run response" >&2
  echo "${run_body}" >&2
  exit 1
fi

approve_body="$(approve_request "${approval_id}")"
if [[ "${approve_body}" != *'"status":"approved"'* ]]; then
  echo "failed to approve" >&2
  echo "${approve_body}" >&2
  exit 1
fi

lease_body="$(issue_lease "${intent_id}")"
lease_id="$(printf '%s' "${lease_body}" | extract_field "id")"
if [[ -z "${lease_id}" ]] || [[ "${lease_body}" != *'"status":"active"'* ]]; then
  echo "failed to issue lease" >&2
  echo "${lease_body}" >&2
  exit 1
fi

execute_body="$(execute_intent "${intent_id}" "${lease_id}")"
if [[ "${execute_body}" != *'"decision":"allow"'* ]] || [[ "${execute_body}" != *'"status":"success"'* ]]; then
  echo "failed to execute intent" >&2
  echo "${execute_body}" >&2
  exit 1
fi

echo "e2e approval flow ok"
