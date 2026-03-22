#!/usr/bin/env bash
# Smoke: Companion crea task → propose → Review persiste request y Companion guarda vínculo.
# Requiere: docker compose up (review + companion + postgres), migraciones aplicadas,
# action_type companion.propose (migración Review 0009).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=../lib/common.sh
source "$SCRIPT_DIR/../lib/common.sh"

echo "=== Smoke: Companion → Review flow ==="

wait_for_http "$API_BASE/healthz"
wait_for_http "$COMPANION_BASE/readyz"
pass "Review and Companion are up"

echo "Creating Companion task..."
CREATE_BODY=$(companion_post "/v1/tasks" "{\"title\":\"smoke-companion-$(date +%s)\",\"goal\":\"smoke e2e\",\"created_by\":\"smoke-script\"}")
TASK_ID=$(echo "$CREATE_BODY" | json_get 'id')
[ -n "$TASK_ID" ] && pass "Task created: $TASK_ID" || fail "No task id in response"

echo "Proposing to Review..."
PROP=$(companion_post "/v1/tasks/$TASK_ID/propose" '{"note":"smoke propose"}')
REQ_ID=$(echo "$PROP" | json_get 'review_submit.request_id')
[ -n "$REQ_ID" ] && pass "Propose returned review_request_id: $REQ_ID" || fail "No review_submit.request_id"

echo "Verifying request exists in Review..."
RR=$(api_get "/v1/requests/$REQ_ID")
RR_ID=$(echo "$RR" | json_get 'id')
[ "$RR_ID" = "$REQ_ID" ] && pass "Review GET request matches" || fail "Review request id mismatch: $RR_ID vs $REQ_ID"

echo "Verifying Companion task detail links action to Review..."
DETAIL=$(companion_get "/v1/tasks/$TASK_ID")
if echo "$DETAIL" | python3 -c "
import json, sys
d = json.load(sys.stdin)
rid = sys.argv[1]
acts = d.get('actions') or []
ok = any(a.get('review_request_id') == rid for a in acts)
sys.exit(0 if ok else 1)
" "$REQ_ID"; then
  pass "Task detail contains action with review_request_id"
else
  fail "Task detail missing review_request_id on action"
fi

TASK_ST=$(echo "$DETAIL" | json_get 'task.status')
[ "$TASK_ST" = "waiting_for_approval" ] && pass "Task status waiting_for_approval" || fail "Unexpected task status: $TASK_ST (expected waiting_for_approval)"

echo ""
green "=== Companion ↔ Review smoke passed ==="
