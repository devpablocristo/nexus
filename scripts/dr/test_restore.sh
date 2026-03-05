#!/usr/bin/env bash
set -euo pipefail

# Tests RDS snapshot restore to a temporary instance
# Usage: ./test_restore.sh <snapshot-id> <temp-instance-id>

SNAPSHOT_ID="${1:?Usage: $0 <snapshot-id> <temp-instance-id>}"
TEMP_INSTANCE="${2:-nexus-restore-test-$(date +%Y%m%d)}"
DB_CLASS="${DB_CLASS:-db.t3.micro}"

echo "Restoring snapshot $SNAPSHOT_ID to $TEMP_INSTANCE..."

aws rds restore-db-instance-from-db-snapshot \
  --db-instance-identifier "$TEMP_INSTANCE" \
  --db-snapshot-identifier "$SNAPSHOT_ID" \
  --db-instance-class "$DB_CLASS" \
  --no-multi-az \
  --tags Key=Purpose,Value=restore-test Key=AutoDelete,Value=true

echo "Waiting for instance to become available..."
aws rds wait db-instance-available --db-instance-identifier "$TEMP_INSTANCE"

ENDPOINT=$(aws rds describe-db-instances \
  --db-instance-identifier "$TEMP_INSTANCE" \
  --query 'DBInstances[0].Endpoint.Address' --output text)

echo "Restore test instance available at: $ENDPOINT"
echo "Running basic connectivity check..."

PGPASSWORD="$DB_PASSWORD" psql -h "$ENDPOINT" -U "$DB_USER" -d nexus_core -c "SELECT COUNT(*) FROM tools;" && echo "Core DB: OK"
PGPASSWORD="$DB_PASSWORD" psql -h "$ENDPOINT" -U "$DB_USER" -d nexus_saas -c "SELECT COUNT(*) FROM tenant_settings;" && echo "SaaS DB: OK"

echo "Cleaning up test instance..."
aws rds delete-db-instance \
  --db-instance-identifier "$TEMP_INSTANCE" \
  --skip-final-snapshot

echo "Restore test completed successfully."
