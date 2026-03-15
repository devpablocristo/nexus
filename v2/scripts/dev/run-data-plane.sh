#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd go

PORT="${PORT:-8080}"
export PORT
control_plane_url="${NEXUS_CONTROL_PLANE_URL:-disabled}"
control_workers_url="${NEXUS_CONTROL_WORKERS_URL:-disabled}"
export NEXUS_API_KEYS="${NEXUS_API_KEYS:-admin=$(admin_api_key)}"
export NEXUS_CONTROL_PLANE_API_KEY="${NEXUS_CONTROL_PLANE_API_KEY:-$(data_plane_service_api_key)}"
export NEXUS_CONTROL_WORKERS_API_KEY="${NEXUS_CONTROL_WORKERS_API_KEY:-$(data_plane_service_api_key)}"

echo "starting data-plane on :${PORT} with control-plane ${control_plane_url} and control-workers ${control_workers_url}"
cd "${DATA_PLANE_DIR}"
exec go run ./cmd/api
