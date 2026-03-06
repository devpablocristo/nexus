# Service Boundaries

Defines ownership between services in the Nexus system.

## Services

| Service | Role | Language | Database |
|---------|------|----------|----------|
| `nexus-core` | Data plane — gateway, enforcement, audit | Go | `nexus` (PostgreSQL) |
| `nexus-saas` | Business plane — events, actions, incidents, alerts, sessions, proposals, assistant, usage metering | Go | `nexus_saas` (PostgreSQL) |
| `nexus-control-operators` | Deterministic control plane — sentry, coordinator, mitigation, recovery | Go | None (file-based persistence in `/app/data`) |
| `nexus-ai-operators` | AI operators — diagnosis, comms, policy suggestions | Python | None |
| `nexus-tower` | Supervision UI | TypeScript/React | None |

## Ownership

### nexus-core (data-plane)
- Run/simulate execution pipeline (`/v1/run`, `/v1/run/simulate`)
- Policy evaluation and approval workflow
- DLP, egress/SSRF, idempotency, timeout budget, circuit breaker
- Tool/policy/secret/egress CRUD
- Audit events with hash-chain
- MCP and A2A protocol endpoints
- Internal operator events API (`/internal/operators/*`)
- Org onboarding (`POST /v1/orgs`)

### nexus-saas (business-plane)
- Events stream (`/v1/events`)
- Actions lifecycle (`/v1/actions`)
- Incidents management (`/v1/incidents`)
- Alert rules CRUD (`/v1/alert-rules`) with webhook dispatch
- Agent session tracking (`/v1/sessions`)
- Policy proposals (`/v1/policy-proposals`)
- Assistant query proxy (`/v1/assistant/query`)
- Admin console API (`/v1/admin/*`)
- Usage metering aggregation
- OIDC/SSO endpoints
- Core proxy (forwards tools/audit/approvals/run requests to nexus-core)
- Internal contracts for entitlements

### nexus-control-operators
- Consumes events from nexus-core via `/internal/operators/events`
- Applies actions via nexus-core API
- Creates incidents via nexus-core API
- No direct database access

### nexus-ai-operators
- Consumes events and context via nexus-saas API
- Proposes actions via nexus-core/nexus-saas API
- No direct database access
- Runtime prompting, evals, guardrails y fallback pertenecen a este servicio, pero nunca decide enforcement sobre `/v1/run`

## Rules

- `nexus-core` must not own SaaS billing/plan state.
- `nexus-saas` must not implement core operational domains (run, policy enforcement, audit write).
- Cross-service communication uses internal HTTP contracts with `X-NEXUS-AI-KEY` or `X-NEXUS-SAAS-KEY` headers.
- Databases are separate: `nexus` (core) and `nexus_saas` (saas).
- Tower talks to the assistant via `nexus-saas`; it must not call `nexus-ai-operators` directly.
- Any new prompt or feature must remain aligned with `docs/prompts/00_base_transversal.md`.

## Internal Contracts

### Core → SaaS
- `POST /internal/usage/events` — usage metering
- `GET /internal/entitlements/:org_id` — plan limits

### SaaS → Core (core proxy)
- `GET/POST/PUT/DELETE /v1/tools/*` — forwarded from Tower
- `GET /v1/audit/*` — forwarded from Tower
- `POST /v1/run` — forwarded from Tower
- `GET/POST /v1/approvals/*` — forwarded from Tower

### Control Operators → Core
- `GET /internal/operators/events?cursor=N&limit=N` — poll events
- `POST /internal/operators/events` — emit events
- `POST /internal/operators/incidents` — create incidents
- `POST /internal/operators/actions/*` — apply/rollback actions
- `GET /readyz` — health check
