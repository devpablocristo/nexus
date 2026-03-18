#!/usr/bin/env bash
# E2E: ciclo completo request → policy eval → approval → execution → replay → learning
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"

echo "=== E2E: full lifecycle ==="

wait_for_http "$API_BASE/healthz"

# Setup: crear policy
POLICY=$(api_post "/v1/policies" '{"name":"e2e-require-approval","expression":"request.action_type == '\''runbook.execute'\''","effect":"require_approval","priority":5,"enabled":true}')
POLICY_ID=$(echo "$POLICY" | json_get 'id')
pass "Setup: policy created"

# 1. Submit request que matchea policy
R=$(api_post "/v1/requests" '{"requester_type":"service","requester_id":"deploy-svc","action_type":"runbook.execute","target_system":"internal","target_resource":"restart-api-gateway","reason":"memory leak detected","context":"RSS 4.2GB, threshold 4GB"}')
REQUEST_ID=$(echo "$R" | json_get 'request_id')
DECISION=$(echo "$R" | json_get 'decision')
APPROVAL_ID=$(echo "$R" | json_get 'approval.id')
[ "$DECISION" = "require_approval" ] && pass "1. Request requires approval" || fail "Expected require_approval"

# 2. Verificar que aparece en pending approvals
PENDING=$(api_get "/v1/approvals/pending")
FOUND=$(echo "$PENDING" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; print(any(a['id']=='$APPROVAL_ID' for a in d))")
[ "$FOUND" = "True" ] && pass "2. Found in pending approvals" || fail "Not in pending"

# 3. Verificar request status
REQ=$(api_get "/v1/requests/$REQUEST_ID")
STATUS=$(echo "$REQ" | json_get 'status')
[ "$STATUS" = "pending_approval" ] && pass "3. Request status: pending_approval" || fail "Expected pending_approval, got $STATUS"

# 4. Approve
api_post "/v1/approvals/$APPROVAL_ID/approve" '{"decided_by":"e2e-admin@co","note":"E2E test approval"}'
pass "4. Approved"

# 5. Verificar request cambió a approved
REQ=$(api_get "/v1/requests/$REQUEST_ID")
STATUS=$(echo "$REQ" | json_get 'status')
[ "$STATUS" = "approved" ] && pass "5. Request status: approved" || fail "Expected approved, got $STATUS"

# 6. Report result
api_post "/v1/requests/$REQUEST_ID/result" '{"success":true,"result":{"restart":"completed"},"duration_ms":2500}'
pass "6. Result reported"

# 7. Verificar request cambió a executed
REQ=$(api_get "/v1/requests/$REQUEST_ID")
STATUS=$(echo "$REQ" | json_get 'status')
[ "$STATUS" = "executed" ] && pass "7. Request status: executed" || fail "Expected executed, got $STATUS"

# 8. Replay completo
REPLAY=$(api_get "/v1/requests/$REQUEST_ID/replay")
FINAL=$(echo "$REPLAY" | json_get 'final_status')
EVENTS=$(echo "$REPLAY" | json_get 'len(timeline)')
[ "$FINAL" = "executed" ] && pass "8. Replay final_status: executed" || fail "Expected executed"
[ "$EVENTS" -ge 4 ] && pass "   Replay has $EVENTS events" || fail "Expected >= 4 events"

# 9. Dashboard refleja los datos
METRICS=$(api_get "/v1/metrics/summary")
TOTAL=$(echo "$METRICS" | json_get 'total_requests')
[ "$TOTAL" -ge 1 ] && pass "9. Dashboard: $TOTAL requests" || fail "Dashboard empty"

# 10. Learning analyze (no debería generar propuestas con tan pocas requests)
ANALYZE=$(api_post "/v1/learning/analyze" '{}')
CREATED=$(echo "$ANALYZE" | json_get 'proposals_created')
pass "10. Learning analyze: $CREATED proposals"

# 11. Idempotency
R_IDEM1=$(api_post "/v1/requests" '{"requester_type":"agent","requester_id":"idem-bot","action_type":"alert.escalate"}' | json_get 'request_id')
# Necesitamos header Idempotency-Key para la segunda
R_IDEM2=$(curl -sf -X POST -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" -H "Idempotency-Key: e2e-idem-key" \
  -d '{"requester_type":"agent","requester_id":"idem-bot","action_type":"alert.escalate"}' "$API_BASE/v1/requests" | json_get 'request_id')
R_IDEM3=$(curl -sf -X POST -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" -H "Idempotency-Key: e2e-idem-key" \
  -d '{"requester_type":"agent","requester_id":"idem-bot","action_type":"alert.escalate"}' "$API_BASE/v1/requests" | json_get 'request_id')
[ "$R_IDEM2" = "$R_IDEM3" ] && pass "11. Idempotency: same ID" || fail "Idempotency failed: $R_IDEM2 vs $R_IDEM3"

# Cleanup
api_delete "/v1/policies/$POLICY_ID" > /dev/null

echo ""
green "=== E2E full lifecycle passed ==="
