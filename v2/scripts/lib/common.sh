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

find_free_port() {
  local start="${1:-18000}"
  local end="${2:-18999}"

  require_cmd python3

  python3 - "${start}" "${end}" <<'PY'
import random
import socket
import sys

start = int(sys.argv[1])
end = int(sys.argv[2])
ports = list(range(start, end + 1))
random.shuffle(ports)

for port in ports:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        try:
            sock.bind(("127.0.0.1", port))
        except OSError:
            continue
    print(port)
    raise SystemExit(0)

raise SystemExit(1)
PY
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

fetch_metrics() {
  local url="$1"
  local api_key="$2"

  require_cmd curl

  curl -fsS -H "X-API-Key: ${api_key}" "${url}/metrics"
}

assert_metrics_contains() {
  local metrics="$1"
  local expected="$2"

  require_cmd grep

  if ! printf '%s' "${metrics}" | grep -F "${expected}" >/dev/null 2>&1; then
    echo "expected metrics to contain: ${expected}" >&2
    echo "${metrics}" >&2
    return 1
  fi
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
