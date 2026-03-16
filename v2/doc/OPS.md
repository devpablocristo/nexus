# Nexus v2 Operations Guide

Relacionado:

- [PRE_PROD.md](PRE_PROD.md)
- [PROD_CHECKLIST.md](PROD_CHECKLIST.md)
- [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md)

## Deployment topology

```
                  ┌─────────────┐
                  │   ALB/LB    │
                  └──────┬──────┘
                         │
           ┌─────────────┼─────────────┐
           │             │             │
    ┌──────▼──────┐ ┌────▼─────┐ ┌────▼──────────┐
    │ data-plane  │ │ control  │ │ control       │
    │ :8080       │ │ plane    │ │ workers       │
    │             │ │ :8081    │ │ :8082         │
    └──────┬──────┘ └────┬─────┘ └────┬──────────┘
           │             │             │
           │    ┌────────┘             │
           │    │    ┌─────────────────┘
           ▼    ▼    ▼
    ┌──────────────────────────────────┐
    │       PostgreSQL instances        │
    │  data-plane │ control-plane       │
    │  control-workers │ audit          │
    └──────────────────────────────────┘
```

Inter-service communication:
- data-plane → control-plane (resources, policies, audit)
- data-plane → control-workers (incidents)
- control-workers → control-plane (audit)

All inter-service calls use API key auth and propagate X-Request-Id.

## Image tagging convention

Image names:

- `nexus-data-plane`
- `nexus-control-plane`
- `nexus-control-workers`

Tag format: `<service>:<version>-<sha7>`

Examples:
- `nexus-data-plane:0.1.0-a1b2c3d`
- `nexus-control-plane:0.1.0-a1b2c3d`

Special tags:
- `latest` — last image built on main (not for production)
- `staging` — image currently deployed to staging
- `prod` — image currently deployed to production

Rules:
- every merge to main produces a tagged image
- staging deploys use explicit version tags, not `latest`
- production deploys use the same tag that was validated in staging
- rollback = deploy the previous version tag

Versioning: semver starting at `0.1.0`. Bump minor for features, patch for fixes. No major until 1.0.

## Rollout strategy

Strategy: **rolling update**.

- one instance at a time
- health check via `/healthz` before routing traffic
- readiness check via `/readyz` (includes DB connectivity)
- drain connections before shutdown (graceful shutdown already implemented)

Rollout order:
1. control-plane first (resources and policies must be available)
2. control-workers second
3. data-plane last (depends on both)

## Rollback procedure

Minimum rollback:

1. Revert to previous container image tag
2. Rolling update with same strategy
3. Verify `/readyz` on all services
4. Run `scripts/smoke/run-actions-basic-flow.sh` against the rolled-back deployment

If migrations were applied:
- up-only strategy means no automatic rollback of schema changes
- new migrations must be backward compatible with the previous code version
- if a migration breaks backward compatibility, fix forward (new migration), do not attempt manual DDL rollback

## Configuration per service

### data-plane

| Variable | Required | Description |
|---|---|---|
| `PORT` | no (default 8080) | HTTP listen port |
| `NEXUS_API_KEYS` | yes | comma-separated accepted API keys |
| `NEXUS_CONTROL_PLANE_URL` | no | control-plane base URL |
| `NEXUS_CONTROL_PLANE_API_KEY` | no | API key for control-plane calls |
| `NEXUS_CONTROL_WORKERS_URL` | no | control-workers base URL |
| `NEXUS_CONTROL_WORKERS_API_KEY` | no | API key for control-workers calls |
| `NEXUS_DATA_PLANE_DATABASE_URL` | no | PostgreSQL connection string |
| `NEXUS_DATA_PLANE_DB_MIN_CONNS` | no (default 1) | pool min connections |
| `NEXUS_DATA_PLANE_DB_MAX_CONNS` | no (default 8) | pool max connections |
| `NEXUS_DATA_PLANE_DB_MAX_CONN_LIFETIME` | no (default 30m) | connection max lifetime |
| `NEXUS_DATA_PLANE_DB_MAX_CONN_IDLE_TIME` | no (default 5m) | idle connection timeout |
| `NEXUS_DATA_PLANE_DB_HEALTH_CHECK_PERIOD` | no (default 30s) | pool health check interval |
| `NEXUS_DATA_PLANE_DB_CONNECT_TIMEOUT` | no (default 5s) | connection timeout |
| `NEXUS_DATA_PLANE_DB_STATEMENT_TIMEOUT` | no (default 5s) | query timeout |

### control-plane

| Variable | Required | Description |
|---|---|---|
| `PORT` | no (default 8081) | HTTP listen port |
| `NEXUS_API_KEYS` | yes | comma-separated accepted API keys |
| `NEXUS_CONTROL_PLANE_DATABASE_URL` | no | PostgreSQL connection string (resources + policies) |
| `NEXUS_AUDIT_DATABASE_URL` | no | PostgreSQL connection string (audit) |
| `NEXUS_CONTROL_PLANE_DB_*` | no | pool config (same params as data-plane) |
| `NEXUS_AUDIT_DB_*` | no | audit pool config |

### control-workers

| Variable | Required | Description |
|---|---|---|
| `PORT` | no (default 8082) | HTTP listen port |
| `NEXUS_API_KEYS` | yes | comma-separated accepted API keys |
| `NEXUS_CONTROL_PLANE_URL` | no | control-plane base URL (for audit) |
| `NEXUS_CONTROL_PLANE_API_KEY` | no | API key for control-plane calls |
| `NEXUS_CONTROL_WORKERS_DATABASE_URL` | no | PostgreSQL connection string |
| `NEXUS_CONTROL_WORKERS_DB_*` | no | pool config |

## API key consumers

| Key | Used by | Accepted by |
|---|---|---|
| admin API key | operators, admin scripts | all three services |
| data-plane service key | data-plane | control-plane, control-workers |
| control-workers service key | control-workers | control-plane |
| Prometheus API key | Prometheus scraper | all three services (/metrics) |

In AWS: keys live in Secrets Manager, injected via ECS task definition.
In docker compose: keys are in `.env` (dev only, not for production).

## API key rotation

Procedure:

1. Generate new key value
2. Add new key to `NEXUS_API_KEYS` of the accepting service (comma-separated, both old and new)
3. Deploy accepting service with both keys
4. Update calling service to use new key
5. Deploy calling service
6. Remove old key from `NEXUS_API_KEYS` of the accepting service
7. Deploy accepting service with only new key

Zero-downtime rotation because both keys are valid during transition.

For AWS Secrets Manager:
1. Update secret value in Secrets Manager
2. Force new deployment of ECS services (they pull secrets on start)

## Runbooks

### Service does not start

1. Check logs for startup errors: `docker compose logs <service>`
2. Common causes:
   - `NEXUS_API_KEYS` not set → service refuses to start without auth config
   - Database URL malformed → pool creation fails
   - Port already in use → bind error
   - Migration failure → check `schema_migrations` table for partial state
3. If migration failure: check the specific migration SQL, fix the issue, restart

### Database not connecting

1. Verify PostgreSQL is running: `docker compose ps` or check RDS status
2. Verify connection string is correct (host, port, dbname, credentials)
3. Check pool config: `MAX_CONNS` might be exhausted
4. Check `STATEMENT_TIMEOUT`: long queries might be timing out
5. Check PostgreSQL logs for connection limit exceeded or auth failures

### Migrations fail

1. Check `schema_migrations` table: `SELECT * FROM schema_migrations WHERE scope = '<scope>' ORDER BY version`
2. If a migration is partially applied (registered but SQL failed): it was rolled back by the transaction
3. Fix the migration SQL file, restart the service — it will retry
4. Never manually edit `schema_migrations` unless you understand the transactional guarantee

### Auth fails

1. Verify API key is in the `NEXUS_API_KEYS` list of the target service
2. Check header format: `X-API-Key: <key>` or `Authorization: Bearer <key>`
3. Check that the key doesn't have leading/trailing whitespace
4. For inter-service: verify the calling service has the correct `NEXUS_*_API_KEY` env var

### Actions blocked unexpectedly

1. Check the action response: `decision` field shows the outcome, `risk` shows the assessment
2. Check evidence: `GET /v1/actions/{id}/evidence` shows which checks passed/failed
3. Check policies: `GET /v1/policies?action_type=<type>&resource_type=<type>` shows matching policies
4. Check policy expressions: CEL expressions are in the `expression` field
5. If cache is stale (graceful degradation active):
   - check data-plane logs for `control-plane unavailable, using cached` warnings (includes `cache_age`, `expires_at`, `version`)
   - check audit records for `"degraded_context": true` in the `data` field — this means the decision was made with cached (potentially stale) data
   - degradation tracking is per-request via `DegradationCollector` in context (no false positives between concurrent requests)

### Database restore

1. Stop the service that uses the database
2. Run restore: `scripts/ops/postgres-restore.sh <dump-file> <database-name>`
3. Restart the service
4. Verify with smoke test

See also: `make smoke-db-restore`

### Rollback

1. Identify the previous working image tag
2. Update deployment to use previous tag
3. Deploy with rolling strategy
4. Run smoke test: `scripts/smoke/run-actions-basic-flow.sh`
5. If rollback is due to a bad migration: see "Migrations fail" section above

## Observability endpoints

All services expose:
- `GET /healthz` — liveness (no auth)
- `GET /readyz` — readiness including DB (no auth)
- `GET /metrics` — Prometheus metrics (requires API key)

## Smoke tests

| Script | What it validates |
|---|---|
| `run-actions-basic-flow.sh` | Full action lifecycle: create → policy eval → approve → lease → execute |
| `run-control-plane-resources-flow.sh` | Resource CRUD operations |
| `run-control-workers-incidents-flow.sh` | Incident and alert workflows |
| `run-persistence-restart-flow.sh` | Data survives service restart |
| `run-postgres-backup-restore-flow.sh` | Backup and restore |
| `run-observability-stack.sh` | Prometheus + Grafana + metrics |
| `run-degradation-flow.sh` | Graceful degradation when control-plane is down |
