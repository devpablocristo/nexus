# Features migradas de v1/v2 a v3

Registro de qué features se trajeron de versiones anteriores para no re-evaluarlas.

## Migradas

| Feature | Origen | Adaptación | Estado |
|---------|--------|-----------|--------|
| **Cascade risk scoring** | v2 `data-plane/internal/action/risk/evaluator.go` | Simplificado: 6 factores (vs 10 en v2), amplificaciones por combinación, sin ML. Determinista y explicable. | ✅ Implementado |
| **CEL policy engine** | v2 `control-plane/internal/policies/evaluator.go` | Mismo patrón: compilar + cachear + evaluar. Sin LRU cache aún. | ✅ Implementado |
| **Hexagonal architecture** | v1/v2 | Mismo patrón: ports & adapters, DI manual, accept interfaces/return structs. | ✅ Implementado |
| **Idempotency** | v2 `data-plane/internal/action/idempotency.go` | Mismo patrón: fingerprint + cache + deduplicación. | ✅ Implementado |
| **Audit trail** | v1/v2 | Append-only events por request. Sin hash-chain (v1 tenía). | ✅ Implementado |
| **Simulation mode** | v1 `usecases_simulate.go` | `POST /v1/requests/simulate` — dry-run que muestra decision, factores de cascada y amplificacion sin persistir. Panel flotante en la consola. | ✅ Implementado |
| **Config module** | Nuevo | Configuracion global via API (`GET/PATCH /v1/config`, `POST /v1/config/reset`) + UI (tab Config). Secciones: risk, approvals, learning, AI, general. | ✅ Implementado |
| **Shadow policies** | v1 `policyproposal` | Campo `mode: enforced/shadow` en policies. Shadow evalúa pero no actúa, incrementa `shadow_hits`. Monitor en Sandbox. Botón "Promote to enforced". | ✅ Implementado |
| **Sandbox (Simulate + Shadow + Replay)** | Nuevo | Tab unificada: simulate request con templates e historial, shadow monitor, replay test contra historial con expresión CEL propuesta. | ✅ Implementado |
| **Feedback loop (execution → risk)** | Nuevo (inspirado en v2 baselines) | `execution_stats` table acumula success/failure por action_type. Factor F5 del cascade usa success_rate real. `ReportResult` alimenta las stats automáticamente. | ✅ Implementado |
| **Break-glass approval** | v1 `approval/usecases.go` | Multi-aprobador: `break_glass: true`, `required_approvals: N`. Aprobación parcial (N-1 no finaliza), rechazo de cualquiera cancela, mismo aprobador no puede decidir dos veces. Configurable por action_type + risk_level. | ✅ Implementado |
| **core backend/databases** | v2 `pkgs/go-pkg/` | Absorbido en `core/backend/go` y `core/databases/postgres/go`; `v3/review` ya consume esos módulos. | ✅ Migrado |

## Evaluadas y descartadas (para PoC/MVP)

| Feature | Origen | Por qué se descartó |
|---------|--------|-------------------|
| Hash-chained audit | v1 | Complejidad vs valor en PoC. Agregar cuando haya requisito de compliance. |
| Execution leases (JWT) | v1/v2 | Sobreingeniería para un solo servicio. Útil si hay multi-service execution. |
| Circuit breaker | v1 | Solo un upstream (PostgreSQL). No hay calls HTTP entre servicios. |
| OIDC + PKCE | v1 | Auth es API key simple. OIDC cuando haya usuarios finales. |
| Redis rate limiting | v1 | No hay Redis en v3. Rate limiting se hará en Go con sliding window. |
| Prometheus + Grafana | v2 | OTel será el approach cuando se necesite. No ahora. |
| DLP detector | v1 | Depende del dominio del cliente. No es core del producto. |
| Multi-service arch | v2 | v3 es monolito modular. Se separa cuando el dolor lo justifique. |
| Canary trap policies | v2 | Interesante pero no prioritario. Agregar post-MVP. |

## Implementadas en Q2 MVP

| Feature | Origen | Adaptación | Estado |
|---------|--------|-----------|--------|
| **Ontología tipada de acciones** | Roadmap unicornio | Tabla `action_types` con name, category, risk_class (low/medium/high/critical), reversible, requires_break_glass, schema (JSON). 9 action types seeded. CRUD completo: POST/GET/GET/{id}/PATCH/DELETE /v1/action-types. Integrada en Submit: action_type desconocido retorna 403 FORBIDDEN. | ✅ Implementado |
| **Delegation graph** | Roadmap unicornio | Tabla `delegations`: owner → agent → allowed_action_types → allowed_resources → purpose → max_risk_class → expires_at. CRUD completo: POST/GET/GET/{id}/PATCH/DELETE /v1/delegations. Integrada en Submit: agente sin delegación vigente retorna 403 FORBIDDEN. | ✅ Implementado |
| **Evidence packs** | Roadmap unicornio | Módulo `evidence/`: genera JSON firmado (HMAC-SHA256) con toda la cadena de una request (request, policy_evaluation, approval, execution, attestation, timeline). Endpoint: `GET /v1/requests/{id}/evidence`. Signing key configurable via `NEXUS_SIGNING_KEY`. Botón de descarga en console. | ✅ Implementado |
| **Outcome attestation** | Roadmap unicornio | Tabla `attestations`: prueba verificable del executor (status, provider_refs, signature, attester, metadata). Endpoints: `POST /v1/requests/{id}/attest` + `GET /v1/requests/{id}/attestation`. Integrada en evidence packs. Nuevo audit event `attested`. | ✅ Implementado |
| **Sandbox avanzado** | Roadmap unicornio | 2 nuevos modos: Batch Test (`POST /v1/requests/simulate/batch`, max 100, resultados agregados) + Approval Sim (`POST /v1/requests/simulate/approval`, simula approve/reject con quorum break-glass). Console: 5 tabs (simulate, batch, approval, shadow, replay). | ✅ Implementado |

## Pendientes de migración (post-MVP)

| Feature | Origen | Prioridad | Notas |
|---------|--------|-----------|-------|
| **CEL program cache LRU** | v2 `evaluator.go` | Media | Copiar el cache de 256 programas |
| **Hysteresis** | v2 `evaluator.go` | Baja | Evitar thrashing entre umbrales |
| **EWMA anomaly detection** | v1 `sentry/worker.go` | Baja | Detección de patrones anómalos en tiempo real |
| **5-tier decisions** | v2 `evaluator.go` | Baja | allow/enhanced_log/additional_auth/require_approval/deny |

## Roadmap Sandbox — Pendientes post-MVP

| Feature | Qué hace | Por qué | Estado |
|---------|----------|---------|----|
| **Simular aprobaciones/rechazos** | Ver qué pasa si apruebo/rechazo una request pendiente | El aprobador necesita ver consecuencias antes de decidir | ✅ Implementado |
| **Escenarios batch** | Ejecutar múltiples simulaciones, ver resultados agregados | Regression testing de policies | ✅ Implementado |
| **Proxy controlado al producto** | Nexus envía la request al producto real, captura la respuesta | Probar la integración real sin comprometerse | Pendiente (Q3) |
| **Evaluación de respuestas** | Analizar la respuesta del producto y sugerir ajustes | Cerrar el loop: si falla → endurecer reglas | Pendiente (Q3) |
| **Snapshot de policies** | Guardar estado de policies y comparar impacto entre versiones | "¿Mejoraron las reglas esta semana?" | Pendiente (Q3) |
