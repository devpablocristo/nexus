#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

PORT="${PORT:-18081}"

echo "starting echo upstream on 127.0.0.1:${PORT}"
start_echo_upstream "${PORT}"
