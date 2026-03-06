# Nexus — Documentación técnica

Nexus es un gateway de control para que agentes de IA ejecuten herramientas (APIs HTTP) de forma segura y gobernada. En lugar de que un agente llame directo a una API (pagos, CRM, etc.), llama a Nexus. Nexus decide si permite o no la ejecución, aplica reglas y límites, y registra todo.

## Suite oficial de prompts

La suite oficial para diseño e implementación vive en `docs/prompts/` y debe leerse empezando por `docs/prompts/00_base_transversal.md`.

Orden oficial:
- `00_base_transversal.md`
- `01_user_identity_clerk_aws.md` a `10_production_hardening_final.md`
- `11_ai_runtime_prompting_eval.md` a `17_architecture_decision_records.md`

Todo prompt posterior hereda las invariantes de `00`: separación `core` vs `saas`, no LLM en enforcement, operators sin writes directos a DB, contratos primero, observabilidad, seguridad y testing.

---

## Arquitectura

```
Consumer / Agent
       │
       ▼
  ┌──────────┐        ┌────────────┐
  │nexus-core│◄──────►│ nexus-saas │
  │ (gateway)│        │  (SaaS API)│
  └────┬─────┘        └──────┬─────┘
       │                     │
       │  HTTP (upstream)    │  HTTP
       ▼                     ▼
  ┌──────────┐   ┌───────────────────────┐   ┌──────────────────┐
  │  Tools   │   │nexus-control-operators│   │nexus-ai-operators│
  │(backends)│   │   (Go, determinista)  │   │  (Python, IA)    │
  └──────────┘   └───────────────────────┘   └──────────────────┘
                          │
                          ▼
                    ┌───────────┐
                    │nexus-tower│
                    │   (UI)    │
                    └───────────┘
```

---

## Servicios

### 1. nexus-core (Go)

Gateway principal (data plane). Recibe requests de ejecución y aplica controles síncronos.

- Recibe pedidos: `POST /v1/run`, `POST /v1/run/simulate`, MCP (`POST /mcp`), A2A (`POST /a2a/call`).
- Pipeline de 19 pasos: auth, policies, DLP, rate limits, circuit breaker, egress/SSRF, idempotencia, timeout budget, ejecución HTTP, auditoría.
- Puede requerir aprobación humana (HITL) antes de ejecutar.
- Audita cada intento (allow/deny/error) con hash-chain.
- Módulo: `nexus-core`, Go 1.24, Gin, GORM, PostgreSQL, Redis.

### 2. nexus-saas (Go)

Capa SaaS multi-tenant. Gestiona entidades de dominio, identidad OIDC, y funcionalidades avanzadas.

- Orgs y API keys (`POST /v1/orgs`).
- Eventos operacionales (`GET /v1/events`).
- Acciones (`POST /v1/actions/apply`, `POST /v1/actions/rollback`).
- Incidentes (`POST /v1/incidents`, `GET /v1/incidents`).
- Reglas de alerta (`/v1/alert-rules`).
- Propuestas de políticas (`/v1/policy-proposals`).
- Sesiones de agente (`GET /v1/sessions/:session_id`).
- Asistente y proxy a AI operators (`POST /v1/assistant/query`).
- OIDC completo con PKCE (`/v1/auth/oidc/*`).
- Usage metering por org.
- Proxy a nexus-core para audit, approvals, openapi.
- Contratos internos para nexus-core: entitlements, runtime-overrides, usage events.
- Módulo: `nexus-saas`, Go 1.24, Gin, GORM, PostgreSQL.

### 3. nexus-control-operators (Go)

Plano de control determinista. Consume eventos del eventstore de nexus-core y ejecuta respuestas automáticas. No forma parte del path síncrono de `/v1/run`.

- **Sentry**: detección de anomalías con EWMA (error rate, latency) por tool. Abre incidentes.
- **Coordinator**: máquina de estados de incidentes (OPEN → DIAGNOSING → MITIGATING → MONITORING → RESOLVED/ESCALATED).
- **Mitigation**: ejecuta dry-run y aplica acciones automáticamente.
- **Recovery**: monitorea post-mitigación. Rollback si las condiciones no mejoran.
- Persistencia en JSON atómicos (`offsets.json`, `sentry_state.json`, `proposals.json`, `recovery_tracks.json`).
- Métricas Prometheus (`nexus_operators_*`).

### 4. nexus-ai-operators (Python)

Operadores asistidos por IA. Consume APIs de Nexus (sin acceso directo a DB).

- Engine loop: consume eventos, calcula señales, aplica acciones temporales.
- Backends LLM: `anthropic`, `ollama`, `fallback` (stub determinista, default).
- Playbooks: throttle, incidents, policy proposals.
- Risk scorer: LOW / MED / HIGH / CRIT según deny_ratio.
- Módulo: FastAPI, Python 3.12, Prometheus.

### 5. nexus-tower (React)

Panel de control web.

- **Tools**: CRUD de tools, egress rules, policies por tool. Selección de tool activa global.
- **Audit Log**: log de requests con filtros (decision, status). Filtro por tool activa.
- **Monitoring**: dashboards Grafana embebidos con filtro por tool.
- Tech: React 18, TypeScript, Vite, TanStack Query, Recharts.
- Se comunica con nexus-core vía `VITE_NEXUS_CORE_URL`.

### 6. SDKs

| SDK | Lenguaje | Features |
|-----|----------|----------|
| `sdks/python-sdk` | Python | `NexusClient` (sync), `AsyncNexusClient` (async), integración LangChain (`NexusTool`, `NexusToolkit`), integración OpenAI Agents SDK (`nexus_function_tools`) |
| `sdks/typescript-sdk` | TypeScript | `NexusClient`: run, simulate, tools, policies, audit, approvals |

### Exposición de métricas (`/metrics`)

- `nexus-ai-operators`: requiere `X-Operator-Key` para acceder a `/metrics`.
- `nexus-core` y `nexus-saas`: endpoints de métricas son para scraping interno (Prometheus en red privada).
- En producción, no exponer `/metrics` públicamente por Internet.

### 7. Paquetes compartidos (pkgs/)

| Paquete | Descripción |
|---------|-------------|
| `pkgs/go-pkg` | Módulo Go compartido (`nexus/pkg`): HTTP middlewares (Gin), errors, types, utils (AES-GCM, redact, SSRF, canonical JSON, SHA256), JSON Schema validation, OpenTelemetry, GORM helpers |
| `pkgs/contracts` | `error-codes.json`, `events.schema.json`, `openapi.nexus-core.snapshot.yaml` |
| `pkgs/python-pkg` | `NexusCoreClient`: cliente HTTP para operadores (list_events, apply_action, create_incident, create_policy_proposal) |
| `pkgs/typescript-pkg` | `NexusCoreClient`: cliente TS para operadores (listEvents, applyAction, listIncidents, listPolicyProposals) |

---

## Flujo de request (POST /v1/run)

El pipeline completo solo aplica a `POST /v1/run` y `POST /v1/run/simulate`. Otras rutas pasan por auth y su handler directo.

| Paso | Nombre | Qué hace |
|------|--------|----------|
| 1 | **Auth** | Valida API key o JWT, extrae org_id, scopes, actor, role |
| 2 | **Authz** | Verifica scope `gateway:run` |
| 3 | **Tool resolution** | Busca la tool por `tool_id` o `tool_name` |
| 4 | **Idempotency** | Si es write tool y viene `Idempotency-Key`, verifica replay/conflicto |
| 5 | **Tool validation** | Verifica que la tool esté `enabled` y sea `kind: http` |
| 6 | **Context enrichment** | Inyecta actor, role, scopes, auth_method al context |
| 7 | **DLP** | Escanea input y context buscando datos sensibles |
| 8 | **Schema validation** | Valida `input` contra JSON Schema de la tool |
| 9 | **Policy evaluation** | Evalúa policies → `allow` o `deny` (first-match por prioridad) |
| 10 | **Approval check** | Si policy tiene `require_approval`, bloquea con `APPROVAL_REQUIRED` |
| 11 | **Action overrides** | Verifica overrides runtime activos (kill switch por tool) |
| 12 | **Tenant rate limit** | Rate limit global del tenant |
| 13 | **Tool rate limit** | Rate limit específico de la tool |
| 14 | **URL + Egress** | SSRF protection + verifica host en allowlist |
| 15 | **Secrets injection** | Carga secrets cifrados e inyecta como headers |
| 16 | **Timeout budget** | Verifica que quede tiempo |
| 17 | **HTTP execution** | Llama al upstream (circuit breaker per-host) |
| 18 | **Output schema** | Valida respuesta del upstream (best-effort) |
| 19 | **Audit** | Graba en audit log (hash-chained, redacción DLP) |

### Payload de request

```json
{
  "request_id": "opcional-uuid",
  "tool_name": "my-service",
  "tool_id": "5dca11d1-...",
  "input": { "msg": "hello" },
  "context": { "source": "my-agent", "session_id": "abc-123" }
}
```

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `request_id` | string | No | UUID para tracing. Si no se envía, Nexus genera uno |
| `tool_name` | string | Uno de los dos | Nombre de la tool (inmutable) |
| `tool_id` | string | Uno de los dos | UUID de la tool. Si ambos vienen, `tool_id` gana |
| `input` | object | Sí | Payload que recibe la tool. Se valida contra JSON Schema |
| `context` | object | No | Metadata del consumer. Usado por policies y DLP |

### Headers

| Header | Descripción |
|--------|-------------|
| `X-NEXUS-CORE-KEY` | API key (si auth por API key) |
| `Authorization` | `Bearer <JWT>` (si auth por JWT) |
| `X-NEXUS-SCOPES` | Scopes del consumer (mínimo `gateway:run`) |
| `X-NEXUS-ACTOR` | Identidad del llamante |
| `X-NEXUS-ROLE` | Rol del llamante |
| `X-Timeout-Ms` | Budget de timeout en ms |
| `Idempotency-Key` | Clave de idempotencia (para write tools) |

---

## Controles y reglas en una ejecución

### Autenticación
- **API key** (`X-NEXUS-CORE-KEY`): hash SHA256 contra `org_api_keys`. Scopes desde DB.
- **JWT** (`Authorization: Bearer`): verificación JWKS. Claims: `org_id`, `role`, `scopes`, `sub`.
- **OIDC**: flujo OAuth2 + PKCE (discovery, authorize, callback) en nexus-saas.
- Sin key válida → 401. Sin scope requerido → 403. Roles `admin`/`secops` bypasean scopes.

### Scopes disponibles
`tools:read`, `tools:write`, `policy:read`, `policy:write`, `egress:read`, `egress:write`, `audit:read`, `gateway:run`, `gateway:simulate`, `mcp:read`, `mcp:call`, `a2a:call`, `admin:console:read`, `admin:console:write`, `admin:secrets`.

### Idempotencia (solo tools write)
- Policy exige `require_idempotency` sin `Idempotency-Key` → 400.
- Misma key, mismo payload → replay (no reejecutar upstream).
- Misma key, distinto payload → 409 conflict.
- Misma key en progreso → 409 in progress.
- Misma key con FAILED → replay terminal del error.

### DLP
Detecta PII en input y context: email, phone, credit_card, jwt, api_key, national_id. Resultado en `context.dlp` para policies.

### Políticas (Policy DSL)
- First-match por prioridad.
- Condiciones + efecto (`allow`/`deny`).
- Paths: `input.*`, `context.*`, `tool.*`.
- Operadores: `exists`, `not_exists`, `eq`, `neq`, `lt`, `lte`, `gt`, `gte`, `in`, `contains`, `regex`.
- Composición: `all`, `any`, `not`.
- Límites: `rate_limit.per_minute`, `max_bytes_input`, `max_bytes_context`, `require_idempotency`, `require_approval`.
- Default: read → allow; write → deny.

### Aprobación humana (HITL)
- Policy con `require_approval: true` → bloquea con `APPROVAL_REQUIRED`.
- Registro en `pending_approvals` con TTL.
- Aprobar/rechazar: `POST /v1/approvals/:id/approve` o `/reject`.
- Approvals vencidas se expiran por `cmd/cleanup-idempotency`.

### Rate limits
- Tenant: RPM global configurable.
- Tool: RPM desde policy o action override.
- Backend: in-memory o Redis (`NEXUS_RATE_LIMIT_BACKEND`).

### Circuit breaker
- Per-host upstream: closed → open (tras N fallos) → half-open (N requests de prueba).
- Config: `NEXUS_CB_FAILURE_THRESHOLD`, `NEXUS_CB_HALF_OPEN_MAX`, `NEXUS_CB_RESET_TIMEOUT_SEC`.

### Egress y SSRF
- Allowlist de hosts por tool.
- SSRF activo: bloquea IPs privadas, loopback, link-local, metadata (169.254.169.254), IPv6 ULA.
- Desactivable con `NEXUS_DISABLE_SSRF_PROTECTION=true` (solo dev/test).

### Timeout budget
- Presupuesto de tiempo consumido por etapas (policy, egress, execution).
- Si se agota → 408 timeout budget exhausted.
- Config: `NEXUS_TIMEOUT_BUDGET_DEFAULT_MS` (10000), `NEXUS_TIMEOUT_BUDGET_MIN_MS`, `NEXUS_TIMEOUT_BUDGET_MAX_MS` (30000).

### Auditoría
- Cada intento (allow/deny/error) se graba con hash-chain.
- Redacción de datos sensibles detectados por DLP.
- Export en CSV y JSONL con hash-chain integro.

### Action overrides (runtime)
- nexus-saas aplica overrides temporales (deny, RPM reducido) por tool u org.
- nexus-core consulta `/internal/runtime-overrides/:org_id/:tool_name` en cada request.

---

## Endpoints por servicio

### nexus-core

#### Públicos (sin auth)
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/healthz` | Health check |
| GET | `/readyz` | Readiness (ping DB) |
| GET | `/docs` | Swagger UI |
| POST | `/v1/orgs` | Crear org + API key (onboarding) |
| GET | `/v1/auth/oidc/config` | Estado OIDC |
| GET | `/v1/auth/oidc/authorize` | Inicio flujo OIDC |
| GET | `/v1/auth/oidc/callback` | Callback OIDC |

#### Autenticados
| Método | Ruta | Descripción |
|--------|------|-------------|
| POST | `/v1/run` | Ejecutar tool via gateway |
| POST | `/v1/run/simulate` | Simular ejecución (sin upstream) |
| GET | `/v1/tools` | Listar tools |
| POST | `/v1/tools` | Crear tool |
| GET | `/v1/tools/:name` | Detalle de tool |
| PUT | `/v1/tools/:name` | Actualizar tool |
| DELETE | `/v1/tools/:name` | Eliminar tool |
| GET | `/v1/tools/:name/policies` | Listar policies de tool |
| POST | `/v1/tools/:name/policies` | Crear policy |
| PUT | `/v1/policies/:id` | Actualizar policy |
| GET | `/v1/tools/:name/egress-rules` | Listar egress rules |
| POST | `/v1/tools/:name/egress-rules` | Crear egress rule |
| DELETE | `/v1/tools/:name/egress-rules` | Eliminar egress rule |
| GET | `/v1/tools/:name/secrets` | Listar secrets |
| POST | `/v1/tools/:name/secrets` | Crear/actualizar secret |
| DELETE | `/v1/tools/:name/secrets` | Eliminar secret |
| GET | `/v1/audit` | Consultar audit trail |
| GET | `/v1/audit/export` | Export audit (CSV/JSONL) |
| GET | `/v1/approvals` | Listar approvals |
| GET | `/v1/approvals/:id` | Detalle de approval |
| POST | `/v1/approvals/:id/approve` | Aprobar |
| POST | `/v1/approvals/:id/reject` | Rechazar |
| POST | `/mcp` | MCP JSON-RPC (tools/list, tools/get, tools/call) |
| POST | `/a2a/call` | Agent-to-Agent tool call |

#### Internos (operator proxy, `X-NEXUS-AI-KEY`)
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/internal/operators/events` | Proxy a eventos SaaS |
| POST | `/internal/operators/events/append` | Append evento |
| POST | `/internal/operators/actions/apply` | Apply acción |
| POST | `/internal/operators/incidents` | Crear incidente |
| POST | `/internal/operators/policy-proposals` | Crear propuesta |

### nexus-saas

#### Públicos
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/health`, `/healthz`, `/readyz` | Health checks |
| POST | `/v1/orgs` | Crear org (onboarding) |
| GET | `/v1/auth/oidc/*` | Flujo OIDC (config, authorize, callback) |

#### Autenticados
| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/v1/events` | Listar eventos operacionales |
| POST | `/v1/actions/apply` | Aplicar acción |
| POST | `/v1/actions/rollback` | Rollback acción |
| GET | `/v1/actions` | Listar acciones |
| POST | `/v1/incidents` | Crear incidente |
| GET | `/v1/incidents` | Listar incidentes |
| GET | `/v1/incidents/:id` | Detalle incidente |
| POST | `/v1/incidents/:id/close` | Cerrar incidente |
| GET | `/v1/alert-rules` | Listar reglas de alerta |
| POST | `/v1/alert-rules` | Crear regla de alerta |
| DELETE | `/v1/alert-rules/:id` | Eliminar regla |
| GET | `/v1/sessions/:session_id` | Sesión de agente |
| POST | `/v1/policy-proposals` | Crear propuesta de política |
| GET | `/v1/policy-proposals` | Listar propuestas |
| POST | `/v1/policy-proposals/:id/approve` | Aprobar propuesta |
| POST | `/v1/policy-proposals/:id/reject` | Rechazar propuesta |
| POST | `/v1/policy-proposals/:id/shadow` | Shadow propuesta |
| POST | `/v1/assistant/query` | Query al asistente; proxy a `nexus-ai-operators` |
| POST | `/v1/assistant/tick` | Tick del asistente |
| GET | `/v1/admin/bootstrap` | Bootstrap admin |
| GET/PUT | `/v1/admin/tenant-settings` | Tenant settings |
| GET | `/v1/admin/activity` | Actividad admin |
| GET | `/v1/audit`, `/v1/audit/export` | Proxy a nexus-core |
| GET/POST | `/v1/approvals/*` | Proxy a nexus-core |

#### Contratos internos (`X-NEXUS-SAAS-KEY`)
| Método | Ruta | Descripción |
|--------|------|-------------|
| POST | `/internal/usage/events` | Ingesta de uso |
| POST | `/internal/events` | Ingesta de eventos |
| GET | `/internal/entitlements/:org_id` | Entitlements por org |
| GET | `/internal/runtime-overrides/:org_id/:tool_name` | Runtime overrides |

### nexus-control-operators

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/healthz` | Liveness |
| GET | `/readyz` | Readiness (conectividad con nexus-core) |
| GET | `/metrics` | Métricas Prometheus |

### nexus-ai-operators

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/healthz` | Liveness |
| GET | `/readyz` | Readiness |
| GET | `/metrics` | Métricas Prometheus |
| POST | `/v1/internal/tick` | Tick manual (`X-Operator-Key`) |
| POST | `/v1/assistant/query` | Query al asistente (`X-Operator-Key`) |

---

## nexus-control-operators — Detalle

### Workers

| Worker | Consumer Group | Responsabilidad |
|--------|----------------|-----------------|
| **Sentry** | `agents.sentry.v1` | Detección EWMA de anomalías en error_rate y latency por tool. Abre incidentes. Emite `anomaly.detected`, `incident.opened`. |
| **Coordinator** | `agents.coordinator.v1` | Máquina de estados: OPEN → DIAGNOSING → MITIGATING → MONITORING → RESOLVED/ESCALATED. |
| **Mitigation** | `agents.mitigation.v1` | Recibe `recommended_actions.created`, ejecuta dry-run + apply. Respeta `ApprovalRequired`. |
| **Recovery** | `agents.recovery.v1` | Monitorea post-mitigación. Rollback si TTL expira o condiciones empeoran. Emite RESOLVED si estable. |

### Flujo de incidente
1. **Sentry** detecta anomalía → `anomaly.detected` → `incident.opened`
2. **Coordinator** → transiciona a DIAGNOSING
3. Acciones recomendadas llegan → `recommended_actions.created`
4. **Mitigation** → dry-run → apply → `action.applied`
5. **Coordinator** → MONITORING
6. **Recovery** → si métricas mejoran → RESOLVED; si empeoran → rollback → OPEN

### Métricas Prometheus

| Métrica | Tipo | Labels |
|---------|------|--------|
| `nexus_operators_events_processed_total` | Counter | worker, status |
| `nexus_operators_event_processing_duration_seconds` | Histogram | worker |
| `nexus_operators_consumer_offset` | Gauge | consumer_group |
| `nexus_operators_core_requests_total` | Counter | method, status |

### Persistencia (JSON atómicos en `OPERATOR_DATA_DIR`)

| Archivo | Contenido |
|---------|-----------|
| `offsets.json` | Offsets de consumer groups |
| `sentry_state.json` | Baselines EWMA y fingerprints |
| `proposals.json` | Propuestas pendientes del action engine |
| `recovery_tracks.json` | Mitigaciones en monitoreo |

### Configuración

| Variable | Default | Descripción |
|----------|---------|-------------|
| `NEXUS_CORE_URL` | — | URL de nexus-core (requerido) |
| `OPERATOR_INTERNAL_KEY` | — | API key para nexus-core |
| `NEXUS_DEFAULT_ORG_ID` | — | Org ID por defecto |
| `OPERATOR_BATCH_SIZE` | 100 | Eventos por poll |
| `OPERATOR_POLL_INTERVAL_MS` | 2000 | Intervalo polling (ms) |
| `OPERATOR_IDLE_INTERVAL_MS` | 5000 | Intervalo idle (ms) |
| `OPERATOR_HEALTH_PORT` | 8090 | Puerto health server |
| `OPERATOR_DATA_DIR` | /app/data | Directorio de persistencia |
| `NEXUS_LOG_LEVEL` | info | Nivel de log |

---

## Migraciones (nexus-core)

| Migración | Crea |
|-----------|------|
| 0001 | Extensión `pgcrypto` |
| 0002 | `orgs`, `org_api_keys`, `tools`, `policies`, `audit_events` |
| 0003 | Trigger `set_updated_at` |
| 0004 | `classification` en tools, `org_api_key_scopes`, `tool_secrets`, `tool_egress_rules`, `dlp_summary` en audit |
| 0005 | `prev_event_hash`, `event_hash` en audit (hash-chain) |
| 0006 | `sensitivity` en tools, `idempotency_keys`, columnas extra en audit |
| 0007 | `tenant_settings`, `admin_activity_events` |
| 0008 | `operational_events`, `actions`, `incidents`, `policy_proposals`, `policy_versions`, trigger audit→operational |
| 0009 | `ops_event_store`, `ops_event_contracts`, `ops_consumer_offsets`, `ops_tenant_registry`, `ops_action_catalog`, `ops_action_proposals`, `ops_action_executions`, `ops_diagnosis_reports`, `ops_comms_drafts`, `ops_incident_fingerprints`, `ops_sentry_baselines` |
| 0010 | `pending_approvals` (workflow HITL) |
| 0011 | `alert_rules` |
| 0012 | `agent_sessions` |
| 0013 | `org_usage_counters` (usage metering) |

### Migraciones (nexus-saas)

| Migración | Crea |
|-----------|------|
| 0001 | `orgs`, `org_api_keys`, `org_api_key_scopes`, `tenant_settings`, `admin_activity_events`, `org_usage_counters`, `saas_usage_event_dedup` |

---

## Variables de entorno (nexus-core)

### Requeridas
- `NEXUS_DATABASE_URL` — PostgreSQL connection string
- `NEXUS_MASTER_KEY` — Clave maestra para cifrado de secrets
- `NEXUS_HTTP_PORT` — Puerto HTTP (default container: 8080)

### Auth
- `NEXUS_AUTH_ALLOW_API_KEY` (true) — Habilitar auth por API key
- `NEXUS_AUTH_ENABLE_JWT` (false) — Habilitar auth por JWT
- `NEXUS_JWKS_URL` — URL del JWKS (requerido si JWT)
- `NEXUS_JWT_ISSUER`, `NEXUS_JWT_AUDIENCE` — Validación de issuer/audience
- `NEXUS_JWT_ORG_CLAIM` (org_id), `NEXUS_JWT_ROLE_CLAIM` (role), `NEXUS_JWT_SCOPES_CLAIM` (scopes), `NEXUS_JWT_ACTOR_CLAIM` (sub)

### OIDC
- `NEXUS_OIDC_ENABLED`, `NEXUS_OIDC_ISSUER_URL`, `NEXUS_OIDC_CLIENT_ID`, `NEXUS_OIDC_CLIENT_SECRET`, `NEXUS_OIDC_REDIRECT_URL`, `NEXUS_OIDC_SCOPES`

### HTTP y gateway
- `NEXUS_HTTP_TIMEOUT_MS` (5000) — Timeout HTTP upstream
- `NEXUS_RATE_LIMIT_DEFAULT_PER_MINUTE` (60) — Rate limit default
- `NEXUS_RATE_LIMIT_BACKEND` (inmemory) — Backend: `inmemory` o `redis`
- `NEXUS_REDIS_URL` — Redis URL
- `NEXUS_IDEMPOTENCY_TTL_HOURS` (24) — TTL idempotency keys
- `NEXUS_TIMEOUT_BUDGET_DEFAULT_MS` (10000), `NEXUS_TIMEOUT_BUDGET_MAX_MS` (30000)
- `NEXUS_DISABLE_SSRF_PROTECTION` — Deshabilitar SSRF (solo dev)
- `NEXUS_EGRESS_ALLOWLIST` — Allowlist por defecto
- `NEXUS_CB_FAILURE_THRESHOLD`, `NEXUS_CB_HALF_OPEN_MAX`, `NEXUS_CB_RESET_TIMEOUT_SEC` — Circuit breaker

### Integración
- `NEXUS_SAAS_URL` — URL de nexus-saas
- `NEXUS_SAAS_INTERNAL_KEY` — Key para contratos internos con nexus-saas
- `NEXUS_SAAS_TIMEOUT_MS` (300) — Timeout llamadas a saas
- `NEXUS_OPERATOR_API_KEY` — Key para operator proxy
- `NEXUS_AI_OPERATORS_URL`, `NEXUS_AI_OPERATORS_INTERNAL_KEY` — URL y key de nexus-ai-operators
- `NEXUS_CORS_ALLOWED_ORIGINS` — CORS

### Observabilidad
- `NEXUS_LOG_LEVEL` (info)
- `NEXUS_OTEL_ENABLED`, `NEXUS_OTEL_SERVICE_NAME`, `NEXUS_OTLP_ENDPOINT`, `NEXUS_OTLP_INSECURE` — OpenTelemetry

---

## Infraestructura

### Docker Compose

| Servicio | Puerto | Descripción |
|----------|--------|-------------|
| postgres | `${NEXUS_POSTGRES_PORT}` | PostgreSQL para nexus-core |
| postgres-saas | `${NEXUS_SAAS_POSTGRES_PORT}` | PostgreSQL para nexus-saas |
| redis | `${NEXUS_REDIS_PORT}` | Cache y rate limiting |
| nexus-core | `${NEXUS_HTTP_PORT}` (8080) | Gateway API |
| nexus-saas | `${NEXUS_SAAS_HTTP_PORT}` (8082) | SaaS API |
| mock-tools | `${NEXUS_MOCK_TOOLS_PORT}` (8081) | Mock backends para tests |
| nexus-control-operators | `${OPERATOR_HEALTH_PORT}` (8090) | Workers deterministas |
| nexus-ai-operators | `${NEXUS_OPERATOR_PORT}` (8000) | Operadores IA |
| nexus-tower | `${NEXUS_TOWER_PORT}` (4173/5174) | UI web |
| prometheus | `${NEXUS_PROMETHEUS_PORT}` (9090) | Métricas |
| grafana | `${NEXUS_GRAFANA_PORT}` (3000) | Dashboards |
| ollama | 11434 (perfil `ollama`) | LLM local |

### go.work

```
use (
    nexus-control-operators
    nexus-core
    nexus-saas
    pkgs/go-pkg
)
```

### Monitoreo

- **Prometheus**: scrape nexus-core (:8080) y nexus-control-operators (:8090) cada 15s.
- **Grafana**: dashboard `nexus-gateway-overview` con variable `tool_name`.
  - KPIs: Total Runs, Allow Rate, Deny Rate, Error Rate, Latency p95/p50.
  - Charts: Run Throughput, Latency Percentiles, Decisions Over Time, Top Tools, HTTP Requests/s.
  - Métricas: `nexus_run_total_prom`, `nexus_run_latency_ms_prom_bucket`, `nexus_gateway_requests_total`.

### Mock tools (nexus-core/cmd/mock-tools)

| Ruta | Descripción |
|------|-------------|
| GET `/healthz` | Health check |
| GET `/.well-known/jwks.json` | JWKS (RSA) para tests JWT |
| GET `/_jwt/issue` | Emisión de JWT (query: org_id, sub, role) |
| POST `/echo` | Echo del body + auth info |
| POST `/transfer` | Simula transferencia (amount, sleep_ms, force_5xx) |
| GET `/_stats/transfer` | Contador de ejecuciones |

### Herramientas CLI

- **`cmd/migrate`**: migraciones SQL (up/down).
- **`cmd/cleanup-idempotency`**: limpia `idempotency_keys` expiradas y `pending_approvals` vencidas.

---

## Scripts

| Script | Descripción |
|--------|-------------|
| `scripts/e2e/01_run_echo.sh` | Flujo mínimo: POST /v1/run con tool echo |
| `scripts/e2e/02_run_my_service.sh` | Llama a tool por nombre (argumento) |
| `scripts/e2e/03_full_core_e2e.sh` | Suite completa: tools CRUD, egress, gateway, schema, policies, secrets, simulate, idempotency, audit, authz (62 tests) |
| `scripts/e2e/04_core_gateway_isolated.sh` | E2E aislado con stack propio: auth, policies, DLP, idempotency, MCP, A2A, audit, approvals |
| `scripts/e2e/05_core_jwt_auth.sh` | E2E JWT-only: stack en modo JWT, verifica bearer tokens |
| `scripts/e2e/06_control_operators.sh` | E2E de control operators: health, metrics, events, persistence, anomaly detection (19 tests) |
| `scripts/seed/seed_demo.sh` | Seed de datos demo (org, tools, policies, egress, api keys) |
| `scripts/bootstrap/bootstrap.sh` | Setup completo: .env, stack, migraciones, seed |
| `scripts/demo/demo.sh` | Demo guiada de features |
| `scripts/admin/quickstart_admin.sh` | Setup admin + validación |
| `scripts/db/wait-for-db.sh` | Espera PostgreSQL (max 60s) |

Todos los scripts soportan `--help` con documentación estilo man.

---

## Makefile targets principales

| Target | Descripción |
|--------|-------------|
| `up` / `build` / `down` / `clean` | Stack lifecycle |
| `migrate-up` / `migrate-down` | Migraciones |
| `seed` / `bootstrap` / `demo` | Seed y setup |
| `core-test` | Tests nexus-core |
| `control-operators-test` | Tests nexus-control-operators |
| `ai-operators-test` | Tests nexus-ai-operators (ruff, mypy, pytest) |
| `tower-test` / `tower-qa` | Tests nexus-tower |
| `qa` | Todos los tests |
| `e2e-first` | 01_run_echo.sh |
| `e2e-core` | 03_full_core_e2e.sh |
| `e2e` | 04_core_gateway_isolated.sh (stack aislado) |
| `e2e-jwt` | 05_core_jwt_auth.sh |
| `e2e-operators` | 06_control_operators.sh |
| `sdk-test-python` / `sdk-test` | Tests de SDKs |
| `core-dev` / `saas-dev` / `control-dev` / `ai-operators-dev` / `tower-dev` | Dev con hot-reload |
| `reset-nexus` | Reset completo |
