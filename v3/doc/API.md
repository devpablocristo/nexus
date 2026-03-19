# Nexus v3 — API Reference

Base URL: `http://localhost:18084`
Auth: `X-API-Key` header en todos los endpoints (excepto health).

## Health

| Endpoint | Método | Auth | Descripción |
|----------|--------|------|-------------|
| `/healthz` | GET | No | Liveness check |
| `/readyz` | GET | No | Readiness check (verifica DB) |

## Requests

### Submit request

```
POST /v1/requests
Header: Idempotency-Key: ... (opcional)
```

```json
{
  "requester_type": "agent",
  "requester_id": "ops-bot",
  "requester_name": "OpsBot v3",
  "action_type": "alert.silence",
  "target_system": "pagerduty",
  "target_resource": "CPU-CRITICAL-PROD-DB-01",
  "params": {"duration_minutes": 240},
  "reason": "Database migration in progress",
  "context": "Prod DB-01 CPU at 94%"
}
```

| Campo | Obligatorio | Tipo |
|-------|:-----------:|------|
| requester_type | ✅ | agent / service / human |
| requester_id | ✅ | string |
| action_type | ✅ | string |
| requester_name | | string |
| target_system | | string |
| target_resource | | string |
| params | | object |
| reason | | string |
| context | | string |
| idempotency_key | | string (también acepta header) |

Respuesta (201):
```json
{
  "request_id": "uuid",
  "decision": "require_approval",
  "risk_level": "high",
  "decision_reason": "Policy 'no-silence-critical'",
  "status": "pending_approval",
  "approval_id": "uuid",
  "expires_at": "2026-03-18T21:00:00Z",
  "ai_summary": "ops-bot quiere silenciar CPU-CRITICAL por 4h..."
}
```

Decisiones posibles: `allow`, `deny`, `require_approval`.

### Get request

```
GET /v1/requests/{id}
```

### List requests

```
GET /v1/requests?status=pending_approval&action_type=alert.silence&limit=20
```

### Report result

```
POST /v1/requests/{id}/result
```

```json
{
  "success": true,
  "result": {"silence_id": "sil_abc123"},
  "duration_ms": 180
}
```

### Replay

```
GET /v1/requests/{id}/replay
```

Respuesta:
```json
{
  "request_id": "uuid",
  "requester": {"type": "agent", "id": "ops-bot"},
  "action_type": "alert.silence",
  "target": "pagerduty / CPU-CRITICAL-PROD-DB-01",
  "final_status": "executed",
  "duration_total": "3m12s",
  "timeline": [
    {"event": "received", "actor": "ops-bot", "at": "...", "summary": "..."},
    {"event": "evaluated", "actor": "nexus", "at": "...", "summary": "..."},
    {"event": "sent_to_approval", "actor": "nexus", "at": "...", "summary": "..."},
    {"event": "approved", "actor": "sre@co", "at": "...", "summary": "..."},
    {"event": "executed", "actor": "ops-bot", "at": "...", "summary": "..."}
  ]
}
```

## Policies (CRUD — 7 operaciones)

| Operación | Método | Path | Status |
|-----------|--------|------|--------|
| Create | POST | `/v1/policies` | 201 |
| Read | GET | `/v1/policies/{id}` | 200 |
| List | GET | `/v1/policies` | 200 |
| Update | PATCH | `/v1/policies/{id}` | 200 |
| Delete | DELETE | `/v1/policies/{id}` | 204 |
| Archive | POST | `/v1/policies/{id}/archive` | 204 |
| Restore | POST | `/v1/policies/{id}/restore` | 204 |

- DELETE = hard delete (irreversible)
- Archive = soft delete (archived_at = now)
- List excluye archivados; `?archived=true` para incluir

### Create policy

```json
{
  "name": "deny-delete-production",
  "description": "Bloquea deletes en producción",
  "expression": "request.action_type == 'delete' && request.target_system == 'production'",
  "effect": "deny",
  "priority": 1,
  "enabled": true,
  "action_type": "delete",
  "target_system": "production",
  "risk_override": "high"
}
```

| Campo | Obligatorio | Valores |
|-------|:-----------:|---------|
| name | ✅ | string |
| expression | ✅ | CEL válido |
| effect | ✅ | allow / deny / require_approval |
| priority | | int (menor = mayor prioridad, default 100) |
| enabled | | bool (default true) |
| description | | string |
| action_type | | string (scope filter) |
| target_system | | string (scope filter) |
| risk_override | | low / medium / high |
| mode | | enforced (default) / shadow |

**Shadow mode:** cuando `mode: "shadow"`, la policy evalúa pero no actúa. Se incrementa `shadow_hits` en cada match. Útil para probar policies antes de activarlas. Se promueve a enforced cambiando `mode` via PATCH.

### Variables CEL disponibles

```
request.action_type          "alert.silence"
request.target_system        "pagerduty"
request.target_resource      "CPU-CRITICAL-PROD-DB-01"
request.params               map
request.reason               string
request.context              string
request.requester_type       "agent"
request.requester_id         "ops-bot"
time.hour                    0-23 (UTC)
time.day_of_week             0-6 (domingo=0)
```

## Simulation

### Simulate (dry-run)

```
POST /v1/requests/simulate
```

Mismos campos que `POST /v1/requests`, pero no persiste. Retorna decisión, factores de cascada, amplificación y score final.

### Replay simulate

```
POST /v1/requests/simulate/replay
```

Evalúa una expresión CEL propuesta contra el historial de requests existentes. Retorna cuántas habrían matcheado y con qué efecto.

## Approvals

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `/v1/approvals/pending` | GET | Listar pendientes |
| `/v1/approvals/{id}/approve` | POST | Aprobar |
| `/v1/approvals/{id}/reject` | POST | Rechazar |

```json
{
  "decided_by": "sre@company.dev",
  "note": "Ventana de mantenimiento confirmada"
}
```

### Break-glass

Cuando una approval tiene `break_glass: true`, requiere `required_approvals` aprobadores distintos. Un rechazo de cualquier aprobador cancela la approval. El mismo aprobador no puede decidir dos veces. El campo `decisions` (JSONB) registra cada decisión parcial.

## Learning

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `/v1/learning/proposals` | GET | Listar propuestas pendientes |
| `/v1/learning/proposals/{id}` | GET | Detalle de propuesta |
| `/v1/learning/proposals/{id}/accept` | POST | Aceptar (crea policy con origin='learned') |
| `/v1/learning/proposals/{id}/dismiss` | POST | Descartar |
| `/v1/learning/analyze` | POST | Trigger análisis de patrones |

## Dashboard

```
GET /v1/metrics/summary?period=7d
```

```json
{
  "period": "7d",
  "total_requests": 156,
  "allowed": 89,
  "denied": 12,
  "pending_approval": 3,
  "approved": 47,
  "rejected": 5
}
```

## Config

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `/v1/config` | GET | Toda la configuración (5 secciones) |
| `/v1/config` | PATCH | Actualizar múltiples secciones |
| `/v1/config/{section}` | PATCH | Actualizar una sección (risk, approvals, learning, ai, general) |
| `/v1/config/reset` | POST | Restaurar valores por defecto |

## Errores

Formato consistente:
```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "policy not found"
  }
}
```

| Code | HTTP | Descripción |
|------|------|-------------|
| VALIDATION | 400 | Input inválido |
| UNAUTHORIZED | 401 | API key inválida o ausente |
| NOT_FOUND | 404 | Recurso no encontrado |
| CONFLICT | 409 | Estado inválido (ej: approval ya decidida) |
| INTERNAL | 500 | Error interno (nunca expone detalles) |
