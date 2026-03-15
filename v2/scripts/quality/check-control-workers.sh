#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd go
require_cmd golangci-lint

cd "${CONTROL_WORKERS_DIR}"

go test ./...
go vet ./...
golangci-lint run ./...
