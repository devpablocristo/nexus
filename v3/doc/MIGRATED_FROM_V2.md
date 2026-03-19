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
| **pkgs/go-pkg** | v2 `pkgs/go-pkg/` | Copiado directo: handlers, postgres, apikey, httpserver, observability. | ✅ Copiado |

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

## Pendientes de migración

| Feature | Origen | Prioridad | Notas |
|---------|--------|-----------|-------|
| **Feedback loop (execution → risk)** | Nuevo (inspirado en v2 baselines) | Alta | Resultado de ejecución retroalimenta risk scoring |
| **Shadow policies** | v1 `policyproposal` | Media | Campo `mode: enforced/shadow` en policies |
| **CEL program cache LRU** | v2 `evaluator.go` | Media | Copiar el cache de 256 programas |
| **Break-glass approval** | v1 `approval/usecases.go` | Media | 2+ aprobadores para ops destructivas |
| **Hysteresis** | v2 `evaluator.go` | Baja | Evitar thrashing entre umbrales |
| **EWMA anomaly detection** | v1 `sentry/worker.go` | Baja | Detección de patrones anómalos en tiempo real |
| **5-tier decisions** | v2 `evaluator.go` | Baja | allow/enhanced_log/additional_auth/require_approval/deny |
