#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

cp -n .env.example .env || true
make up
make migrate-up
make seed

echo "Bootstrap completed."
