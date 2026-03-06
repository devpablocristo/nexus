# Agent-Operated Model

## Control Principles

- `nexus-core` enforces execution policies deterministically. No LLM in the run pipeline.
- AI never bypasses enforcement and never writes directly to database state.
- All operator actions are append-only auditable events in Nexus Core.
- Human-in-the-loop approval can block executions until explicitly approved or rejected.

## Data and Action Flow

1. `nexus-core` processes `/v1/run` requests, applies the full governance pipeline, and writes audit events.
2. A DB trigger copies audit entries into `operational_events` as `tool.call.completed`.
3. `nexus-control-operators` polls `/internal/operators/events` and consumes events with four deterministic workers (sentry, coordinator, mitigation, recovery).
4. `nexus-ai-operators` observa estado operativo vía el bridge interno de operators y completa el contexto del assistant con un snapshot tenant-aware consumido desde `nexus-saas`.
5. Control actions are applied via the Action Engine (dry-run → apply → rollback lifecycle).
6. Humans supervise from `nexus-tower` and approve/reject proposals.
7. Alert rules fire webhooks when metrics (deny_rate, error_rate, rate_limited_count) exceed thresholds.

## Workers

### Deterministic (nexus-control-operators, Go)

| Worker | Consumes | Produces | Role |
|--------|----------|----------|------|
| Sentry | `tool.call.completed`, `policy.denied`, `quota.exceeded`, `tool_degraded` | `anomaly.detected`, `incident.opened` | Anomaly detection (EWMA baselines) |
| Coordinator | `incident.opened`, `anomaly.detected`, `incident.state_changed` | `incident.state_changed` | Incident state machine (OPEN→DIAGNOSING→MITIGATING→MONITORING→RESOLVED/ESCALATED) |
| Mitigation | `recommended_actions.created` | `action.applied` | Dry-run and apply remediation actions |
| Recovery | `action.applied` | `action.rolled_back`, `incident.state_changed` | Post-mitigation monitoring and rollback |

### AI-assisted (nexus-ai-operators, Python)

| Flow | Role |
|------|------|
| `assistant_system` | Assistant queries from Tower UI |
| `diagnosis_system` | Diagnosis-oriented summaries |
| `comms_system` | Internal communication drafts |
| `executive_qa_system` | Leadership-style operational Q&A |

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

- UI: `POST /v1/assistant/query` on nexus-saas.
- SaaS forwards to nexus-ai-operators `/v1/assistant/query` using internal key.
- AI operators fetch `GET /internal/assistant/context/:org_id` from SaaS to build a redacted tenant snapshot.
- Response is structured (`summary`, `tables`, `actions`) and rendered in Tower.

## Determinism Boundary

| Component | Deterministic | AI/LLM |
|-----------|:---:|:---:|
| Gateway `/v1/run`, `/mcp`, `/a2a` | Yes | No |
| Policy evaluation, DLP, egress | Yes | No |
| Sentry, Coordinator, Mitigation, Recovery | Yes | No |
| Diagnostician, Comms, Executive Q&A | No | Yes |

Enforcement decisions are never delegated to LLM.
