#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
NAME
    wait-for-db.sh — block until PostgreSQL is ready (max 60s)

SYNOPSIS
    wait-for-db.sh [-h|--help] [DB_URL]

DESCRIPTION
    Polls the database with "SELECT 1" every second for up to 60 seconds.
    Exits 0 when the database responds, or 1 on timeout.
    Used internally by seed and e2e scripts.

ARGUMENTS
    DB_URL   PostgreSQL connection string
             (default: postgres://postgres:postgres@localhost:5432/nexus?sslmode=disable)

ENVIRONMENT
    NEXUS_COMPOSE_FILE   Compose file for docker exec (default: docker-compose.yml)

EXAMPLES
    bash scripts/db/wait-for-db.sh
    bash scripts/db/wait-for-db.sh "postgres://user:pass@host:5432/mydb"
EOF
  exit 0
}
[[ "${1:-}" =~ ^(-h|--help)$ ]] && usage

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
