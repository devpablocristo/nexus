#!/usr/bin/env bash
set -euo pipefail

# Tests RDS snapshot restore to a temporary instance and writes machine-readable evidence.
# Usage: ./test_restore.sh <snapshot-id> [temp-instance-id]

SNAPSHOT_ID="${1:?Usage: $0 <snapshot-id> [temp-instance-id]}"
TEMP_INSTANCE="${2:-nexus-restore-test-$(date +%Y%m%d-%H%M%S)}"
DB_CLASS="${DB_CLASS:-db.t3.micro}"
RESTORE_EVIDENCE_ENV="${RESTORE_EVIDENCE_ENV:-prod}"
RESTORE_EVIDENCE_SYSTEM="${RESTORE_EVIDENCE_SYSTEM:-database}"
RESTORE_EVIDENCE_SOURCE="${RESTORE_EVIDENCE_SOURCE:-scripts/dr/test_restore.sh}"
RESTORE_EVIDENCE_FILE="${RESTORE_EVIDENCE_FILE:-${PWD}/${TEMP_INSTANCE}-restore-evidence.json}"
KEEP_RESTORE_INSTANCE="${KEEP_RESTORE_INSTANCE:-false}"
STARTED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

STATUS="failed"
ENDPOINT=""
INSTANCE_CREATED="false"
CORE_OK="false"
SAAS_OK="false"
ARTIFACT_SHA256=""

require_env() {
  local var_name="$1"
  if [[ -z "${!var_name:-}" ]]; then
    echo "Missing required env var: ${var_name}" >&2
    exit 1
  fi
}

write_evidence() {
  local completed_at
  completed_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  export SNAPSHOT_ID TEMP_INSTANCE RESTORE_EVIDENCE_ENV RESTORE_EVIDENCE_SYSTEM RESTORE_EVIDENCE_SOURCE
  export STARTED_AT STATUS ENDPOINT CORE_OK SAAS_OK completed_at
  python3 - "$RESTORE_EVIDENCE_FILE" <<'PY'
import json
import os
import sys

payload = {
    "snapshot_id": os.environ["SNAPSHOT_ID"],
    "restore_target": os.environ["TEMP_INSTANCE"],
    "environment": os.environ["RESTORE_EVIDENCE_ENV"],
    "system": os.environ["RESTORE_EVIDENCE_SYSTEM"],
    "status": os.environ["STATUS"],
    "source": os.environ["RESTORE_EVIDENCE_SOURCE"],
    "started_at": os.environ["STARTED_AT"],
    "completed_at": os.environ["completed_at"],
    "endpoint": os.environ.get("ENDPOINT", ""),
    "checks": {
        "nexus_core": os.environ.get("CORE_OK", "false") == "true",
        "nexus_saas": os.environ.get("SAAS_OK", "false") == "true",
    },
}
with open(sys.argv[1], "w", encoding="utf-8") as handle:
    json.dump(payload, handle, indent=2, sort_keys=True)
    handle.write("\n")
PY
  ARTIFACT_SHA256="$(sha256sum "$RESTORE_EVIDENCE_FILE" | awk '{print $1}')"
}

publish_evidence() {
  if [[ -z "${NEXUS_SAAS_URL:-}" || -z "${NEXUS_SAAS_INTERNAL_KEY:-}" || -z "${RESTORE_EVIDENCE_ORG_ID:-}" ]]; then
    return 0
  fi
  local publish_payload
  publish_payload="$(mktemp)"
  python3 - "$RESTORE_EVIDENCE_FILE" "$publish_payload" <<'PY'
import json
import os
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    payload = json.load(handle)

payload["org_id"] = os.environ["RESTORE_EVIDENCE_ORG_ID"]
payload["artifact_sha256"] = os.environ.get("ARTIFACT_SHA256", "")
payload["summary"] = {
    "checks": payload.get("checks", {}),
    "endpoint": payload.get("endpoint", ""),
}

with open(sys.argv[2], "w", encoding="utf-8") as handle:
    json.dump(payload, handle)
PY
  if ! curl -fsS \
    -X POST \
    -H "Content-Type: application/json" \
    -H "X-NEXUS-SAAS-KEY: ${NEXUS_SAAS_INTERNAL_KEY}" \
    --data-binary @"$publish_payload" \
    "${NEXUS_SAAS_URL%/}/internal/restore-evidence" >/dev/null; then
    echo "Warning: failed to publish restore evidence to SaaS" >&2
  fi
  rm -f "$publish_payload"
}

cleanup() {
  write_evidence
  export ARTIFACT_SHA256
  publish_evidence
  if [[ "$INSTANCE_CREATED" == "true" && "$KEEP_RESTORE_INSTANCE" != "true" ]]; then
    echo "Cleaning up test instance..."
    aws rds delete-db-instance \
      --db-instance-identifier "$TEMP_INSTANCE" \
      --skip-final-snapshot >/dev/null
  fi
}

trap cleanup EXIT

require_env DB_USER
require_env DB_PASSWORD

echo "Restoring snapshot $SNAPSHOT_ID to $TEMP_INSTANCE..."

aws rds restore-db-instance-from-db-snapshot \
  --db-instance-identifier "$TEMP_INSTANCE" \
  --db-snapshot-identifier "$SNAPSHOT_ID" \
  --db-instance-class "$DB_CLASS" \
  --no-multi-az \
  --tags Key=Purpose,Value=restore-test Key=AutoDelete,Value=true

INSTANCE_CREATED="true"

echo "Waiting for instance to become available..."
aws rds wait db-instance-available --db-instance-identifier "$TEMP_INSTANCE"

ENDPOINT="$(aws rds describe-db-instances \
  --db-instance-identifier "$TEMP_INSTANCE" \
  --query 'DBInstances[0].Endpoint.Address' --output text)"

echo "Restore test instance available at: $ENDPOINT"
echo "Running connectivity checks..."

if PGPASSWORD="$DB_PASSWORD" psql -h "$ENDPOINT" -U "$DB_USER" -d nexus_core -c "SELECT COUNT(*) FROM tools;" >/dev/null; then
  CORE_OK="true"
  echo "Core DB: OK"
fi

if PGPASSWORD="$DB_PASSWORD" psql -h "$ENDPOINT" -U "$DB_USER" -d nexus_saas -c "SELECT COUNT(*) FROM tenant_settings;" >/dev/null; then
  SAAS_OK="true"
  echo "SaaS DB: OK"
fi

if [[ "$CORE_OK" != "true" || "$SAAS_OK" != "true" ]]; then
  echo "Restore connectivity checks failed" >&2
  exit 1
fi

STATUS="passed"
echo "Restore test completed successfully."
echo "Evidence written to $RESTORE_EVIDENCE_FILE"
