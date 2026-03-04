#!/usr/bin/env bash
set -euo pipefail

DB_URL="${1:-postgres://postgres:postgres@localhost:5432/nexus?sslmode=disable}"
COMPOSE_FILE="${NEXUS_COMPOSE_FILE:-docker-compose.yml}"
compose() { docker compose -f "$COMPOSE_FILE" "$@"; }

for i in {1..60}; do
  if compose exec -T postgres psql "$DB_URL" -c "select 1" >/dev/null 2>&1; then
    exit 0
  fi
  sleep 1
done

echo "DB not ready" >&2
exit 1
