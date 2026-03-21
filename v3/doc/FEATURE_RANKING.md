# Feature Ranking: v1 + v2 → Review v3

Todas las funcionalidades de Nexus v1 y v2, ordenadas desde "imprescindible" hasta "descartar". Contexto: Review v3 es un PoC para monitoring/incident response con 3 personas.

## TIER 1: Imprescindible (implementar en el PoC)

| # | Feature | Origen | Por que es imprescindible |
|---|---------|--------|--------------------------|
| 1 | **CEL policy engine** | v2 | Sin esto no hay evaluacion de reglas. Es el core del producto. |
| 2 | **Audit trail append-only** | v1+v2 | Sin esto no hay replay. El replay es el segundo diferenciador despues de la IA. |
| 3 | **Approval workflow (basico)** | v1+v2 | Sin approval no hay producto. La accion se frena, un humano decide. |
| 4 | **AI contextualizer (Claude)** | nuevo | Es EL diferenciador. Sin IA, es un webhook con formulario. Con IA, un SRE decide en 10 segundos. |
| 5 | **Idempotency-Key** | v2 | Los agentes reintentan. Sin dedup, acciones duplicadas. Especialmente critico para alert.silence. |
| 6 | **API key auth para agentes** | v2 | Sin auth, cualquiera propone acciones. SHA256 hash, probado en v2. |
| 7 | **Arquitectura hexagonal (ports & adapters)** | v2 | Permite testear sin DB ni Claude, swappear adapters. Base de calidad del codigo. |
| 8 | **Health endpoints (healthz, readyz)** | v2 | Minimo para operar. readyz = DB ping. |
| 9 | **Structured logging (slog JSON)** | v2 | Sin logs estructurados no se puede debuggear en produccion. |
| 10 | **Approval inbox UI (React, minima)** | nuevo | Sin UI no hay demo. La demo es lo que vende. |

## TIER 2: Muy importante (implementar en v1 completo, semanas 5-10)

| # | Feature | Origen | Por que es muy importante |
|---|---------|--------|--------------------------|
| 11 | **Hash-chained audit** | v1 | Prueba criptografica de no-alteracion. Cuando el cliente pregunta "como se que no editaron el log?" |
| 12 | **Policy proposals (IA sugiere reglas)** | v1 | ✅ Implementado. Loop de feedback: deteccion de patrones → propuestas → humano acepta → auto-crear policy. |
| 13 | **Break-glass approval** | v1 | ✅ Implementado. Multi-aprobador: `break_glass: true`, `required_approvals: N`. Un rechazo cancela todo, mismo aprobador no puede decidir dos veces. Configurable por action_type + risk_level. |
| 14 | **Rate limiting por agente** | v1 | Agente con bug manda 10K propuestas/minuto. Sliding window en memoria, sin Redis. |
| 15 | **Background job: expiracion de approvals** | nuevo | Sin esto, approvals pendientes quedan para siempre. Job cada 1min que expira TTL vencidos. |
| 16 | **DegradationCollector per-request** | v2 | Marca `ai_degraded: true` cuando Claude no respondio. El SRE sabe que decide sin contexto IA. |
| 17 | **Canary agents** | v2 adaptado | Agente de prueba que propone acciones falsas. Si pasan, las reglas tienen huecos. Cero codigo especial. |
| 18 | **Dashboard (metricas agregadas)** | nuevo | ✅ Implementado. `GET /v1/metrics/summary` + tab Dashboard en consola. |
| 19 | **Prometheus metrics (RED)** | v2 | Requests, errors, duration. `core/backend/go/observability` ya lo tiene. |
| 20 | **JWT auth para UI** | nuevo | Los aprobadores necesitan auth real, no solo API key. |

## TIER 3: Importante (implementar en v1.1 o v1.2)

| # | Feature | Origen | Por que es importante |
|---|---------|--------|----------------------|
| 21 | **SDK Python** | v1 | 80% de los agentes son Python. `nexus.propose(...)` es mas facil que construir HTTP a mano. |
| 22 | **Webhooks de notificacion** | nuevo | Notificar al agente cuando su accion fue aprobada/rechazada, en vez de polling. |
| 23 | **Audit export CSV** | v1 | Para compliance. "Exportame todo lo que hizo el bot en marzo." |
| 24 | **Conectores de lectura** | nuevo | Leer estado de alerta de PagerDuty/Datadog para enriquecer el AI summary. Mejora mucho la calidad. |
| 25 | **Multi-tenancy basico (org_id)** | v2 saas | Para vender SaaS a multiples clientes. org_id en todas las tablas (ya preparado en schema). |
| 26 | **Billing (Stripe)** | v1+v2 saas | Para cobrar. Starter $1.5K, Growth $4K. |
| 27 | **Slack/PagerDuty alert routing** | v2 workers | Cuando una accion es denegada o expira, notificar por Slack/PagerDuty ademas del inbox. |
| 28 | **Security headers middleware** | v2 | X-Content-Type-Options, X-Frame-Options, Referrer-Policy. Cero esfuerzo, viene en `core/backend/go/httpserver`. |
| 29 | **Graceful shutdown** | v2 | Signal handling, connection draining. Necesario para deploys sin downtime. |
| 30 | **Request ID propagation (X-Request-Id)** | v2 | Tracing end-to-end. Viene en `core/backend/go/observability`. |

## TIER 4: Nice-to-have (implementar si hay tiempo)

| # | Feature | Origen | Valor |
|---|---------|--------|-------|
| 31 | **SDK Go** | v1 | Para agentes escritos en Go. Menos prioritario que Python. |
| 32 | **SDK TypeScript** | v1 | Para agentes en Node.js. Menos comun en ops. |
| 33 | **Simulacion de politicas** | v2 Fase 1C | ✅ Implementado como `POST /v1/requests/simulate` + panel flotante en consola |
| 34 | **Replay de incidentes** | v2 Fase 1C | ✅ Implementado como `POST /v1/requests/simulate/replay` — evalua expresion CEL propuesta contra historial real |
| 35 | **Backtest de politicas** | v2 Fase 1C | "Si bajo el threshold, cuantas mas acciones se frenan?" |
| 36 | **Risk tiering configurable** | nuevo | ✅ Implementado via config module (API + UI). El usuario configura action_types high/medium/low. |
| 37 | **Approval TTL configurable por policy** | v2 | Diferentes TTLs segun la politica (5min para criticos, 1h para bajos). |
| 38 | **Grafana dashboards pre-provisioned** | v2 | Dashboard listo con metricas de Review. |
| 39 | **Docker Compose con observability** | v2 | Compose con Prometheus + Grafana + exporters. |
| 40 | **Terraform IaC** | v2 | AWS deploy automatizado. No para MVP, si para escalar. |

## TIER 5: Futuro lejano (no implementar ahora, guardar como referencia)

| # | Feature | Origen | Cuando tiene sentido |
|---|---------|--------|---------------------|
| 41 | **Risk cascade multi-factor** | v2 Fase 1A | ✅ Implementado: 6 factores + amplificacion multiplicativa (coagulacion). |
| 42 | **Baselines estadisticas** | v2 Fase 1A | Cuando haya suficiente volumen para que las baselines tengan confidence. |
| 43 | **Hysteresis en decision bands** | v2 Fase 1A | Solo con cascada continua. Review usa tiering discreto. |
| 44 | **Amplificaciones no-lineales** | v2 Fase 1A | ✅ Implementado como parte de la cascada (combinaciones de factores con multiplicadores). |
| 44b | **Ontologia tipada de acciones** | Roadmap unicornio | ✅ Implementado (Q2 MVP): tabla action_types, 9 seeded, CRUD 5 ops, verificacion en Submit (403). |
| 44c | **Delegation graph** | Roadmap unicornio | ✅ Implementado (Q2 MVP): tabla delegations, CRUD 5 ops, verificacion en Submit (403). |
| 45 | **Confidence saturation** | v2 Fase 1A | Solo con baselines. |
| 46 | **Bucketed counters (sliding windows)** | v2 Fase 1B | "Mas de 10 silences en 2 horas → bloquear." Util pero no MVP. |
| 47 | **Multi-step approvals (4-eyes, quorum)** | v2 Fase 1B | ✅ Implementado parcialmente via break-glass (multi-aprobador con required_approvals configurable). Falta quorum flexible. |
| 48 | **Resource groups** | v2 Fase 1B | Para agrupar alertas/servicios. No necesario para el wedge. |
| 49 | **Execution leases (ephemeral tokens)** | v2 | Solo si Review ejecuta acciones en nombre del agente. Hoy el agente ejecuta. |
| 50 | **Caching con soft/hard TTL + degradation** | v2 | Solo para microservicios. Review es monolito. |
| 51 | **Circuit breakers per-tool** | v1 | Para ejecucion de tools. Review no ejecuta. |
| 52 | **DLP (Data Loss Prevention)** | v1 | Redaccion de PII en inputs. No relevante para alert.silence params. |
| 53 | **AI assistant chat** | v1 | Chat interactivo con Nexus. Demasiado ambicioso para PoC. |
| 54 | **Response adaptation (inflammation, fever, lockdown)** | v2 Fase 2 | Respuesta automatica proporcional. Fascinante pero post-PMF. |
| 55 | **Resource graph** | v2 Fase 3 | Propagacion de señales entre recursos. Post-PMF. |
| 56 | **Adaptive layer (antibodies)** | v2 Fase 4 | Auto-generar policies de incidentes confirmados. Peligroso, necesita confianza. |

## TIER 6: Descartar para Review (no aplicable o contraproducente)

| # | Feature | Origen | Por que descartar |
|---|---------|--------|------------------|
| 57 | **Egress control / SSRF protection** | v1 | Review no hace HTTP calls a tools externos. El agente ejecuta. |
| 58 | **Secrets management (credential injection)** | v1 | Review no inyecta credenciales. El agente tiene sus propias. |
| 59 | **MCP (Model Context Protocol)** | v1 | Protocolo para tool invocation. Review no invoca tools. |
| 60 | **A2A (App-to-App) integration** | v1 | Comunicacion directa entre servicios. Review es monolito. |
| 61 | **Action overrides** | v1 | Override de comportamiento per-tool. Review no tiene tools. |
| 62 | **Tool registry (CRUD de tools)** | v1 | Review no registra tools — registra agentes y politicas. |
| 63 | **Input/output JSON schema validation** | v1 | Para validar schemas de tools. Review valida propuestas, no schemas de tools. |
| 64 | **Lease credential brokerage** | v1 | Injectar credenciales via lease. No aplica. |
| 65 | **EWMA anomaly detection (sentry worker)** | v1 | Deteccion de anomalias en tool calls. Review no ejecuta tools. |
| 66 | **Coordinator / Mitigation / Recovery workers** | v1 | Orquestacion de remediacion automatica. Review no remedia — aprueba. |
| 67 | **Multi-instance signaling** | v2 Fase 5 | Deteccion distribuida. Review es single-instance. |
| 68 | **Tower (full web dashboard)** | v1 | Dashboard completo de v1 (React). Demasiado grande. Review tiene UI minima propia. |
| 69 | **Clerk identity provider** | v1+v2 | Overkill para Review v1. JWT simple basta. Clerk se agrega con multi-tenancy. |
| 70 | **Redis** | v1 | v1 usa Redis para rate limiting e idempotency. Review usa PostgreSQL y memoria. |

## Resumen

| Tier | Cantidad | Cuando |
|------|----------|--------|
| 1: Imprescindible | 10 | PoC (semanas 1-4) |
| 2: Muy importante | 10 | v1 completo (semanas 5-10) |
| 3: Importante | 10 | v1.1 / v1.2 |
| 4: Nice-to-have | 10 | Si hay tiempo |
| 5: Futuro lejano | 18 | Cuando el producto lo pida (2 ya implementadas: ontologia, delegations) |
| 6: Descartar | 14 | No aplica a Review |
| **Total** | **72** | |
