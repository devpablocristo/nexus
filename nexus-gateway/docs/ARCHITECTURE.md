# Nexus Backend Architecture

## Module boundaries

- `internal/<module>/handler`: Gin handlers + DTOs (transport).
- `internal/<module>/usecases`: application logic and ports (interfaces).
- `internal/<module>/repository`: GORM adapters implementing ports.
- `internal/executor/*`: non-DB adapters (HTTP executor, rate limiting).
- `pkg/*`: domain-agnostic reusable packages (no Org/Tool/Policy/Audit terms).

## Request flow (/v1/run)

1. **Auth middleware** resolves `org_id` from `X-NEXUS-GATEWAY-KEY` and injects `org_id` + actor (`X-NEXUS-ACTOR`) into context.
2. `gateway/handler` parses request, ensures/creates `request_id`, then calls the gateway usecase port.
3. `gateway/usecases` orchestrates:
   - tool resolve by `(org_id, tool_name)`
   - JSON Schema validation of `input`
   - policy evaluation (conditions + limits + rate-limit)
   - tool execution (HTTP executor)
   - output validation (best effort)
   - audit event persistence (redacted)
4. `audit/repository` persists an append-only row in `audit_events`.

## Multi-tenancy enforcement

Every repository method accepts `orgID` and applies `WHERE org_id = ?` filters (and tool ownership checks). No handler is allowed to query without `org_id`.

## Why pkg/ is domain-agnostic

`pkg/` is reusable across domains/services (logging helpers, error types, JSON schema validator, HTTP middleware). Anything that mentions Tool/Policy/Audit/Org belongs in `internal/`.

## Wire organization

- Exactly one injector in `wire/wire.go`: `InitializeAPI(cfg) (*App, func(), error)`
- Provider sets are grouped by infra (`ConfigSet`, `BootstrapSet`, `MiddlewareSet`, `ExecutorSet`) and by module (`OrgSet`, `ToolSet`, `PolicySet`, `AuditSet`, `GatewaySet`).

