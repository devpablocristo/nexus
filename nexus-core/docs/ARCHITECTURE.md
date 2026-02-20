# Nexus Gateway Architecture

## Logical architecture (hexagonal)

`transport (Gin handlers)` → `usecases (application services + ports)` → `adapters (repositories/executors)`

- Handlers only map HTTP/JSON (or MCP JSON-RPC) to usecase inputs/outputs.
- Usecases orchestrate business flow and do not import Gin/GORM.
- Adapters implement ports for DB, HTTP execution, rate limiting, secrets, identity, egress, and audit storage.

## Main request flow (`POST /v1/run`)

1. Auth middleware resolves tenant and actor context (`org_id`, `actor`, `role`, `scopes`).
2. Gateway usecase resolves tool by tenant and validates input schema.
3. Policy engine evaluates conditions/limits (including DLP summary and egress checks).
4. Executor performs outbound call (timeouts/retries/response limits).
5. Audit event is appended with redacted payloads and hash-chain integrity fields.

## Packaging and quality gates

- OpenAPI source of truth: `docs/openapi.yaml`.
- CI workflow: `.github/workflows/ci.yml` with jobs:
  - `unit` (`go test ./...`)
  - `smoke` (compose + migrate + seed + health checks)
  - `qa` (`make qa`)
  - `jwt-e2e` (`make jwt-e2e`)
- Contract tests for audit export:
  - `internal/audit/export_contract_test.go`
  - `internal/audit/testdata/export.jsonl.golden`
  - `internal/audit/testdata/export.csv.golden`

## Idempotency guarantee (WRITE tools)

- Same key + same fingerprint:
  - `COMPLETED` → success replay
  - `FAILED` → terminal error replay
- Same key + different fingerprint → conflict (`IDEMPOTENCY_KEY_CONFLICT`)
- Retry after `FAILED` requires a new `Idempotency-Key`.
