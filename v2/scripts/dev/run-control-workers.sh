#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd go

PORT="${PORT:-8082}"
export PORT
export NEXUS_API_KEYS="${NEXUS_API_KEYS:-admin=$(admin_api_key),data-plane=$(data_plane_service_api_key)}"
export NEXUS_CONTROL_PLANE_API_KEY="${NEXUS_CONTROL_PLANE_API_KEY:-$(control_workers_service_api_key)}"

echo "starting control-workers on :${PORT}"
cd "${CONTROL_WORKERS_DIR}"
exec go run ./cmd/api
