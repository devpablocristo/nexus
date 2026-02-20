#!/usr/bin/env bash
set -euo pipefail

cp -n .env.example .env || true
make up
make migrate-up
make seed

echo "Bootstrap completed."
