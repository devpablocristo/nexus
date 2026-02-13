#!/usr/bin/env bash
set -euo pipefail

DB_URL="${1:-postgres://postgres:postgres@localhost:5432/nexus?sslmode=disable}"

for i in {1..60}; do
  if docker compose exec -T postgres psql "$DB_URL" -c "select 1" >/dev/null 2>&1; then
    exit 0
  fi
  sleep 1
done

echo "DB not ready" >&2
exit 1

