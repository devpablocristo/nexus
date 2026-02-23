# Internal Agents Overview

## Scope
Este documento define los workers internos del MVP y sus responsabilidades.

## 1) Sentry (sin LLM)
- Consume: `tool_call.finished`, `policy.denied`, `quota.exceeded`, `tool_degraded`.
- Produce: `anomaly.detected`, `incident.opened` (cuando no existe incidente activo para fingerprint).
- Objetivo: detectar anomalías con baselines deterministas.

## 2) Incident Coordinator (sin LLM)
- Consume: `incident.*`, `diagnosis.created`, `action.*`, `comms.*`.
- Produce: `incident.state_changed`, solicitudes de diagnóstico y mitigación.
- Objetivo: orquestar el flujo y aplicar cooldown/locks.

## 3) Diagnostician (LLM read-only)
- Consume: incidentes abiertos y señales de coordinación.
- Produce: `diagnosis.created`, `recommended_actions.created`.
- Reglas: JSON estricto; si no hay evidencia suficiente => `unknown`.

## 4) Mitigation (sin LLM)
- Consume: `recommended_actions.created`.
- Produce: `action.proposed`, `action.dry_run_ok|failed`, `action.applied|failed`.
- Reglas: aplica sólo acciones permitidas y respeta approvals/TTL.

## 5) Verification & Recovery (sin LLM)
- Consume: `action.applied` + métricas/estado.
- Produce: `incident.state_changed`, `action.rolled_back` cuando corresponde.
- Objetivo: validar mejora estable o revertir automáticamente.

## 6) Comms (LLM)
- Consume: `incident.*`, `diagnosis.created`, `action.applied`.
- Produce: `comms.draft_created`, `comms.awaiting_approval`, `comms.sent_internal`.
- Reglas: sin root cause concluyente cuando `confidence < 0.7`.

## 7) Executive Q&A (LLM)
- Consume: consultas de operador.
- Produce: respuesta con `answer`, `evidence_refs`, y propuestas de acción opcionales.
- Reglas: propuestas sólo vía Action Engine.

## Operación
- Cada worker tiene `consumer_group` único.
- Todos persisten `last_seen_sequence` en `consumer_offsets`.
- Reintentos permitidos bajo at-least-once con idempotencia en handlers.
