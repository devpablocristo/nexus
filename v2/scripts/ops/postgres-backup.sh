#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd docker

usage() {
  cat <<'EOF' >&2
usage: scripts/ops/postgres-backup.sh <data-plane|control-plane|control-workers|audit> <output.sql>
EOF
  exit 1
}

[[ $# -eq 2 ]] || usage

database_alias="$1"
output_path="$2"

case "${database_alias}" in
  data-plane)
    db_service="data-plane-postgres"
    db_name="nexus_data_plane"
    ;;
  control-plane)
    db_service="control-plane-postgres"
    db_name="nexus_control_plane"
    ;;
  control-workers)
    db_service="control-workers-postgres"
    db_name="nexus_control_workers"
    ;;
  audit)
    db_service="audit-postgres"
    db_name="nexus_audit"
    ;;
  *)
    usage
    ;;
esac

mkdir -p "$(dirname "${output_path}")"

docker compose exec -T "${db_service}" pg_dump \
  -U postgres \
  --clean \
  --create \
  --if-exists \
  --no-owner \
  --no-privileges \
  "${db_name}" > "${output_path}"

echo "postgres backup written to ${output_path}"
