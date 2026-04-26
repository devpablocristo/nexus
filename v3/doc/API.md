# Nexus v3 — API Reference

Base URL: `http://localhost:18084`
Auth: `X-API-Key` o JWT Bearer en todos los endpoints (excepto health). El backend deriva un principal efectivo con `actor_id`, `org_id`, roles/scopes y método de auth. Headers manuales como `X-User-ID` y `X-Org-ID` no son autoridad de identidad cuando hay middleware de auth.

Formato extendido de API key:

```text
name=secret|org_id=org-a|actor=companion|service_principal=true|scopes=nexus:requests:read+nexus:requests:write
```

El formato legacy `admin=secret` sigue funcionando en local/dev. Los scopes principales son:

| Scope | Uso |
|-------|-----|
| `nexus:requests:read` | leer/listar/simular/replay de requests |
| `nexus:requests:write` | crear requests |
| `nexus:requests:result` | reportar `/result` |
| `nexus:approvals:decide` | listar/decidir approvals |
| `nexus:policies:admin` | administrar policies |
| `nexus:evidence:write` | crear attestations/evidence write |
| `nexus:cross_org` | acceso cross-org explicito |

## Health

| Endpoint | Método | Auth | Descripción |
|----------|--------|------|-------------|
| `/healthz` | GET | No | Liveness check |
| `/readyz` | GET | No | Readiness check (verifica DB) |

## Requests

### Submit request

```
POST /v1/requests
Header: Idempotency-Key: ... (opcional, fuente canónica)
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
| idempotency_key | | string (compatibilidad gradual; si existe header, gana `Idempotency-Key`) |

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

Solo acepta resultados para requests en estado ejecutable (`allowed` o `approved`). Estados como `denied`, `pending_approval`, `rejected`, `executed` o `failed` devuelven `409`.

Requiere scope `nexus:requests:result` y respeta `org_id` del principal efectivo salvo principal con `nexus:cross_org`.

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

Create/update valida que la expresión CEL compile y retorne bool, que `effect`, `mode`, `risk_override` y prioridad sean válidos, y rechaza configuraciones inválidas con `400`.

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

`decided_by` se deriva del principal autenticado cuando está disponible; el body queda como compatibilidad local. Una approval vencida o ya decidida devuelve `409`.

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

## Action Types (ontología tipada)

| Operación | Método | Path | Status |
|-----------|--------|------|--------|
| Create | POST | `/v1/action-types` | 201 |
| Read | GET | `/v1/action-types/{id}` | 200 |
| List | GET | `/v1/action-types` | 200 |
| Update | PATCH | `/v1/action-types/{id}` | 200 |
| Delete | DELETE | `/v1/action-types/{id}` | 204 |

`risk_class`, `schema` y `requires_break_glass` impactan el runtime de Submit/Simulate: el riesgo base sale del action type, `params` se valida contra `schema.required` cuando existe y `requires_break_glass` activa approvals múltiples.

### Create action type

```json
{
  "name": "treasury.transfer",
  "description": "Transferir fondos entre cuentas",
  "category": "treasury",
  "risk_class": "critical",
  "schema": {},
  "reversible": false,
  "requires_break_glass": true
}
```

| Campo | Obligatorio | Valores |
|-------|:-----------:|---------|
| name | si | string (único) |
| description | | string |
| category | | string |
| risk_class | | low / medium / high / critical (default: low) |
| schema | | object (JSON schema para validación de params) |
| reversible | | bool (default: true) |
| requires_break_glass | | bool (default: false) |

9 action types pre-configurados: alert.silence, alert.escalate, runbook.execute, incident.resolve, config.update, deploy.trigger, delete, iam.grant_role, treasury.transfer.

Integración en Submit: si `action_type` no está registrado en la tabla → 403 FORBIDDEN.

## Delegations (delegation graph)

| Operación | Método | Path | Status |
|-----------|--------|------|--------|
| Create | POST | `/v1/delegations` | 201 |
| Read | GET | `/v1/delegations/{id}` | 200 |
| List | GET | `/v1/delegations` | 200 |
| Update | PATCH | `/v1/delegations/{id}` | 200 |
| Delete | DELETE | `/v1/delegations/{id}` | 204 |

### Create delegation

```json
{
  "owner_id": "sre-team-lead",
  "owner_type": "user",
  "agent_id": "ops-bot",
  "agent_type": "agent",
  "allowed_action_types": ["alert.silence", "alert.escalate"],
  "allowed_resources": ["pagerduty/*"],
  "purpose": "Gestión automática de alertas",
  "max_risk_class": "high",
  "expires_at": "2026-06-01T00:00:00Z"
}
```

| Campo | Obligatorio | Valores |
|-------|:-----------:|---------|
| owner_id | si | string |
| agent_id | si | string |
| owner_type | | string (default: user) |
| agent_type | | string (default: agent) |
| allowed_action_types | | array de strings |
| allowed_resources | | array de strings |
| purpose | | string |
| max_risk_class | | low / medium / high / critical (default: high) |
| expires_at | | RFC3339 datetime |
| enabled | | bool (default: true) |

Integración en Submit: si el agente no tiene delegación vigente para la acción solicitada → 403 FORBIDDEN.

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
| FORBIDDEN | 403 | Action type desconocido o agente sin delegación vigente |
| NOT_FOUND | 404 | Recurso no encontrado |
| CONFLICT | 409 | Estado inválido (ej: approval ya decidida) |
| INTERNAL | 500 | Error interno (nunca expone detalles) |
