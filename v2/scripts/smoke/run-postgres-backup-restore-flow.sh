#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd curl
require_cmd docker
require_cmd mktemp

CONTROL_PLANE_URL="http://127.0.0.1:${NEXUS_CONTROL_PLANE_PORT:-18082}"
ADMIN_API_KEY="$(admin_api_key)"

wait_for_http "${CONTROL_PLANE_URL}/readyz" 80 0.2

resource_body="$(
  curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -H 'Content-Type: application/json' \
    -H 'X-Nexus-Actor-Type: user' -H 'X-Nexus-Actor-Id: alice' \
    -X POST "${CONTROL_PLANE_URL}/v1/resources" -d @- <<'JSON'
{
  "type": "wallet",
  "name": "backup restore wallet",
  "environment": "prod",
  "chain": "ethereum",
  "labels": {"tier": "warm"},
  "criticality": "high"
}
JSON
)"
resource_id="$(printf '%s' "${resource_body}" | json_get "id")"
[[ -n "${resource_id}" ]] || {
  echo "failed to create resource for backup/restore" >&2
  echo "${resource_body}" >&2
  exit 1
}

backup_file="$(mktemp -t nexus-control-plane-backup-XXXXXX.sql)"
cleanup() {
  rm -f "${backup_file}"
}
trap cleanup EXIT

"${SCRIPT_DIR}/../ops/postgres-backup.sh" control-plane "${backup_file}" >/dev/null

docker compose stop nexus-control-plane >/dev/null
docker compose exec -T control-plane-postgres psql -U postgres -d nexus_control_plane -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;' >/dev/null
docker compose up -d --wait nexus-control-plane >/dev/null

missing_status="$(curl -sS -H "X-API-Key: ${ADMIN_API_KEY}" -o /dev/null -w '%{http_code}' "${CONTROL_PLANE_URL}/v1/resources/${resource_id}")"
if [[ "${missing_status}" != "404" ]]; then
  echo "expected resource to be absent before restore, got ${missing_status}" >&2
  exit 1
fi

"${SCRIPT_DIR}/../ops/postgres-restore.sh" control-plane "${backup_file}" >/dev/null
wait_for_http "${CONTROL_PLANE_URL}/readyz" 80 0.2
curl -fsS -H "X-API-Key: ${ADMIN_API_KEY}" "${CONTROL_PLANE_URL}/v1/resources/${resource_id}" >/dev/null

echo "postgres backup restore smoke ok"
