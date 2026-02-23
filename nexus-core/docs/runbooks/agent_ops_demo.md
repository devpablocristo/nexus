# Agent Ops Demo Runbook

## Objetivo
Demostrar el flujo E2E mínimo:

`tool_call.finished spike -> anomaly.detected -> incident.opened -> diagnosis.created -> recommended_actions.created -> action dry-run/apply -> recovery -> comms.draft_created`

## Prerrequisitos
- Migraciones aplicadas.
- `NEXUS_LLM_PROVIDER=mock`.
- Datos demo seed cargados.

## Pasos
1. Generar una ráfaga de `tool_call.finished` con error rate alto para un tool.
2. Verificar que Sentry emita `anomaly.detected`.
3. Confirmar que Coordinator abra incidente.
4. Ejecutar Diagnostician (mock) y validar `diagnosis.created`.
5. Confirmar `recommended_actions.created`.
6. Ejecutar Mitigation:
   - `POST /v1/admin/actions/dry-run`
   - `POST /v1/admin/actions/apply`
7. Verificar transición a `MONITORING` y `RESOLVED` o rollback.
8. Revisar evento `comms.draft_created`.

## Checks esperados
- Todos los eventos validados por schema.
- `consumer_offsets` avanza por `consumer_group`.
- No duplicación de efectos ante restart de worker.
- `request_id`, `incident_id`, `action_id` correlacionados en eventos.

## Troubleshooting
- Si falla validación de schema LLM:
  - evento inválido persistido con marca de error,
  - no se ejecutan acciones,
  - salida de diagnóstico degradada a `unknown`.
- Si hay lag de consumidores:
  - revisar `consumer_offsets`,
  - revisar poll interval y tamaño de batch.
