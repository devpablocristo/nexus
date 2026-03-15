#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd curl

UPSTREAM_PORT="${UPSTREAM_PORT:-18110}"
DATA_PLANE_PORT="${DATA_PLANE_PORT:-18111}"
UPSTREAM_URL="http://127.0.0.1:${UPSTREAM_PORT}/echo"
RUN_URL="http://127.0.0.1:${DATA_PLANE_PORT}/v1/run"
READY_URL="http://127.0.0.1:${DATA_PLANE_PORT}/v1/run/intents"

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

response_file="$(mktemp)"
status="$(
  curl -sS -o "${response_file}" -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -X POST "${RUN_URL}" \
    -d '{"tool_name":"echo","input":{"hello":"world"}}'
)"
body="$(cat "${response_file}")"
rm -f "${response_file}"

if [[ "${status}" != "200" ]]; then
  echo "unexpected status: ${status}" >&2
  echo "${body}" >&2
  exit 1
fi

if [[ "${body}" != *'"decision":"allow"'* ]] || [[ "${body}" != *'"status":"success"'* ]] || [[ "${body}" != *'"hello":"world"'* ]]; then
  echo "unexpected response body" >&2
  echo "${body}" >&2
  exit 1
fi

echo "smoke ok"
