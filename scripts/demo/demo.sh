#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
NAME
    demo.sh — 5-minute guided demo of Nexus gateway features

SYNOPSIS
    demo.sh [-h|--help]

DESCRIPTION
    Interactive walkthrough that demonstrates key Nexus capabilities
    against a running stack. Each step prints a header and the JSON
    response from nexus-core:

      1. Health check
      2. Egress allowlist setup (echo + transfer → mock-tools)
      3. DLP deny: credit card sent to external tool
      4. WRITE with idempotency + timeout budget
      5. Idempotency replay (no upstream re-execution)
      6. Audit export (JSONL with hash-chain)
      7. SSRF/egress protection (block cloud metadata 169.254.169.254)

ENVIRONMENT
    NEXUS_HTTP_PORT   Core HTTP port                 (default: 8080)
    NEXUS_API_KEY     API key from seed output       (required)

PREREQUISITES
    Stack running (make up), migrations applied (make migrate-up),
    seed done (make seed). NEXUS_API_KEY must be set.

EXAMPLES
    export NEXUS_API_KEY="nexus-core-local-key"
    bash scripts/demo/demo.sh
EOF
  exit 0
}
[[ "${1:-}" =~ ^(-h|--help)$ ]] && usage

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BASE="http://localhost:${NEXUS_HTTP_PORT:-8080}"
: "${NEXUS_API_KEY:?Set NEXUS_API_KEY from seed output}"

hdr() { printf "\n=== %s ===\n" "$1"; }
acurl() { curl -sS -H "X-NEXUS-CORE-KEY: ${NEXUS_API_KEY}" "$@"; }

# 1) Health check
hdr "1. Health check"
acurl "${BASE}/healthz" | jq

# 2) Allow egress to mock-tools (default-deny blocks everything)
hdr "2. Allow egress to mock-tools for echo + transfer"
printf "echo egress: "
acurl -o /dev/null -w "%{http_code}" \
  -H "Content-Type: application/json" \
  -d '{"host":"mock-tools","enabled":true}' \
  "${BASE}/v1/tools/echo/egress-rules"
echo

printf "transfer egress: "
acurl -o /dev/null -w "%{http_code}" \
  -H "Content-Type: application/json" \
  -d '{"host":"mock-tools","enabled":true}' \
  "${BASE}/v1/tools/transfer/egress-rules"
echo

# 3) DLP + external classification deny
hdr "3. DLP deny: credit card to external tool"
acurl -H "Content-Type: application/json" \
  -d '{"tool_name":"transfer","input":{"amount":500,"card_number":"4111111111111111"},"context":{"user_id":"u_1"}}' \
  "${BASE}/v1/run" | jq

# 4) WRITE with idempotency + timeout
hdr "4. WRITE with idempotency + timeout"
acurl -H "Idempotency-Key: demo-transfer-001" \
  -H "X-Timeout-Ms: 10000" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"transfer","input":{"amount":500},"context":{"user_id":"u_1"}}' \
  "${BASE}/v1/run" | jq

# 5) Replay same idempotency key
hdr "5. Replay (same key, no upstream re-execution)"
acurl -H "Idempotency-Key: demo-transfer-001" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"transfer","input":{"amount":500},"context":{"user_id":"u_1"}}' \
  "${BASE}/v1/run" | jq

# 6) Audit export with hash-chain
hdr "6. Audit export (JSONL, hash-chain)"
acurl "${BASE}/v1/audit/export?format=jsonl&tool_name=transfer&limit=5"
echo

# 7) (Optional) SSRF protection demo
# Note: if SSRF protection is disabled (NEXUS_DISABLE_SSRF_PROTECTION=true),
# this call is still expected to be blocked by egress default-deny unless you
# explicitly allowlist the IP/host.
hdr "7. SSRF/egress protection: block cloud metadata endpoint"
echo "Creating tool pointing to 169.254.169.254..."
acurl -H "Content-Type: application/json" \
  -d '{"name":"ssrf-test","kind":"http","method":"GET","url":"http://169.254.169.254/latest/meta-data/","input_schema":{"type":"object"},"action_type":"read","risk_level":5,"enabled":true}' \
  "${BASE}/v1/tools" | jq

echo "Calling ssrf-test (should be blocked)..."
acurl -H "Content-Type: application/json" \
  -d '{"tool_name":"ssrf-test","input":{}}' \
  "${BASE}/v1/run" | jq

echo
echo "Demo complete."
