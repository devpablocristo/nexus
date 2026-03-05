#!/usr/bin/env bash
set -u

# Usage: ./smoke_prod.sh [API_BASE_URL] [SAAS_BASE_URL] [TOWER_URL]
# Example: ./smoke_prod.sh https://api.nexus.io https://saas.nexus.io https://app.nexus.io

API_URL="${1:-http://localhost:8080}"
SAAS_URL="${2:-http://localhost:8082}"
TOWER_URL="${3:-http://localhost:5174}"

if [ -f ".env" ]; then
  # shellcheck disable=SC1091
  set -a && . ./.env && set +a
fi

API_KEY="${NEXUS_API_KEY:-${VITE_NEXUS_API_KEY:-dev-api-key}}"
SCOPES="${NEXUS_SCOPES:-${VITE_NEXUS_SCOPES:-tools:read,gateway:run,admin:console:read}}"
if ! printf "%s" "$SCOPES" | grep -q "tools:read"; then
  SCOPES="${SCOPES},tools:read"
fi

PASS_COUNT=0
FAIL_COUNT=0

print_result() {
  local status="$1"
  local name="$2"
  local detail="$3"
  if [ "$status" = "PASS" ]; then
    PASS_COUNT=$((PASS_COUNT + 1))
    printf "[PASS] %-45s %s\n" "$name" "$detail"
  else
    FAIL_COUNT=$((FAIL_COUNT + 1))
    printf "[FAIL] %-45s %s\n" "$name" "$detail"
  fi
}

http_check() {
  local name="$1"
  local url="$2"
  local expected_code="$3"
  local contains="$4"
  shift 4

  local body_file
  body_file="$(mktemp)"
  local code
  code="$(curl -sS -L -o "$body_file" -w '%{http_code}' "$url" "$@" 2>/dev/null || true)"

  if [ "$code" != "$expected_code" ]; then
    print_result "FAIL" "$name" "expected HTTP $expected_code, got $code"
    rm -f "$body_file"
    return
  fi

  if [ -n "$contains" ] && ! grep -q "$contains" "$body_file"; then
    print_result "FAIL" "$name" "HTTP $code but missing pattern: $contains"
    rm -f "$body_file"
    return
  fi

  print_result "PASS" "$name" "HTTP $code"
  rm -f "$body_file"
}

header_check() {
  local name="$1"
  local url="$2"
  local h1="$3"
  local h2="$4"

  local headers
  headers="$(curl -sS -I "$url" 2>/dev/null || true)"

  if echo "$headers" | grep -qi "^$h1:" && echo "$headers" | grep -qi "^$h2:"; then
    print_result "PASS" "$name" "$h1 + $h2 present"
  else
    print_result "FAIL" "$name" "missing $h1 and/or $h2"
  fi
}

printf "Running smoke checks:\n"
printf "  API:   %s\n" "$API_URL"
printf "  SAAS:  %s\n" "$SAAS_URL"
printf "  TOWER: %s\n\n" "$TOWER_URL"

http_check "Core readiness" "$API_URL/readyz" "200" ""
http_check "SaaS health" "$SAAS_URL/health" "200" ""
http_check "Tower index HTML" "$TOWER_URL/" "200" "<!doctype html"
http_check "Core metrics exposed" "$API_URL/metrics" "200" "nexus_"
http_check "SaaS metrics exposed" "$SAAS_URL/metrics" "200" "nexus_saas_"
http_check "Core tools list with API key" "$API_URL/v1/tools" "200" "\"items\"" \
  -H "X-NEXUS-CORE-KEY: $API_KEY" \
  -H "X-NEXUS-SCOPES: $SCOPES" \
  -H "X-NEXUS-ACTOR: smoke-script"
http_check "SaaS Swagger UI" "$SAAS_URL/docs" "200" "SwaggerUIBundle"
header_check "Tower security headers" "$TOWER_URL/" "X-Content-Type-Options" "Content-Security-Policy"

printf "\nSummary: %d passed, %d failed\n" "$PASS_COUNT" "$FAIL_COUNT"

if [ "$FAIL_COUNT" -gt 0 ]; then
  exit 1
fi

exit 0
