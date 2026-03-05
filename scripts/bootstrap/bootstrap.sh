#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
NAME
    bootstrap.sh — one-command setup: .env, stack, migrations, seed

SYNOPSIS
    bootstrap.sh [-h|--help]

DESCRIPTION
    Full bootstrap in one shot:
      1. Copies .env.example → .env (if missing)
      2. make up       (docker compose up --build)
      3. make migrate-up
      4. make seed

    After running, the stack is ready for demo/e2e scripts.

PREREQUISITES
    Docker, Make, and the repo root at $PWD.

EXAMPLES
    bash scripts/bootstrap/bootstrap.sh
EOF
  exit 0
}
[[ "${1:-}" =~ ^(-h|--help)$ ]] && usage

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

cp -n .env.example .env || true
make up
make migrate-up
make seed

echo "Bootstrap completed."
