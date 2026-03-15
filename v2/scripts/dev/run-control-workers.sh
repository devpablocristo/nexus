#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd go

PORT="${PORT:-8082}"
export PORT

echo "starting control-workers on :${PORT}"
cd "${CONTROL_WORKERS_DIR}"
exec go run ./cmd/api
