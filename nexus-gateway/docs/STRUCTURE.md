# Nexus Gateway Design Notes

## Directory structure (implementation organization)

- `internal/<module>/handler`: Gin handlers + DTOs (transport).
- `internal/<module>/usecases`: application logic and ports (interfaces).
- `internal/<module>/repository`: GORM adapters implementing ports.
- `internal/gateway/executor/*`: gateway adapters (HTTP executor, rate limiting).
- `internal/identity`: JWT/JWKS authentication usecase + verifier adapter.
- `internal/gateway/idempotency_repository.go`: idempotency key persistence for write tools.
- `internal/dlp`: PII/token detectors used to produce `context.dlp` summary.
- `internal/secrets`: encrypted secret vault + secure injection metadata.
- `internal/egress`: per-tool host allowlist adapter.
- `internal/mcp`: MCP JSON-RPC endpoint (`tools/list|get|call`).
- `pkg/*`: domain-agnostic reusable packages (no Org/Tool/Policy/Audit terms).

Esta sección describe **estructura de carpetas**, no la arquitectura lógica por sí sola.

## Request flow (/v1/run)

1. **Auth middleware** resolves principal from API key and/or JWT/JWKS, then injects `org_id`, actor/role/scopes into context.
2. `gateway/handler` parses request, ensures/creates `request_id`, then calls the gateway usecase port.
3. `gateway/usecases` orchestrates:
   - tool resolve by `(org_id, tool_name)`
   - JSON Schema validation of `input`
   - DLP summary generation (`context.dlp.*`)
   - policy evaluation (conditions + limits + distributed/in-memory rate-limit + actor/role/scopes + dlp)
   - egress allowlist validation
   - secret lookup/decrypt and runtime header injection
   - tool execution (HTTP executor)
   - output validation (best effort)
   - idempotency handling (`Idempotency-Key`) for write tools
   - timeout budget handling (`X-Timeout-Ms`) and stage timings
   - audit event persistence (redacted + `dlp_summary` + hash-chain + idempotency/budget fields)
4. `mcp/handler` maps MCP methods to tool/gateway usecases without bypassing controls.

## Multi-tenancy enforcement

Every repository method accepts `orgID` and applies `WHERE org_id = ?` filters (and tool ownership checks). No handler is allowed to query without `org_id`.

## Why pkg/ is domain-agnostic

`pkg/` is reusable across domains/services (logging helpers, error types, JSON schema validator, HTTP middleware). Anything that mentions Tool/Policy/Audit/Org belongs in `internal/`.

## Wire organization

- Exactly one injector in `wire/wire.go`: `InitializeAPI(cfg) (*App, func(), error)`
- Provider sets are grouped by infra (`BootstrapSet`, `MiddlewareSet`, `ExecutorSet`) and by module (`OrgSet`, `ToolSet`, `PolicySet`, `AuditSet`, `SecretsSet`, `EgressSet`, `GatewaySet`, `MCPSet`).
- Telemetry is initialized in `cmd/api/main.go` and request tracing is attached via Gin middleware.

## Quality gates and contracts

- OpenAPI is maintained in `docs/openapi.yaml` as the API contract baseline.
- CI workflow (`.github/workflows/ci.yml`) runs:
  - `unit`: `go test ./...`
  - `smoke`: compose up + migrate + seed + health checks
  - `qa`: `make qa`
  - `jwt-e2e`: `make jwt-e2e`
- Audit export contract coverage lives in:
  - `internal/audit/export_contract_test.go`
  - `internal/audit/testdata/export.jsonl.golden`
  - `internal/audit/testdata/export.csv.golden`
