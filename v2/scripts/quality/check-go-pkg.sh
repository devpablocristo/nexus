#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/common.sh
source "${SCRIPT_DIR}/../lib/common.sh"

require_cmd go
require_cmd golangci-lint

cd "${V2_ROOT}/pkgs/go-pkg"

go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
