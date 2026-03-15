#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd go

PORT="${PORT:-8080}"
export PORT
export NEXUS_TOOL_ECHO_URL="${NEXUS_TOOL_ECHO_URL:-http://127.0.0.1:18081/echo}"
control_plane_url="${NEXUS_CONTROL_PLANE_URL:-disabled}"
control_workers_url="${NEXUS_CONTROL_WORKERS_URL:-disabled}"

echo "starting data-plane on :${PORT} with echo tool ${NEXUS_TOOL_ECHO_URL}, control-plane ${control_plane_url} and control-workers ${control_workers_url}"
cd "${DATA_PLANE_DIR}"
exec go run ./cmd/api
