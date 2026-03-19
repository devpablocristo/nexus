# Nexus v3 — Roadmap Unicornio

## Tesis

Nexus no es un approval inbox. Es un **control plane soberano** que emite autoridad delegada verificable, fuerza enforcement en runtime y produce evidencia auditable para acciones críticas.

La aprobación es un subproducto. El producto es **autoridad efímera + evidencia verificable**.

---

## Estado actual (PoC completado)

| Primitive | Estado | Implementación |
|-----------|--------|---------------|
| Policy evaluation (CEL) | ✅ Completo | First-match-wins, priority ordering |
| Cascade risk scoring | ✅ Completo | 6 factores + amplificación multiplicativa (coagulación) |
| Feedback loop | ✅ Completo | execution_stats retroalimenta F5 dinámicamente |
| Break-glass | ✅ Completo | Multi-aprobador, no repite, un rechazo cancela |
| Shadow policies | ✅ Completo | Evalúa sin actuar, shadow_hits, promote to enforced |
| Sandbox | ✅ Completo | Simulate + shadow monitor + replay test (backtesting) |
| Learning loop | ✅ Completo | Pattern analyzer → proposals → human accepts → auto-create policy |
| AI contextualizer | ✅ Completo | Claude con fallback |
| Config | ✅ Completo | 5 secciones, 0 hardcoded, API + UI |
| Audit trail | ✅ Completo | Append-only events, replay timeline |
| Ontología tipada | ✅ Completo | action_types con schema, risk_class, 9 seeded, CRUD, verificación en Submit |
| Delegation graph | ✅ Completo | delegations: owner → agent → action_types → resources → TTL, verificación en Submit |
| Console | ✅ Completo | 9 tabs (+ Actions, Agents), i18n EN/ES, Sandbox |

---

## Gap: PoC → Unicornio

### Primitives faltantes (ordenadas por impacto en defensibilidad)

### 1. Execution Leases (JWT firmados)

**Qué es:** Un token firmado criptográficamente que autoriza UNA acción específica con TTL corto. Sin lease válido, el sistema target no puede ejecutar.

**Por qué es el moat:** Hoy Nexus "recomienda" (allow/deny). Con leases, Nexus "habilita". La ejecución depende criptográficamente de Nexus. Eso convierte a Nexus en infraestructura crítica, no en una herramienta opcional.

**Qué ya tenemos:** v1 tenía JWT leases básicos. El patrón existe — hay que rehacerlo con claims estándar (exp/nbf/jti/aud/sub) + action_hash + constraints.

**Implementación:**
```
POST /v1/leases → emite lease firmado
Lease = JWT con: action_type, resource, action_hash, constraints, TTL
PEP valida lease localmente (verificación de firma, no roundtrip)
Lease es single-use (jti tracking)
```

**Esfuerzo:** Alto (2-3 semanas). Requiere key management, signing, verification.

### 2. PEP / SDK (Enforcement Point)

**Qué es:** Un componente que corre cerca del sistema target (sidecar, proxy, o SDK embebido) que intercepta la acción y verifica el lease antes de dejar pasar.

**Por qué importa:** Sin PEP, el sistema target puede ignorar a Nexus. Con PEP, Nexus es un gate físico, no lógico.

**Implementación:**
- SDK Go: `nexus.Authorize(ctx, action) → lease` + `nexus.Execute(ctx, lease, fn) → result`
- SDK Python/JS: misma interfaz
- Proxy HTTP: intercepta requests al target, verifica lease en header

**Esfuerzo:** Alto (3-4 semanas por SDK). Empezar con Go SDK.

### 3. Evidence Packs (exportables y firmados)

**Qué es:** Empaquetar el audit trail de una request como un JSON estructurado, firmado, exportable. Incluye: actores, request, policy evaluation, approvals, lease, execution, outcome attestation.

**Por qué importa:** Es lo que el auditor/regulador quiere ver. Transforma "tenemos logs" en "tenemos evidencia verificable".

**Qué ya tenemos:** El audit trail tiene toda la data. Solo falta estructurarla como pack + firmar.

**Implementación:**
```
GET /v1/requests/{id}/evidence → JSON firmado con toda la cadena
Campos: actors, request, policy_evaluation, approvals, lease, execution, attestation
Opcionalmente: hash-chain entre events (como v1)
```

**Esfuerzo:** Medio (1-2 semanas). Es mayormente formateo + firma.

### 4. Outcome Attestation

**Qué es:** El sistema target (o el PEP) reporta qué ejecutó realmente, firmado. No es solo "success: true" — es una prueba verificable de qué pasó.

**Por qué importa:** Sin esto, Nexus sabe que aprobó algo pero no puede demostrar qué pasó después. Con attestation, la cadena es completa: request → decision → lease → execution → proof.

**Qué ya tenemos:** `POST /v1/requests/{id}/result` recibe success/failure. Falta firma y refs del provider.

**Implementación:**
```
POST /v1/requests/{id}/attest
{
  "status": "success",
  "provider_refs": {"tx_id": "bank_tx_555"},
  "signature": "jws_or_hash",
  "attester": "pep:treasury_gateway"
}
```

**Esfuerzo:** Medio (1-2 semanas).

### 5. Delegation Graph -- IMPLEMENTADO (Q2 MVP)

**Qué es:** Modelar explícitamente quién delega qué a quién: `owner → agent → action_types → resources → TTL`. Sin esto, Nexus trata a todos los requesters como iguales.

**Por qué importa:** Un agente debería tener permisos acotados a lo que su owner le delegó. No es lo mismo "ops-bot del team finops" que "ops-bot del team infra".

**Implementación (completada):**
- Tabla `delegations`: owner_id, owner_type, agent_id, agent_type, allowed_action_types (JSONB), allowed_resources (JSONB), purpose, max_risk_class, expires_at, enabled
- CRUD: POST/GET/GET/{id}/PATCH/DELETE /v1/delegations
- Integrado en Submit: agente sin delegación vigente → 403 FORBIDDEN
- UI: pestaña "Agents" en la consola

### 6. Ontología Tipada de Acciones -- IMPLEMENTADO (Q2 MVP)

**Qué es:** `treasury.transfer`, `iam.grant_role`, `infra.deploy` como tipos de primer nivel con schema validado, no strings libres.

**Por qué importa:** Permite expansión por ontología (agregar un action_type nuevo es agregar un schema, no código). Y permite que las policies sean más precisas.

**Implementación (completada):**
- Tabla `action_types`: name (unique), description, category, risk_class (low/medium/high/critical), schema (JSONB), reversible, requires_break_glass, enabled
- 9 action types seeded: alert.silence, alert.escalate, runbook.execute, incident.resolve, config.update, deploy.trigger, delete, iam.grant_role, treasury.transfer
- CRUD: POST/GET/GET/{id}/PATCH/DELETE /v1/action-types
- Integrado en Submit: action_type desconocido → 403 FORBIDDEN
- UI: pestaña "Actions" en la consola

---

## Roadmap MVP → Producción → Unicornio

### Q2 2026 — MVP (COMPLETADO)

| Semana | Entregable | Estado |
|--------|-----------|--------|
| 1-2 | Ontología tipada de acciones (schema + validación) | ✅ Completo |
| 3-4 | Delegation graph (tabla + verificación en Submit) | ✅ Completo |
| 5-6 | Evidence packs (export JSON firmado) | Pendiente |
| 7-8 | Outcome attestation (attest endpoint + firma) | Pendiente |
| 8 | Sandbox avanzado: simular aprobaciones, batch | Pendiente |

**Resultado parcial:** Nexus tiene identidad de agentes y acciones tipadas. Falta evidencia exportable para pilotos enterprise.

### Q3 2026 — Enforcement (el moat)

| Semana | Entregable |
|--------|-----------|
| 1-3 | Execution leases (JWT firmados, key management) |
| 4-6 | PEP/SDK Go v1 (authorize + execute + attest) |
| 7-8 | Integración vertical #1 (treasury o IAM) |
| 9-10 | PEP proxy HTTP (para sistemas sin SDK) |

**Resultado:** Nexus pasa de "recomendar" a "habilitar/impedir". El moat está construido.

### Q4 2026 — Enterprise

| Semana | Entregable |
|--------|-----------|
| 1-4 | Multi-tenant / BYOK / self-hosted option |
| 5-8 | Compliance packs (AI Act, NIST, SR 11-7) |
| 9-10 | 2da integración vertical |
| 11-12 | Performance hardening, SLOs, OTel |

**Resultado:** Vendible a enterprise con compliance y soberanía.

### Q1 2027 — Escala

- Marketplace de conectores
- SDK Python + JS
- 3ra vertical
- ML: anomaly detection, policy recommendations

---

## Verticales (en orden de prioridad)

| # | Vertical | Primer action_type | Por qué primero |
|---|----------|-------------------|-----------------|
| 1 | **Treasury / crypto** | `treasury.transfer` | Pérdida cuantificable, regulación fuerte, ROI inmediato |
| 2 | **Privileged access** | `iam.grant_role` | El error escala a breach, NHI es tema caliente |
| 3 | **Infra / prod changes** | `infra.deploy` | Backtesting vendible, DevOps ya entiende policy-as-code |
| 4 | **Incident response** | `incident.remediate` | Expansión natural desde ops |

---

## Lo que NO hacer

- No construir un IdP (Okta/Entra ya lo hacen)
- No construir un workflow engine (ServiceNow ya lo hace)
- No construir un framework de agentes (OpenAI/LangChain ya lo hacen)
- No perseguir todas las verticales a la vez
- No vender "AI approvals" — vender "autoridad soberana + evidencia verificable"

---

## Frase de posicionamiento

> **Nexus es el plano soberano que emite permisos efímeros y evidencia verificable para acciones críticas ejecutadas por agentes y automatizaciones.**

No es un approval inbox. No es un policy engine. No es un audit log. Es las tres cosas juntas, con enforcement criptográfico y pruebas de ejecución.
