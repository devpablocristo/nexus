#!/usr/bin/env bash
# Verificar calidad del código Go del API
set -euo pipefail

cd "$(dirname "$0")/../../decision-plane"

echo "=== go build ==="
go build ./...

echo "=== go vet ==="
go vet ./...

echo "=== go test ==="
go test ./... -count=1 -race

echo ""
echo "Quality checks passed."
