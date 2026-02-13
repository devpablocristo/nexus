#!/usr/bin/env bash
# 5-Minute Demo — mirrors the README "5-Minute Demo (Copy/Paste)" section.
# Prerequisites: docker compose stack running (make dev), migrations applied, seed done.
# Usage:
#   cp .env.example .env
#   make dev && make migrate-up && make seed
#   export NEXUS_API_KEY="<seed-output-value>"
#   bash scripts/demo.sh
set -euo pipefail

BASE="http://localhost:${NEXUS_HTTP_PORT:-8080}"
: "${NEXUS_API_KEY:?Set NEXUS_API_KEY from seed output}"

hdr() { printf "\n=== %s ===\n" "$1"; }
acurl() { curl -sS -H "X-NEXUS-GATEWAY-KEY: ${NEXUS_API_KEY}" "$@"; }

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
hdr "7. SSRF protection: block cloud metadata endpoint"
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
