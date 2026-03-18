#!/usr/bin/env bash
# Ejecutar el decision-plane localmente contra el postgres de docker compose
set -euo pipefail

export DATABASE_URL="${DATABASE_URL:-postgres://postgres:postgres@localhost:15434/nexus_decision_plane?sslmode=disable}"
export NEXUS_API_KEYS="${NEXUS_API_KEYS:-admin=nexus-decision-plane-admin-dev-key}"
export PORT="${PORT:-8080}"

cd "$(dirname "$0")/../../decision-plane"
echo "Starting nexus-decision-plane on :$PORT..."
go run ./cmd/api/
