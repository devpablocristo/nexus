# Event Catalog

Catálogo narrativo de eventos operativos usados hoy por el repo.

## Eventos compartidos de runtime / SaaS

| event_type | Producer | Consumers | Trigger | Idempotencia / notas |
|------------|----------|-----------|---------|----------------------|
| `tool.call.completed` | `nexus-core` trigger sobre audit/runtime | AI operators, docs, analytics | ejecución terminada | append-only |
| `tool.denied` | `nexus-core` | tower/analytics | request bloqueada | append-only |
| `tool.rate_limited` | `nexus-core` | tower/analytics | tenant/tool limit excedido | append-only |
| `action.applied` | `nexus-saas`, control-operators | recovery, tower | override temporal aplicado | idempotente por `action_id` |
| `action.rolled_back` | `nexus-saas`, recovery | tower | rollback | idempotente por `action_id` |
| `action.expired` | `nexus-saas` | tower | TTL vencido | append-only |
| `incident.opened` | `nexus-saas`, control-operators | notifications, coordinator, tower | incidente creado | idempotente por `incident_id` |
| `incident.closed` | `nexus-saas` | notifications, tower | cierre manual/runtime | append-only |
| `proposal.created` | `nexus-saas` | tower, reviewers | propuesta nueva | append-only |
| `proposal.approved` | `nexus-saas` | tower | aprobación humana | append-only |
| `proposal.rejected` | `nexus-saas` | tower | rechazo humano | append-only |
| `proposal.shadow_started` | `nexus-saas` | tower | shadow mode | append-only |

## Eventos del control plane determinista

| event_type | Producer | Consumers | Trigger | Notas |
|------------|----------|-----------|---------|-------|
| `tool_call.finished` | control-plane ingest | sentry, recovery | runtime/canonicalized tool completion | contrato de operators |
| `policy.denied` | control-plane ingest | sentry | deny rate / anomaly detection | schema dedicado |
| `quota.exceeded` | control-plane ingest | sentry | over-limit runtime | schema dedicado |
| `tool_degraded` | sentry inputs | sentry | señal degradada | interno del control plane |
| `anomaly.detected` | sentry | coordinator, tower | EWMA/anomaly match | internal operational event |
| `incident.state_changed` | coordinator, recovery | tower, humans | state machine transition | ordering importa por incident |
| `diagnosis.created` | diagnosis flow | coordinator | hipótesis/evidencia generada | no source of truth externa |
| `recommended_actions.created` | diagnosis/control logic | mitigation, coordinator | lista de acciones recomendadas | interno |
| `action.proposed` | action engine | humans/coordinator | propuesta previa al apply | interno |
| `action.dry_run_ok` | action engine | mitigation/coordinator | dry-run exitoso | interno |
| `action.dry_run_failed` | action engine | mitigation/coordinator | dry-run fallido | interno |
| `action.failed` | action engine | coordinator | apply fallido | interno |
| `comms.draft_created` | comms flow | humans | draft inicial | interno |
| `comms.awaiting_approval` | comms flow | humans | draft bloqueado por aprobación | interno |
| `comms.sent_internal` | comms flow | audit/humans | comunicación enviada | interno |

## Consistency assumptions

- Los catálogos `events` de `nexus-saas` son append-only y se consumen por cursor.
- Los events del control plane usan schemas propios bajo `control-workers/internal/ops/schemas/events`.
- Eventual consistency entre `core`, `saas` y operators es explícita: el UI observa estado derivado, no sincronía estricta.
