# Event Contract V1

## Objetivo
Definir el contrato canónico de eventos para operación por agentes dentro de `nexus-core`, con orden total determinista y consumo idempotente.

## Envelope canónico (obligatorio)
Todo evento persistido en `event_store` debe cumplir:

- `id`: UUID del evento.
- `event_type`: tipo canónico.
- `version`: entero del contrato.
- `occurred_at`: RFC3339 informativo.
- `org_id`: UUID del tenant.
- `correlation`: objeto con
  - `request_id?`
  - `incident_id?`
  - `action_id?`
- `actor`: objeto con
  - `actor_id?`
  - `actor_type` en `agent|human|system`
- `source`: servicio/módulo emisor.
- `payload`: JSON del dominio del evento.

## Orden y consumo
- Orden total de consumo: `event_store.sequence BIGSERIAL`.
- `occurred_at` no se usa para ordenar.
- Bus fase 1: DB-only.
- Lectura por cursor: `WHERE sequence > last_seen_sequence ORDER BY sequence ASC`.
- Offset por consumidor: `consumer_offsets(consumer_group, last_seen_sequence)`.
- Modelo: at-least-once + handlers idempotentes.

## Tipos mínimos de evento (v1)
- `tool_call.finished`
- `policy.denied`
- `quota.exceeded`
- `tool_degraded`
- `anomaly.detected`
- `incident.opened`
- `incident.state_changed`
- `diagnosis.created`
- `recommended_actions.created`
- `action.proposed`
- `action.dry_run_ok`
- `action.dry_run_failed`
- `action.applied`
- `action.failed`
- `action.rolled_back`
- `comms.draft_created`
- `comms.awaiting_approval`
- `comms.sent_internal`

## Versionado
- Versionado por par (`event_type`, `version`).
- Schemas en `internal/ops/schemas/events/`.
- Cambios incompatibles requieren `version` nueva.

## Validación
Cada evento se valida en dos pasos:
1. Envelope: `internal/ops/schemas/events/envelope_v1.json`.
2. Payload específico: `internal/ops/schemas/events/<event>_v1.json`.

## Ejemplo 1: `anomaly.detected`
```json
{
  "id": "4e8ab760-e8f3-4bc9-9298-37e4e28cb44a",
  "event_type": "anomaly.detected",
  "version": 1,
  "occurred_at": "2026-02-23T12:00:00Z",
  "org_id": "996e9e43-7bab-4e68-a831-0a766befbf54",
  "correlation": {
    "request_id": "a0f9f95e-90d0-4a9d-bf7a-4c545f52a2af"
  },
  "actor": {
    "actor_id": "sentry-worker",
    "actor_type": "agent"
  },
  "source": "agents.sentry",
  "payload": {
    "fingerprint": "fp:org:echo:error_rate",
    "signal": "error_rate_spike",
    "tool_name": "echo",
    "window_size": 50,
    "observed_value": 0.68,
    "threshold_value": 0.35,
    "evidence_refs": [
      "audit:tool_call.finished:429"
    ]
  }
}
```

## Ejemplo 2: `incident.opened`
```json
{
  "id": "5703dfd0-a360-4eb5-a36e-c7ea168e6f67",
  "event_type": "incident.opened",
  "version": 1,
  "occurred_at": "2026-02-23T12:01:00Z",
  "org_id": "996e9e43-7bab-4e68-a831-0a766befbf54",
  "correlation": {
    "incident_id": "f503f46f-c137-4165-b9ca-999d0d6f328f"
  },
  "actor": {
    "actor_id": "incident-coordinator",
    "actor_type": "agent"
  },
  "source": "agents.coordinator",
  "payload": {
    "incident_id": "f503f46f-c137-4165-b9ca-999d0d6f328f",
    "severity": "HIGH",
    "state": "OPEN",
    "title": "Tool error-rate spike",
    "summary": "Error-rate de echo por encima de baseline",
    "fingerprint": "fp:org:echo:error_rate"
  }
}
```

## Ejemplo 3: `action.applied`
```json
{
  "id": "75ec26cf-5652-4f2d-90fb-dd69601f4e50",
  "event_type": "action.applied",
  "version": 1,
  "occurred_at": "2026-02-23T12:02:00Z",
  "org_id": "996e9e43-7bab-4e68-a831-0a766befbf54",
  "correlation": {
    "incident_id": "f503f46f-c137-4165-b9ca-999d0d6f328f",
    "action_id": "82f62307-4127-41fa-a1d1-b38031e3a465",
    "request_id": "a0f9f95e-90d0-4a9d-bf7a-4c545f52a2af"
  },
  "actor": {
    "actor_id": "mitigation-worker",
    "actor_type": "agent"
  },
  "source": "agents.mitigation",
  "payload": {
    "proposal_id": "5791f5df-0fda-4119-8fe9-643d7c983a5d",
    "action_id": "82f62307-4127-41fa-a1d1-b38031e3a465",
    "action_type": "set_rate_limit",
    "scope": {
      "level": "tool",
      "org_id": "996e9e43-7bab-4e68-a831-0a766befbf54",
      "tool_id": "echo"
    },
    "ttl_seconds": 600
  }
}
```
