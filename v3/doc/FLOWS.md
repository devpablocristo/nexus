# Nexus v3 — Flujos

## 1. Auto-allow (sin policy match, riesgo bajo)

```
Requester ──POST /v1/requests──▶ Nexus
                                    │
                               Validar + idempotencia
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
Lista de políticas activas (enabled=true, archived_at IS NULL)
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
   └──▶ Si ninguna matchea → decision por riesgo default
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
