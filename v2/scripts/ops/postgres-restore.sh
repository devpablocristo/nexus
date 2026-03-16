#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd docker

usage() {
  cat <<'EOF' >&2
usage: scripts/ops/postgres-restore.sh <data-plane|control-plane|control-workers|audit> <backup.sql>
EOF
  exit 1
}

[[ $# -eq 2 ]] || usage

database_alias="$1"
backup_path="$2"

[[ -f "${backup_path}" ]] || {
  echo "backup file not found: ${backup_path}" >&2
  exit 1
}

case "${database_alias}" in
  data-plane)
    app_services=(nexus-data-plane)
    db_service="data-plane-postgres"
    db_name="nexus_data_plane"
    ;;
  control-plane)
    app_services=(nexus-control-plane)
    db_service="control-plane-postgres"
    db_name="nexus_control_plane"
    ;;
  control-workers)
    app_services=(nexus-control-workers)
    db_service="control-workers-postgres"
    db_name="nexus_control_workers"
    ;;
  audit)
    app_services=(nexus-control-plane)
    db_service="audit-postgres"
    db_name="nexus_audit"
    ;;
  *)
    usage
    ;;
esac

docker compose stop "${app_services[@]}" >/dev/null
docker compose exec -T "${db_service}" psql -U postgres -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${db_name}' AND pid <> pg_backend_pid();" >/dev/null
docker compose exec -T "${db_service}" psql -U postgres -d postgres < "${backup_path}" >/dev/null
docker compose up -d --wait "${app_services[@]}" >/dev/null

echo "postgres restore completed for ${database_alias}"
