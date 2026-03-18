#!/usr/bin/env bash
# Ejecutar review localmente contra el postgres de docker compose
set -euo pipefail

export DATABASE_URL="${DATABASE_URL:-postgres://postgres:postgres@localhost:15434/nexus_review?sslmode=disable}"
export NEXUS_API_KEYS="${NEXUS_API_KEYS:-admin=nexus-review-admin-dev-key}"
export PORT="${PORT:-8080}"

cd "$(dirname "$0")/../../review"
echo "Starting nexus-review on :$PORT..."
go run ./cmd/api/
