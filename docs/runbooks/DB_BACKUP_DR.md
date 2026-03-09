# DB Backup And DR Evidence

## Goal

Validate that production database restores are periodically tested and leave machine-readable evidence that Nexus can use during sensitive preflights.

## Evidence Flow

1. Run `scripts/dr/test_restore.sh <snapshot-id> [temp-instance-id]`.
2. The script restores the snapshot to a temporary RDS instance.
3. It checks connectivity against `nexus_core` and `nexus_saas`.
4. It writes a JSON artifact with timestamps, snapshot id, target instance, endpoint, and check results.
5. If `NEXUS_SAAS_URL`, `NEXUS_SAAS_INTERNAL_KEY`, and `RESTORE_EVIDENCE_ORG_ID` are configured, it publishes that artifact to `nexus-saas` internal contracts.

## Required Environment

- `DB_USER`
- `DB_PASSWORD`
- optional: `NEXUS_SAAS_URL`
- optional: `NEXUS_SAAS_INTERNAL_KEY`
- optional: `RESTORE_EVIDENCE_ORG_ID`
- optional: `RESTORE_EVIDENCE_ENV` (`prod` by default)
- optional: `RESTORE_EVIDENCE_SYSTEM` (`database` by default)
- optional: `RESTORE_EVIDENCE_FILE`

## Operational Rule

- Sensitive production preflights should require a recent successful restore evidence artifact.
- If restore evidence is missing or stale, the execution must be blocked until DR validation is refreshed.

## Retention

- Keep the JSON artifact alongside the CI/job logs or upload it to your artifact store.
- The SaaS registry keeps the structured record needed by Nexus runtime checks.
# Database Backup & Disaster Recovery

## Scope

This runbook covers backup and recovery procedures for Nexus production databases:
- `production-nexus-core` (DB `nexus`)
- `production-nexus-saas` (DB `nexus_saas`)

Infrastructure assumptions (Terraform-managed):
- Engine: PostgreSQL 16.x on Amazon RDS
- Automated backups: enabled
- PITR: enabled
- Encryption at rest: enabled
- Multi-AZ: production only

## Automated Backups (RDS)

- Retention: `7 days`
- Backup window: `03:00-04:00 UTC` daily
- PITR granularity: approximately `5 minutes`
- Final snapshot on deletion: enabled
- Copy tags to snapshots: enabled

## Recovery Procedures

### Restore to Point in Time (PITR)

1. Go to **RDS Console** -> select source instance.
2. Choose **Actions -> Restore to point in time**.
3. Select restore timestamp (up to ~5 minutes before now).
4. Configure restored instance identifier and networking.
5. Launch restore and wait until status is `available`.
6. Update service config / ECS task definitions to the new endpoint.
7. Run smoke tests and health checks.

CLI example:

```bash
aws rds restore-db-instance-to-point-in-time \
  --source-db-instance-identifier production-nexus-core \
  --target-db-instance-identifier production-nexus-core-restored \
  --restore-time "2026-03-05T10:30:00Z"
```

### Restore from Snapshot

1. Open **RDS Console -> Snapshots**.
2. Select desired snapshot.
3. Click **Restore snapshot**.
4. Configure instance class, subnet group, SGs, and identifier.
5. Wait for `available` state.
6. Point applications to restored endpoint and validate.

### Manual Snapshot Before Risky Change

Always snapshot before high-risk migrations/releases.

```bash
aws rds create-db-snapshot \
  --db-instance-identifier production-nexus-core \
  --db-snapshot-identifier manual-pre-migration-$(date +%Y%m%d-%H%M)

aws rds create-db-snapshot \
  --db-instance-identifier production-nexus-saas \
  --db-snapshot-identifier manual-pre-migration-saas-$(date +%Y%m%d-%H%M)
```

## Multi-AZ Failover

In production, RDS Multi-AZ is enabled.

Expected behavior:
- Automatic failover on AZ/instance failure
- Typical failover time: 2-5 minutes
- Endpoint hostname remains the same

Post-failover checks:
1. Validate `GET /readyz` on `nexus-core`.
2. Validate `GET /health` on `nexus-saas`.
3. Confirm error rates return to baseline.

## Monitoring & Alerting

CloudWatch alarms configured:
- `RDS CPUUtilization > 80%`
- `RDS FreeStorageSpace < 5GB`
- `ALB 5xx > 10 / 5 min`
- `ECS CPU > 80%`
- `ECS Memory > 80%`

Actions:
- SNS topic fan-out to on-call email (`alert_email` Terraform variable)
- Incident should be created in operational channel with timeline

## Validation Checklist After Recovery

1. DB instance status is `available`.
2. App services healthy (`/readyz`, `/health`).
3. Read/write checks succeed on both DBs.
4. No sustained 5xx at ALB.
5. Event ingestion and billing counters continue incrementing.
6. Audit queries and admin UI are functional.

## RTO / RPO Targets

| Metric | Target |
|--------|--------|
| RPO (max data loss) | <= 5 minutes (PITR) |
| RTO (restore time)  | <= 30 minutes |

## Periodic Restore Test

Run a restore validation at least monthly using a temporary instance:

```bash
./scripts/dr/test_restore.sh <snapshot-id> <temp-instance-id>
```

Requirements:
- AWS credentials with `rds:RestoreDBInstanceFromDBSnapshot` and `rds:DeleteDBInstance`
- `psql` installed locally
- `DB_USER` and `DB_PASSWORD` exported in shell

Validation:
1. Script restores snapshot to a temporary DB instance.
2. Script waits for `available` status.
3. Script runs basic connectivity checks on both `nexus_core` and `nexus_saas`.
4. Script deletes the temporary instance (`--skip-final-snapshot`).

## Complete DB Failure Runbook

1. Detect failure from CloudWatch alarm or failed health checks.
2. If Multi-AZ enabled, allow automatic failover first.
3. If still unavailable, restore from PITR or latest snapshot.
4. Update runtime endpoints/secrets if restored to new instance.
5. Run pending migrations (if needed):

```bash
make migrate-up
```

6. Perform smoke tests:

```bash
curl -f https://api.nexus.example.com/readyz
curl -f https://api.nexus.example.com/health
```

7. Communicate status and recovery timeline to stakeholders.
8. Open postmortem with root cause and follow-up actions.
