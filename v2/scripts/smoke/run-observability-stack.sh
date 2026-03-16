#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd curl
require_cmd python3

PROMETHEUS_PORT="${NEXUS_PROMETHEUS_PORT:-19090}"
GRAFANA_PORT="${NEXUS_GRAFANA_PORT:-13000}"
PROMETHEUS_URL="http://127.0.0.1:${PROMETHEUS_PORT}"
GRAFANA_URL="http://127.0.0.1:${GRAFANA_PORT}"
GRAFANA_USER="${NEXUS_GRAFANA_ADMIN_USER:-admin}"
GRAFANA_PASSWORD="${NEXUS_GRAFANA_ADMIN_PASSWORD:-admin}"

wait_for_http "${PROMETHEUS_URL}/-/ready" 80 0.2
wait_for_http "${GRAFANA_URL}/api/health" 80 0.2

targets_json="$(curl -fsS "${PROMETHEUS_URL}/api/v1/targets")"
check_targets() {
  local payload="$1"
  python3 - "${payload}" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
required = {
    "nexus-data-plane",
    "nexus-control-plane",
    "nexus-control-workers",
    "postgres-data-plane",
    "postgres-control-plane",
    "postgres-control-workers",
    "postgres-audit",
}
active = {
    item["labels"]["job"]
    for item in payload["data"]["activeTargets"]
    if item.get("health") == "up"
}
missing = sorted(required - active)
if missing:
    raise SystemExit(f"missing healthy prometheus targets: {missing}")
PY
}

check_rules() {
  local payload="$1"
  python3 - "${payload}" <<'PY'
import json
import sys

payload = json.loads(sys.argv[1])
required = {
    "NexusServiceDown",
    "NexusHighErrorRate",
    "NexusHighLatencyP95",
    "NexusDatabaseDown",
}
present = set()
for group in payload["data"]["groups"]:
    for rule in group.get("rules", []):
        if "name" in rule:
            present.add(rule["name"])
missing = sorted(required - present)
if missing:
    raise SystemExit(f"missing prometheus alert rules: {missing}")
PY
}

check_dashboards() {
  local payload="$1"
  python3 - "${payload}" <<'PY'
import json
import sys

items = json.loads(sys.argv[1])
titles = {item.get("title") for item in items}
if "Nexus Pre-Prod Overview" not in titles:
    raise SystemExit("grafana dashboard not provisioned")
PY
}

retry_targets() {
  for _ in $(seq 1 40); do
    local payload
    if payload="$(curl -fsS "${PROMETHEUS_URL}/api/v1/targets")" && check_targets "${payload}"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

retry_rules() {
  for _ in $(seq 1 20); do
    local payload
    if payload="$(curl -fsS "${PROMETHEUS_URL}/api/v1/rules")" && check_rules "${payload}"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

retry_dashboards() {
  for _ in $(seq 1 30); do
    local payload
    if payload="$(curl -fsS -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" "${GRAFANA_URL}/api/search?query=Nexus")" && check_dashboards "${payload}"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

retry_targets
retry_rules
retry_dashboards

echo "observability stack smoke ok"
