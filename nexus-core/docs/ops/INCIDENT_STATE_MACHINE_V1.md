# Incident State Machine V1

## Estados
- `OPEN`
- `DIAGNOSING`
- `MITIGATING`
- `MONITORING`
- `RESOLVED`
- `ESCALATED`

## Transiciones permitidas
- `OPEN -> DIAGNOSING`
- `DIAGNOSING -> MITIGATING`
- `DIAGNOSING -> ESCALATED`
- `MITIGATING -> MONITORING`
- `MITIGATING -> ESCALATED`
- `MONITORING -> RESOLVED`
- `MONITORING -> OPEN` (rollback o regresión)
- `ESCALATED -> MITIGATING`
- `ESCALATED -> RESOLVED`

## Reglas
- `incident.opened` crea el incidente en `OPEN`.
- `anomaly.detected` solo reabre/actualiza si el fingerprint coincide.
- `diagnosis.created` mueve a `DIAGNOSING` y puede generar `recommended_actions.created`.
- `action.applied` mueve a `MITIGATING` (si corresponde).
- `action.rolled_back` puede mover a `OPEN`.
- ventana de verificación estable mueve `MONITORING -> RESOLVED`.
- sin progreso por cooldown configurable mueve a `ESCALATED`.

## No-claim policy
- si diagnóstico tiene `confidence < 0.7`, no declarar root cause definitivo en estado público.
- si evidencia insuficiente: root cause = `unknown`.
