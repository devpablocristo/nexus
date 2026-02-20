# Agent-Operated Model

## Control Principles

- `nexus-core` enforces execution policies deterministically.
- AI never bypasses enforcement and never writes directly to database state.
- All operator actions are append-only auditable events in Nexus Core.

## Data and Action Flow

1. Runtime emits events into `operational_events`.
2. `nexus-operator` consumes `/v1/events` with cursor.
3. Operator emits reversible controls (TTL) via `/v1/actions/apply`.
4. Operator opens incidents and proposals via API.
5. Humans supervise from `nexus-tower` and approve/reject proposal outcomes.

## Ask-Agent Flow

- UI: `POST /v1/assistant/query` on Nexus Core.
- Core proxy: forwards to operator `/v1/assistant/query` using internal key.
- Response is structured (`summary`, `tables`, `actions`) and rendered in Tower.

## Determinism Boundary

- In scope deterministic: `/v1/run`, `/mcp`, `/a2a` and policy/limits/egress enforcement.
- Out of scope deterministic: operator narrative/summarization endpoint.
- Enforcement decisions are never delegated to LLM.
