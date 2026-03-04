# Agent-Operated Model

## Control Principles

- `nexus-core` enforces execution policies deterministically.
- AI never bypasses enforcement and never writes directly to database state.
- All operator actions are append-only auditable events in Nexus Core.
- Human-in-the-loop approval can block executions until explicitly approved or rejected.

## Data and Action Flow

1. Runtime emits events into `operational_events`.
2. `nexus-control-operators` consumes internal operational streams and executes deterministic controls.
3. `nexus-ai-operators` consumes `/v1/events` with cursor and proposes AI-assisted actions.
4. Control actions are applied via `/v1/actions/apply`, incidents/proposals via API.
5. Humans supervise from `nexus-tower` and approve/reject proposal outcomes.
6. Alert rules fire webhooks when metrics (deny_rate, error_rate, rate_limited_count) exceed thresholds.

## Human-in-the-Loop (HITL)

- Policy DSL supports `require_approval: true` on any tool/condition match.
- When triggered, execution halts and a `pending_approval` record is created.
- Humans approve/reject via Tower UI or `POST /v1/approvals/:id/approve|reject`.
- Approvals have configurable TTL; expired approvals are cleaned up automatically.

## Agent Session Tracking

- Each agent can carry a `session_id` across calls.
- Core tracks per-session counters: `total_calls`, `total_writes`, `total_denials`.
- Queryable via `GET /v1/sessions/:session_id` for observability and anomaly detection.

## Ask-Agent Flow

- UI: `POST /v1/assistant/query` on Nexus Core.
- Core proxy: forwards to nexus-ai-operators `/v1/assistant/query` using internal key.
- Response is structured (`summary`, `tables`, `actions`) and rendered in Tower.

## Determinism Boundary

- In scope deterministic: `/v1/run`, `/mcp`, `/a2a` and policy/limits/egress/approval enforcement.
- Out of scope deterministic: nexus-ai-operators narrative/summarization endpoint.
- Enforcement decisions are never delegated to LLM.
