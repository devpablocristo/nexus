#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
V2_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
DATA_PLANE_DIR="${V2_ROOT}/data-plane"
CONTROL_PLANE_DIR="${V2_ROOT}/control-plane"
CONTROL_WORKERS_DIR="${V2_ROOT}/control-workers"

NEXUS_ADMIN_API_KEY="${NEXUS_ADMIN_API_KEY:-nexus-admin-dev-key}"
NEXUS_DATA_PLANE_SERVICE_API_KEY="${NEXUS_DATA_PLANE_SERVICE_API_KEY:-nexus-data-plane-service-dev-key}"
NEXUS_CONTROL_WORKERS_SERVICE_API_KEY="${NEXUS_CONTROL_WORKERS_SERVICE_API_KEY:-nexus-control-workers-service-dev-key}"

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "missing required command: ${cmd}" >&2
    exit 1
  fi
}

json_get() {
  local path="$1"

  require_cmd python3

  python3 -c '
import json
import sys

path = [part for part in sys.argv[1].split(".") if part]
value = json.load(sys.stdin)

for part in path:
    if isinstance(value, list):
        value = value[int(part)]
        continue
    if isinstance(value, dict):
        value = value.get(part)
        continue
    value = None
    break

if value is None:
    print("")
elif isinstance(value, bool):
    print("true" if value else "false")
elif isinstance(value, (dict, list)):
    print(json.dumps(value))
else:
    print(value)
' "${path}"
}

json_len() {
  local path="$1"

  require_cmd python3

  python3 -c '
import json
import sys

path = [part for part in sys.argv[1].split(".") if part]
value = json.load(sys.stdin)

for part in path:
    if isinstance(value, list):
        value = value[int(part)]
        continue
    if isinstance(value, dict):
        value = value.get(part)
        continue
    value = None
    break

if isinstance(value, (list, dict)):
    print(len(value))
else:
    print(0)
' "${path}"
}

wait_for_http() {
  local url="$1"
  local attempts="${2:-50}"
  local delay="${3:-0.2}"

  require_cmd curl

  for _ in $(seq 1 "${attempts}"); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay}"
  done

  echo "timed out waiting for ${url}" >&2
  return 1
}

admin_api_key() {
  printf '%s' "${NEXUS_ADMIN_API_KEY}"
}

data_plane_service_api_key() {
  printf '%s' "${NEXUS_DATA_PLANE_SERVICE_API_KEY}"
}

control_workers_service_api_key() {
  printf '%s' "${NEXUS_CONTROL_WORKERS_SERVICE_API_KEY}"
}
