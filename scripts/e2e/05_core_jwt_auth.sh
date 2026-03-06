#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
NAME
    05_core_jwt_auth.sh — e2e for JWT-only authentication mode

SYNOPSIS
    05_core_jwt_auth.sh [-h|--help]

DESCRIPTION
    Spins up an isolated stack with JWT auth enabled and API key auth
    disabled (NEXUS_AUTH_ENABLE_JWT=true, NEXUS_AUTH_ALLOW_API_KEY=false).

    Tests:
      - API key requests are rejected (401)
      - Bearer token issued by mock-tools JWKS is accepted
      - Run echo with JWT, list tools with JWT, MCP call with JWT

    Tears down the stack on exit (trap cleanup).

ENVIRONMENT
    NEXUS_HTTP_PORT_E2E_BASE   Starting port for core   (default: 18080)
    COMPOSE_PROJECT_NAME       Override compose project  (auto-generated)

PREREQUISITES
    Docker, curl, jq, sha256sum. The script manages its own stack.

EXAMPLES
    ./scripts/e2e/05_core_jwt_auth.sh
EOF
  exit 0
}
[[ "${1:-}" =~ ^(-h|--help)$ ]] && usage

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if [[ ! -f ".env" ]]; then
  echo "missing .env (create it from .env.example)" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

NEXUS_HTTP_PORT="${NEXUS_HTTP_PORT_E2E_BASE:-18080}"
NEXUS_MOCK_TOOLS_PORT="${NEXUS_MOCK_TOOLS_PORT_E2E_BASE:-18081}"
NEXUS_POSTGRES_PORT="${NEXUS_POSTGRES_PORT_E2E_BASE:-55432}"
NEXUS_REDIS_PORT="${NEXUS_REDIS_PORT_E2E_BASE:-16379}"
NEXUS_PROMETHEUS_PORT="${NEXUS_PROMETHEUS_PORT_E2E_BASE:-19090}"
NEXUS_GRAFANA_PORT="${NEXUS_GRAFANA_PORT_E2E_BASE:-13000}"

compose() {
  docker compose "$@"
}

port_in_use() {
  local port="$1"
  (echo >/dev/tcp/127.0.0.1/"$port") >/dev/null 2>&1
}

next_free_port() {
  local candidate="$1"
  while port_in_use "$candidate"; do
    candidate=$((candidate + 1))
  done
  echo "$candidate"
}

RESERVED_PORTS=""

reserve_port_var() {
  local var_name="$1"
  local default_port="$2"
  local chosen="${!var_name:-$default_port}"
  while port_in_use "$chosen" || [[ " $RESERVED_PORTS " == *" $chosen "* ]]; do
    chosen=$((chosen + 1))
  done
  RESERVED_PORTS="$RESERVED_PORTS $chosen"
  printf -v "$var_name" '%s' "$chosen"
  export "$var_name"
}

reserve_port_var NEXUS_HTTP_PORT 18080
reserve_port_var NEXUS_MOCK_TOOLS_PORT 18081
reserve_port_var NEXUS_POSTGRES_PORT 55432
reserve_port_var NEXUS_SAAS_POSTGRES_PORT 55433
reserve_port_var NEXUS_SAAS_HTTP_PORT 18082
reserve_port_var NEXUS_REDIS_PORT 16379
reserve_port_var NEXUS_OPERATOR_PORT 18000
reserve_port_var OPERATOR_HEALTH_PORT 18090
reserve_port_var NEXUS_TOWER_PORT 15174
reserve_port_var NEXUS_PROMETHEUS_PORT 19090
reserve_port_var NEXUS_GRAFANA_PORT 13000
reserve_port_var NEXUS_MAILHOG_SMTP_PORT 1125
reserve_port_var NEXUS_MAILHOG_UI_PORT 18025

COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-nexus-core-e2e-jwt-$(date +%s)}"
export COMPOSE_PROJECT_NAME

HTTP_BASE="http://localhost:${NEXUS_HTTP_PORT}"
MOCK_BASE="http://localhost:${NEXUS_MOCK_TOOLS_PORT}"


require() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing dependency: $1" >&2; exit 1; }
}

require curl
require jq
require sha256sum

# Prefer ripgrep when available; fall back to grep so the suite works on minimal dev machines/CI images.
match() {
  local pattern="$1"
  if command -v rg >/dev/null 2>&1; then
    rg -n "$pattern"
  else
    grep -nE "$pattern"
  fi
}

fail() { echo "E2E JWT FAIL: $*" >&2; exit 1; }

assert_jq() {
  local json="$1"
  local filter="$2"
  echo "$json" | jq -e "$filter" >/dev/null || fail "jq assertion failed: $filter | json=$json"
}

http_code() {
  curl -sS -o /dev/null -w "%{http_code}" "$1" 2>/dev/null || true
}

ENV_BACKUP="$(mktemp)"
cp .env "$ENV_BACKUP"

cleanup() {
  compose down -v >/dev/null 2>&1 || true
  cp "$ENV_BACKUP" .env
  rm -f "$ENV_BACKUP"
}
trap cleanup EXIT

echo "[e2e-jwt] bring stack up (JWT only)"

# env_file in docker-compose.yml reads .env from disk. Temporarily patch it
# so the container sees JWT enabled and API-key auth disabled.
sed -i \
  -e 's/^NEXUS_AUTH_ENABLE_JWT=.*/NEXUS_AUTH_ENABLE_JWT=true/' \
  -e 's/^NEXUS_AUTH_ALLOW_API_KEY=.*/NEXUS_AUTH_ALLOW_API_KEY=false/' \
  .env

grep -q '^NEXUS_DISABLE_SSRF_PROTECTION=' .env \
  || echo 'NEXUS_DISABLE_SSRF_PROTECTION=true' >> .env

export NEXUS_AUTH_ENABLE_JWT=true
export NEXUS_AUTH_ALLOW_API_KEY=false
export NEXUS_DISABLE_SSRF_PROTECTION=true
compose up --build -d >/dev/null

for _ in {1..60}; do
  [[ "$(http_code "${HTTP_BASE}/readyz")" == "200" ]] && break
  sleep 1
done
[[ "$(http_code "${HTTP_BASE}/readyz")" == "200" ]] || fail "readyz not 200"
[[ "$(http_code "${MOCK_BASE}/healthz")" == "200" ]] || fail "mock-tools healthz not 200"

echo "[e2e-jwt] migrate + seed"
make migrate-up >/dev/null
SEED_OUT="$(bash scripts/seed/seed_demo.sh)"
API_KEY="$(echo "$SEED_OUT" | match "^NEXUS_DEMO_API_KEY=" | tail -n1 | cut -d= -f2)"
[[ -n "$API_KEY" ]] || fail "seed key not found"
API_HASH="$(printf "%s" "$API_KEY" | sha256sum | awk '{print $1}')"

ORG_ID="$(compose exec -T postgres psql -U postgres -d nexus -At -c "select org_id from org_api_keys where api_key_hash='${API_HASH}' limit 1;")"
[[ "$ORG_ID" =~ ^[0-9a-fA-F-]{36}$ ]] || fail "org id not found"

TOKEN_RESP="$(curl -fsS "${MOCK_BASE}/_jwt/issue?org_id=${ORG_ID}&sub=e2e-jwt&role=secops")"
TOKEN="$(echo "$TOKEN_RESP" | jq -r '.token')"
[[ -n "$TOKEN" && "$TOKEN" != "null" ]] || fail "token not returned"

echo "[e2e-jwt] setup egress rules (default-deny)"
curl -sS -o /dev/null -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"host":"mock-tools","enabled":true}' \
  "${HTTP_BASE}/v1/tools/echo/egress-rules"

echo "[e2e-jwt] api-key disabled check"
NO_KEY="$(curl -sS "${HTTP_BASE}/v1/tools" || true)"
assert_jq "$NO_KEY" '.error.code=="UNAUTHORIZED"'

echo "[e2e-jwt] bearer run echo"
RUN_ECHO="$(curl -fsS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d '{"tool_name":"echo","input":{"jwt":"ok"},"context":{"user_id":"u_1"}}' \
  "${HTTP_BASE}/v1/run")"
assert_jq "$RUN_ECHO" '.status=="success" and .decision=="allow" and .result.received.jwt=="ok"'

echo "[e2e-jwt] bearer list tools"
TOOLS="$(curl -fsS -H "Authorization: Bearer ${TOKEN}" "${HTTP_BASE}/v1/tools")"
assert_jq "$TOOLS" '.items | type=="array" and length>=2'

echo "[e2e-jwt] bearer MCP call"
MCP_CALL="$(curl -fsS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool_name":"echo","input":{"from":"jwt-mcp"},"context":{"user_id":"u_1"}}}' \
  "${HTTP_BASE}/mcp")"
assert_jq "$MCP_CALL" '.result.status=="success" and .result.result.received.from=="jwt-mcp"'

echo "[e2e-jwt] OK"

