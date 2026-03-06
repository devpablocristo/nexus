# Service Boundaries

## Ownership por servicio

### `nexus-core`

Owner del data plane:

- `/v1/run`, `/v1/run/simulate`
- `/mcp`
- `/a2a/call`
- tools, policies, egress, secrets
- approvals e idempotencia
- audit y hash-chain
- internal operator bridge `/internal/operators/*`

### `nexus-saas`

Owner del business plane:

- billing, tenant lifecycle, entitlements
- users, org members, notifications
- incidents, actions, events, policy proposals
- alert rules y usage metering
- assistant proxy `/v1/assistant/*`
- core proxy limitado a audit, approvals y openapi

### `nexus-control-operators`

- consume eventos operativos
- abre incidentes y aplica acciones vía APIs internas
- no escribe directo a DB

### `nexus-ai-operators`

- backend del assistant
- prompting runtime versionado, guardrails, fallback y evals
- engine loop observando eventos vía bridge interno
- no escribe directo a DB

### `nexus-tower`

- consume APIs de `core` y `saas`
- la UI no es source of truth
- el assistant entra por `nexus-saas`, no directo a `nexus-ai-operators`

## Contratos internos

### Core -> SaaS

- `GET /internal/entitlements/:org_id`
- `GET /internal/runtime-overrides/:org_id/:tool_name`
- `POST /internal/usage/events`

### Operators -> Core bridge

- `GET /internal/operators/events`
- `POST /internal/operators/events/append`
- `POST /internal/operators/actions/apply`
- `POST /internal/operators/incidents`
- `POST /internal/operators/policy-proposals`

### SaaS -> AI operators

- `POST /v1/assistant/query`
- `POST /v1/internal/tick`

### AI operators -> SaaS

- `GET /internal/assistant/context/:org_id`

## Reglas

- `nexus-core` no toma ownership de billing, users o tenant settings.
- `nexus-saas` no implementa enforcement.
- Cambios cross-service requieren contrato, tests y docs coordinados.
