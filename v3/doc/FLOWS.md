# Nexus v3 — Flujos

## 0. Verificación previa (action type + delegación)

```
Requester ──POST /v1/requests──▶ Nexus
                                    │
                               Validar + idempotencia
                                    │
                               ¿action_type registrado en action_types?
                               ├── No → 403 FORBIDDEN ("unknown action type")
                               │
                               ¿requester tiene delegación vigente?
                               ├── No → 403 FORBIDDEN ("agent not delegated")
                               │
                               Continuar con evaluación normal...
```

Ambas verificaciones se ejecutan antes de cualquier evaluación de policies o riesgo.

## 1. Auto-allow (sin policy match, riesgo bajo)

```
Requester ──POST /v1/requests──▶ Nexus
                                    │
                               Validar + idempotencia + action_type + delegación
                                    │
                               Evaluar políticas CEL (ninguna matchea)
                                    │
                               Risk: low → Decision: allow
                                    │
                               Audit: [received, evaluated, allowed]
                                    │
                               Return {decision: "allow", status: "allowed"}
                                    │
Requester ◀────────────────────────┘
   │
   └──▶ Ejecuta acción, reporta resultado
         POST /v1/requests/{id}/result
```

## 2. Deny (policy matchea con effect=deny)

```
Requester ──POST /v1/requests──▶ Nexus
                                    │
                               Policy 'deny-deletes-prod' matchea
                                    │
                               Decision: deny
                                    │
                               Audit: [received, evaluated, denied]
                                    │
                               Return {decision: "deny", reason: "Policy 'deny-deletes-prod'"}
```

## 3. Require approval (policy o high risk)

```
Requester ──POST /v1/requests──▶ Nexus
                                    │
                               Policy matchea → require_approval
                                    │
                               Claude genera resumen AI (best-effort)
                                    │
                               Crear approval (TTL 1h)
                                    │
                               Audit: [received, evaluated, sent_to_approval]
                                    │
                               Return {decision: "require_approval", approval_id, ai_summary}
                                    │
Requester ◀────────────────────────┘
   │
   └──▶ Poll GET /v1/requests/{id} (espera status change)

                    ┌─────────────────────────────────┐
                    │  Console — Inbox                  │
                    │                                   │
                    │  ● HIGH  alert.silence            │
                    │  "ops-bot quiere silenciar..."     │
                    │                                   │
                    │  Nota: "Ventana confirmada" ____  │
                    │  Escribir APPROVE: ____________   │
                    │  [Confirmar aprobación] [Cancelar]│
                    └─────────────────────────────────┘
                                    │
                               Approve (nota obligatoria + escribir APPROVE)
                                    │
                               Audit: [approved]
                                    │
                               request.status → approved
                                    │
Requester detecta status=approved
   │
   └──▶ Ejecuta, reporta resultado
         Audit: [executed]
         request.status → executed
```

## 4. Learning loop

```
Trigger: POST /v1/learning/analyze
   │
   └──▶ Analizar requests de últimos 14 días
   │
   └──▶ Agrupar por action_type
   │    Contar: total, approved, rejected
   │
   └──▶ Detectar patrones:
   │    ≥50 muestras AND ≥90% approval rate
   │
   └──▶ Para cada patrón:
   │    Generar propuesta CEL (stub o Claude)
   │    Guardar en policy_proposals (status=pending)
   │
   └──▶ Usuario ve propuestas en Console → Learning
         │
         ├── Accept → crea policy (origin='learned', proposal_id=FK)
         │            futuras requests se auto-aprueban
         │
         └── Dismiss → marca como descartada

Resultado: menos intervención humana con el tiempo.
```

## 5. Replay (postmortem)

```
GET /v1/requests/{id}/replay
   │
   └──▶ Buscar request + todos sus events (ordered by created_at)
   │
   └──▶ Reconstruir timeline:
         {
           request_id, requester, action_type, target,
           final_status, duration_total,
           timeline: [
             {event: "received",        actor: "ops-bot",    at: "T+0s"},
             {event: "evaluated",        actor: "nexus",      at: "T+1s"},
             {event: "sent_to_approval", actor: "nexus",      at: "T+2s"},
             {event: "approved",         actor: "sre@co",     at: "T+3m10s"},
             {event: "executed",         actor: "ops-bot",    at: "T+3m12s"}
           ]
         }
```

## 6. Idempotencia

```
POST /v1/requests (Idempotency-Key: "ops-bot-silence-db01")
   │
   └──▶ ¿Key existe en idempotency_keys?
         │
         ├── Sí → retornar respuesta cacheada (sin re-procesar)
         │
         └── No → procesar normalmente
                   guardar response en cache
                   TTL: 24h (allow/deny), approval TTL (pending_approval)
```

## 7. Policy evaluation (CEL)

```
Lista de políticas activas (enabled=true, archived_at IS NULL, mode='enforced')
   │
   └──▶ Filtrar por scope:
   │    policy.action_type == request.action_type (si definido)
   │    policy.target_system == request.target_system (si definido)
   │
   └──▶ Ordenar por priority (menor = mayor)
   │
   └──▶ Para cada policy (first-match-wins):
   │    Compilar CEL expression (cacheado)
   │    Evaluar: {request: {...}, time: {hour, day_of_week}}
   │    Si true → usar esta policy
   │
   └──▶ Shadow policies (mode='shadow'):
   │    Se evalúan en paralelo pero NO afectan la decisión
   │    Si matchean → incrementar shadow_hits
   │
   └──▶ Si ninguna enforced matchea → decision por riesgo default
```

## 8. Risk tiering

```
¿Policy tiene risk_override? → usar ese nivel
¿action_type en high list? (alert.silence, runbook.execute) → HIGH
¿action_type en medium list? (incident.resolve) → MEDIUM
Default → LOW

Luego:
  deny         + cualquier riesgo  → deny
  require_app  + cualquier riesgo  → require_approval
  allow        + high              → require_approval
  allow        + medium/low        → allow
  (no match)   + high              → require_approval
  (no match)   + medium/low        → allow
```

## 9. Break-glass (multi-aprobador)

```
Request llega con break_glass=true (o config lo determina por action_type + risk)
   │
   └──▶ Crear approval con:
   │    break_glass=true
   │    required_approvals=N (configurable)
   │    decisions=[] (JSONB vacío)
   │
   └──▶ Inbox muestra badge "Break Glass" + progreso (ej: "1/3")
   │
   └──▶ Aprobador 1 aprueba:
   │    decisions=[{by: "sre1@co", action: "approve", at: "..."}]
   │    Estado sigue pending (1 < N)
   │
   └──▶ Aprobador 2 aprueba:
   │    decisions=[..., {by: "sre2@co", action: "approve", at: "..."}]
   │    Si 2 == N → approval completada → request aprobada
   │
   └──▶ Si cualquier aprobador RECHAZA:
         decisions=[..., {by: "sre3@co", action: "reject", at: "..."}]
         Approval cancelada → request rechazada

Reglas:
  - El mismo aprobador NO puede decidir dos veces
  - Un rechazo cancela todo (no importa cuántos aprobaron)
  - Aprobación parcial es visible en el Inbox
```

## 10. Feedback loop (execution → risk)

```
POST /v1/requests/{id}/result {success: true/false}
   │
   └──▶ Actualizar request.execution_result + request.status → executed
   │
   └──▶ Actualizar execution_stats para ese action_type:
   │    success → success_count++ , last_success_at = now()
   │    failure → failure_count++ , last_failure_at = now()
   │
   └──▶ Próxima request con ese action_type:
         Factor F5 (execution_history) del cascade risk scoring
         usa success_rate = success_count / (success + failure)
         - success_rate < 50%  → score alto (acción que falla mucho)
         - success_rate > 90%  → score negativo (acción confiable, reduce riesgo)
```

## 11. Simulate (dry-run)

```
POST /v1/requests/simulate
   │
   └──▶ Mismos campos que POST /v1/requests
   │
   └──▶ Evaluar policies CEL + cascade risk scoring
   │    (misma lógica que una request real)
   │
   └──▶ NO persistir, NO crear approval, NO emitir audit events
   │
   └──▶ Retornar: decisión, factores activados, amplificación, score final
```

## 12. Replay simulate (test CEL contra historial)

```
POST /v1/requests/simulate/replay
   │
   └──▶ Recibir expresión CEL propuesta
   │
   └──▶ Evaluar contra historial de requests existentes
   │
   └──▶ Retornar: cuántas habrían matcheado, con qué efecto,
         distribución por action_type/decision
```

## 13. Action types (ontología tipada)

```
CRUD: POST/GET/GET/{id}/PATCH/DELETE /v1/action-types
   │
   └──▶ Cada action type define: name, category, risk_class, schema, reversible, requires_break_glass
   │
   └──▶ 9 action types seeded en migración 0006
   │
   └──▶ Integración en Submit:
         POST /v1/requests → ¿action_type registrado?
         ├── Sí → continuar evaluación normal
         └── No → 403 FORBIDDEN
```

## 14. Delegations (delegation graph)

```
CRUD: POST/GET/GET/{id}/PATCH/DELETE /v1/delegations
   │
   └──▶ Cada delegación define: owner → agent → allowed_action_types → allowed_resources
   │    → purpose → max_risk_class → expires_at
   │
   └──▶ Integración en Submit:
         POST /v1/requests → ¿requester tiene delegación vigente para este action_type?
         ├── Sí (enabled=true, no expirada, action_type en allowed_action_types) → continuar
         └── No → 403 FORBIDDEN
```
