## ¿Qué es Nexus?

En una frase: es el "portero" entre agentes de IA y herramientas reales: decide qué se ejecuta, con qué límites, y deja todo registrado.

**Nexus** es un **gateway de control** para que agentes de IA ejecuten herramientas (APIs HTTP) de forma segura y gobernada.

En lugar de que un agente llame directo a una API (pagos, CRM, etc.), el agente llama a Nexus. Nexus decide si permite o no la ejecución, aplica reglas y límites, y registra todo.

---

## Las piezas

1. **nexus-core**  
   - Recibe pedidos de ejecución (`POST /v1/run`, MCP, A2A).
   - Aplica controles síncronos del gateway: auth, políticas, DLP, rate limits, circuit breaker, egress/SSRF, idempotencia, timeout budget.
   - Puede requerir aprobación humana antes de ejecutar (HITL).
   - Ejecuta la tool HTTP solo si todas las validaciones pasan.
   - Audita cada intento (allow/deny/error) con hash-chain.

2. **nexus-control-operators** (determinista, Go)  
   - Plano de control interno: monitorea eventos y ejecuta respuestas deterministas.
   - Workers activos: `sentry`, `coordinator`, `mitigation`, `recovery`.
   - No forma parte del path síncrono de `/v1/run` (opera en background).
   - Se despliega como servicio dedicado (`nexus-control-operators`).

3. **nexus-ai-operators** (IA, Python)  
   - Servicio externo de operadores con IA/ML.
   - Consume APIs de Nexus (sin acceso directo a DB).
   - Objetivo: diagnóstico inteligente, policy suggestions, automaciones asistidas por IA.
   - Debe invocar herramientas de control y no aplicar cambios críticos fuera de controles deterministas.

4. **nexus-tower**  
   - UI de supervisión: overview, run explorer, timeline, policies, approvals, alerts, sessions, ask-agent, exports.

5. **sim-engine**  
   - Simulador de eventos para QA y demos. Migraciones y scripts de replay.

6. **SDKs** (Python + TypeScript)
   - Clientes tipados para toda la API.
   - Integraciones: LangChain (`NexusTool`, `NexusToolkit`), OpenAI Agents SDK (`nexus_function_tools`).

---

## Flujo de request (POST /v1/run)

El pipeline completo solo aplica a `POST /v1/run` y `POST /v1/run/simulate`. Otras rutas pasan por auth y su handler directo.

1. **Auth** — Identifica org, actor y permisos (API key o JWT).
2. **Tool lookup** — Busca la tool por nombre en la org.
3. **Idempotencia** — Para writes, comprueba si ya se procesó esa key.
4. **Validación** — Context, DLP (PII) y schema del input.
5. **Políticas** — Evalúa condiciones (first-match) y decide allow/deny.
6. **Approval (HITL)** — Si la política exige aprobación humana, bloquea hasta que se apruebe.
7. **Controles** — Rate limits, egress (hosts permitidos), secrets, action overrides (control operators).
8. **Ejecución** — Si allow y no requiere approval: llama por HTTP al upstream (respetando timeout budget).
9. **Respuesta** — Valida output schema, escribe auditoría y devuelve el resultado.

### Timeout budget

Hay un presupuesto de tiempo que se consume en el pipeline. Si se agota antes de ejecutar, la request se bloquea con timeout.

### Rutas públicas (sin auth)

- **OIDC** (`/v1/oidc/*`) — Punto de entrada para login OAuth2.
- **Onboarding** (`POST /v1/orgs`) — Crear org y API key inicial.

El resto de rutas bajo `/v1` requieren auth.

### Demo: echo y mock-tools

En local, el seed crea el tool **echo** que apunta a `http://mock-tools:8081/echo`. **mock-tools** es un servicio HTTP de prueba (en docker-compose) que emula un upstream real: recibe el JSON y lo devuelve. Sirve para probar el flujo sin servicios externos.

---

## Reglas, límites y controles en una ejecución

### 1. Autenticación (antes de llegar al gateway)
- **API key o JWT obligatoria**: sin key → 401; key inválida → 401.
- **OIDC/SSO**: flujo OAuth2 + PKCE opcional.
- **Scopes por endpoint**: sin scope → 403; excepción: admin/secops pasan siempre.

### 2. Resolución de tool
- Tool inexistente → 404 tool not found.

### 3. Idempotencia (solo tools WRITE)
- Policy exige `require_idempotency` y no mandás `Idempotency-Key` → 400.
- Misma key con otro payload → 409 conflict.
- Misma key en progreso → 409 in progress.
- Stale in-progress → se limpia y se trata como nueva.
- Misma key con FAILED → replay terminal del error (no reintenta upstream).

### 4. Validación de tool
- Tool deshabilitada → 403 tool disabled.
- Kind distinto de `http` → 403 unsupported tool kind.

### 5. DLP (detección de datos sensibles)
- Se analiza `input` y `context` (email, phone, credit_card, jwt, api_key, national_id).
- Resultado en `context.dlp` para que las policies lo usen.

### 6. Validación de schema de entrada
- Schema inválido → 403 tool input schema invalid.
- Input no cumple schema → 400 input does not match schema.

### 7. Políticas (Policy DSL)
- Orden: first-match por prioridad.
- Condiciones + efecto (allow/deny).
- Paths: `input.*`, `context.*`, `tool.*`.
- Operadores: exists, not_exists, eq, neq, lt, lte, gt, gte, in, contains, regex.
- Composición: all, any, not.
- Límites por policy: rate_limit.per_minute, max_bytes_input, max_bytes_context, require_idempotency, require_approval.
- Default: read → allow; write → deny.

### 8. Aprobación humana (HITL)
- Si la policy matcheada tiene `require_approval: true`, la ejecución se bloquea con `APPROVAL_REQUIRED`.
- Se crea un registro en `pending_approvals` con TTL configurable.
- Un humano aprueba o rechaza vía `POST /v1/approvals/:id/approve` o `/reject`.
- Las approvals vencidas se expiran automáticamente por el cleanup job.

### 9. Límites de tamaño (cuando policy matchea allow)
- max_bytes_input → 403 input too large.
- max_bytes_context → 403 context too large.

### 10. Action overrides (runtime)
- Acción activa que deniega → 403 blocked by active action override.
- Puede bajar rate limit tenant/tool.

### 11. Rate limit por tenant
- Tenant supera run_rpm → 403 tenant run rate limit exceeded.

### 12. Rate limit por tool
- Tool supera rate limit (policy o override) → 403 rate limit exceeded.

### 13. URL y egress (SSRF + allowlist)
- URL no parseable → 400 invalid tool url.
- SSRF activo: bloquea IPs privadas, loopback, link-local, metadata (169.254.169.254), IPv6 ULA.
- Host no en egress allowlist de la tool → 403 egress host denied.

### 14. Timeout budget
- Presupuesto agotado antes de ejecutar → 408 timeout budget exhausted.
- Se trackea por etapa (policy, egress, execution) en `stage_durations_ms`.

### 15. Circuit breaker
- Per-host upstream: si el host acumula fallos consecutivos, el breaker se abre.
- Open → requests rechazados con `CIRCUIT_BREAKER_OPEN` sin llamar a upstream.
- Half-open → permite N requests de prueba; si pasan, cierra el breaker.
- Configurable: `NEXUS_CB_FAILURE_THRESHOLD`, `NEXUS_CB_HALF_OPEN_MAX`, `NEXUS_CB_RESET_TIMEOUT_SEC`.

### 16. Ejecución HTTP
- Fallo → 502 con código de error (timeout, 5xx, etc.).
- Retries: solo tools read; write no reintenta.
- Secret injection: secretos cifrados se inyectan en headers al ejecutar.

### 17. Validación de schema de salida
- Tool define output_schema y la respuesta no cumple → 502 tool output does not match schema.

### 18. Auditoría
- Siempre se registra cada intento (allow/deny/error) con hash-chain, redacción de datos sensibles y DLP summary.
- Export en CSV y JSONL.

### 19. Tracking de sesión
- Si el request incluye session context, se incrementan los contadores del agente (calls, writes, denials).
- Consultable vía `GET /v1/sessions/:session_id`.

### 20. Alertas
- Reglas configurables por org: metric (deny_rate, error_rate, rate_limited_count), threshold, window, cooldown.
- Cuando el valor excede el threshold, se dispara un webhook con el payload de alerta.
- Métricas calculadas desde la tabla `audit_events`.

---

## Resumen

| Categoría | Qué hace |
|----------|----------|
| Auth | API key, JWT, OIDC/SSO, scopes por endpoint |
| Idempotencia | Requerida en writes, replay, conflict, in-progress, terminal replay |
| Tool | Enabled, kind, schema input/output |
| DLP | Detección PII, expuesto en context.dlp |
| Políticas | Condiciones + allow/deny + límites por policy |
| Aprobación HITL | Bloqueo hasta aprobación humana, con TTL y expiración |
| Límites | max_bytes_input, max_bytes_context, rate_limit |
| Overrides | Deny temporal por acciones activas |
| Rate limits | Tenant y por tool (in-memory o Redis) |
| Circuit breaker | Per-host upstream, configurable |
| Egress/SSRF | Allowlist por tool, bloqueo IPs privadas |
| Timeout | Budget consumido por etapas |
| Secret injection | Vault cifrado, inyección en headers |
| Auditoría | Siempre, con hash-chain, redacción, export CSV/JSONL |
| Sesiones | Tracking de calls/writes/denials por agente |
| Alertas | Webhook cuando métricas superan umbral |
| SDKs | Python (sync + async) y TypeScript, con integraciones LangChain y OpenAI |

---

## Endpoints principales

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `/v1/run` | POST | Ejecutar tool a través del gateway |
| `/v1/run/simulate` | POST | Simular ejecución (sin upstream) |
| `/v1/tools` | GET/POST | Listar/crear tools |
| `/v1/tools/:name` | GET/PUT | Detalle/actualizar tool |
| `/v1/tools/:name/policies` | GET/POST | Políticas por tool |
| `/v1/tools/:name/egress-rules` | GET/POST | Reglas de egress por tool |
| `/v1/tools/:name/secrets` | POST | Upsert secreto para tool |
| `/v1/audit` | GET | Consultar audit trail |
| `/v1/audit/export` | GET | Exportar audit (CSV/JSONL) |
| `/v1/approvals` | GET | Listar approvals pendientes |
| `/v1/approvals/:id` | GET | Detalle de approval |
| `/v1/approvals/:id/approve` | POST | Aprobar |
| `/v1/approvals/:id/reject` | POST | Rechazar |
| `/v1/alert-rules` | GET/POST | Listar/crear reglas de alerta |
| `/v1/alert-rules/:id` | DELETE | Eliminar regla de alerta |
| `/v1/sessions/:session_id` | GET | Consultar sesión de agente |
| `/v1/orgs` | POST | Crear org + API key (onboarding) |
| `/v1/events` | GET | Stream de eventos operacionales |
| `/v1/actions` | GET/POST | Acciones del operador |
| `/v1/incidents` | GET/POST | Incidentes |
| `/v1/policy-proposals` | GET/POST | Propuestas de políticas |
| `/v1/assistant/query` | POST | Consulta al asistente |
| `/mcp` | POST | Model Context Protocol (JSON-RPC) |
| `/a2a/call` | POST | Agent-to-Agent |

---

## Estructura de directorios de nexus-core

**nexus-core** es el gateway (data plane): solo recibe requests, aplica controles síncronos y ejecuta. No incluye workers de control; esos viven en `nexus-control-operators`.

### 1. `cmd/` — Entry points

| Directorio | Responsabilidad |
|------------|-----------------|
| `cmd/api` | API HTTP principal (Gin), health, docs, rutas `/v1/*` |
| `cmd/cleanup-idempotency` | Job para limpiar idempotencia expirada y approvals vencidos |
| `cmd/config` | Carga de configuración desde env (DB, HTTP, auth, OIDC, circuit breaker, etc.) |
| `cmd/migrate` | Ejecución de migraciones SQL |
| `cmd/mock-tools` | Servidor mock de tools para pruebas |

### 2. `internal/` — Módulos por dominio

#### Gateway y control de ejecución

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/gateway` | Orquestación de `POST /v1/run`: auth, tool, DLP, políticas, approval, egress, ejecución HTTP, idempotencia, auditoría |
| `internal/gateway/executor/http` | Ejecutor HTTP hacia upstream (timeouts, retries, circuit breaker) |
| `internal/gateway/executor/circuitbreaker` | Circuit breaker por host upstream (closed/open/half-open) |
| `internal/gateway/executor/ratelimit` | Rate limiting (in-memory y Redis) |
| `internal/gateway/executor/telemetry` | Métricas de ejecución (OTel + Prometheus) |

#### Autenticación e identidad

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/identity` | Resolución de principal (org, actor, role, scopes) |
| `internal/identity/executor/jwks` | Verificación JWT vía JWKS |
| `internal/identity/executor/oidc` | Flujo OIDC (discovery, token exchange, PKCE) |

#### Herramientas y políticas

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/tool` | CRUD de tools (URL, method, schemas, egress) |
| `internal/policy` | Políticas (conditions + limits) y evaluador DSL |
| `internal/policyproposal` | Propuestas de políticas generadas por agentes |
| `internal/egress` | Allowlist de hosts por tool (SSRF) |

#### Seguridad y datos sensibles

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/dlp` | Detección de PII (email, phone, credit_card, etc.) en input/context |
| `internal/secrets` | Vault de secretos cifrados e inyección en headers |

#### Auditoría y eventos

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/audit` | Eventos de auditoría (allow/deny/error), hash-chain, export CSV/JSONL |
| `internal/events` | Eventos de dominio (append, stream) |

#### Acciones e incidentes

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/actions` | Acciones aplicadas (set_rate_limit, etc.) y su ciclo de vida |
| `internal/incidents` | Incidentes (abrir, cerrar, evidencia) |

#### Aprobaciones, alertas y sesiones

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/approval` | Workflow de aprobación humana (HITL): crear, listar, aprobar, rechazar, expirar |
| `internal/alerts` | Reglas de alerta con webhook: deny_rate, error_rate, rate_limited_count; métricas desde audit |
| `internal/session` | Tracking de sesiones de agente: calls, writes, denials por session_id |

#### Ops y agentes (fuera de core)

**nexus-core es solo gateway.** No contiene ops ni agentes. Los siguientes componentes viven en otros servicios:

- **nexus-control-operators**: `internal/ops/` (actionengine, eventstore, tenant), workers deterministas (sentry, coordinator, mitigation, recovery).
- **nexus-ai-operators**: diagnóstico IA, comms IA, assistant query (Python).

#### Otros módulos

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/admin` | Admin API (activity events, hard limits) |
| `internal/assistant` | Proxy de `/v1/assistant/query` hacia nexus-ai-operators |
| `internal/org` | Organizaciones (multi-tenant) + onboarding (`POST /v1/orgs`) |
| `internal/mcp` | Endpoint MCP JSON-RPC (`tools/list`, `tools/call`) |
| `internal/a2a` | Protocolo Agent-to-Agent |
| `internal/shared/authz` | Permisos y scopes por endpoint |
| `internal/shared/handlers` | Middleware de auth (API key, JWT) |

### 3. `pkg/` — Utilidades reutilizables

| Directorio | Responsabilidad |
|------------|-----------------|
| `pkg/config/godotenv` | Carga de `.env` |
| `pkg/databases/sql/gorm` | Conexión GORM a PostgreSQL |
| `pkg/http/errors` | Tipos de error HTTP y escritura de respuestas |
| `pkg/http/middlewares/gin` | RequestID, Recovery, CORS, BodyLimit, Logger |
| `pkg/http/servers/gin` | Creación del engine Gin |
| `pkg/telemetry` | OpenTelemetry (tracing, métricas) |
| `pkg/types` | Context keys, error codes, tipos HTTP |
| `pkg/utils` | SHA256Hex, canonical JSON, AES-GCM, redact, SSRF |
| `pkg/validations/jsonschema` | Compilación y validación de JSON Schema |

### 4. Otros directorios

| Directorio | Responsabilidad |
|------------|-----------------|
| `wire` | Inyección de dependencias (Wire) y bootstrap de rutas |
| `migrations` | Migraciones SQL (up/down, 12 migraciones) |
| `docs` | Documentación (OpenAPI, admin UI) |
| `monitoring/grafana` | Dashboards y provisioning |
| `monitoring/prometheus` | Configuración de Prometheus |
| `scripts` | Scripts de DB, demo, e2e, seed |

### 5. SDKs (`/sdks`)

| Directorio | Responsabilidad |
|------------|-----------------|
| `sdks/python-sdk` | SDK Python: sync (`NexusClient`) + async (`AsyncNexusClient`), tipos, tests |
| `sdks/python-sdk/nexus_sdk/integrations/langchain.py` | `NexusTool` + `NexusToolkit` para LangChain |
| `sdks/python-sdk/nexus_sdk/integrations/openai_agents.py` | `nexus_function_tools` para OpenAI Agents SDK |
| `sdks/typescript-sdk` | SDK TypeScript: `NexusClient`, tipos exportados |

---

## Estructura de nexus-control-operators

Servicio Go dedicado al control plane determinista. No forma parte del path síncrono de `/v1/run`.

| Directorio | Responsabilidad |
|------------|-----------------|
| `cmd/ops-workers` | Entry point del servicio |
| `internal/agents/sentry` | Detección de anomalías (EWMA, baselines); `worker/helpers.go` para utilidades |
| `internal/agents/coordinator` | Orquestación de incidentes y cooldown; `worker/helpers.go` |
| `internal/agents/mitigation` | Aplicación de acciones recomendadas; `worker/helpers.go` |
| `internal/agents/recovery` | Verificación y rollback automático; `worker/helpers.go` |
| `internal/ops/actionengine` | Motor de acciones: proponer, dry-run, aplicar, rollback |
| `internal/ops/eventstore` | Emitter, consumer, store de eventos (schema validation) |
| `internal/ops/schemas` | JSON Schemas para eventos y acciones |
| `internal/ops/tenant` | Perfiles de tenant (límites, cost model) |
| `internal/incidents` | Modelo de incidentes |
| `internal/events` | Eventos de dominio |

---

## Estructura de nexus-ai-operators

Servicio Python con operadores IA/ML. Consume APIs de Nexus (sin acceso directo a DB).

| Componente | Responsabilidad |
|------------|-----------------|
| `/v1/assistant/query` | Respuestas de asistente (summary, tables, actions) para Tower |
| `/v1/internal/tick` | Tick manual para procesamiento por lotes |
| Diagnóstico, policy proposals | Funcionalidad IA que propone acciones; las aplica nexus-control-operators vía API |












































El payload completo que un consumer envía a `POST /v1/run` ahora es:

```json
{
  "request_id": "opcional-uuid-generado-por-el-consumer",
  "tool_name": "my-service",
  "tool_id": "5dca11d1-5ee8-4b73-90ab-5e67fe8d31b6",
  "input": {
    "msg": "hello from consumer"
  },
  "context": {
    "source": "my-agent",
    "session_id": "abc-123"
  }
}
```

Donde:

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `request_id` | string | No | UUID para tracing. Si no se envía, Nexus genera uno |
| `tool_name` | string | Uno de los dos | Nombre de la tool (inmutable, legible) |
| `tool_id` | string | Uno de los dos | UUID de la tool. Si ambos vienen, `tool_id` gana |
| `input` | object | Sí | Payload que recibe la tool. Se valida contra su JSON Schema |
| `context` | object | No | Metadata del consumer (actor, session, etc.). Usado por policies y DLP |

Además, hay headers opcionales:

| Header | Descripción |
|--------|-------------|
| `X-NEXUS-CORE-KEY` | API key (requerido) |
| `X-NEXUS-SCOPES` | Scopes del consumer (mínimo `gateway:run`) |
| `X-NEXUS-ACTOR` | Identidad del llamante (para audit) |
| `X-NEXUS-ROLE` | Rol del llamante (para policies) |
| `X-Timeout-Ms` | Budget de timeout en ms |
| `Idempotency-Key` | Clave de idempotencia (para write tools) |



Sí, es así. El pipeline completo de `POST /v1/run` en orden es:

| Paso | Nombre | Qué hace |
|------|--------|----------|
| 1 | **Auth** | Valida API key, extrae org_id, scopes, actor, role |
| 2 | **Authz** | Verifica que tenga scope `gateway:run` |
| 3 | **Tool resolution** | Busca la tool por `tool_id` o `tool_name` |
| 4 | **Idempotency** | Si es write tool y viene `Idempotency-Key`, verifica replay/conflicto |
| 5 | **Tool validation** | Verifica que la tool esté `enabled` y sea `kind: http` |
| 6 | **Context enrichment** | Inyecta actor, role, scopes, auth_method al context |
| 7 | **DLP** | Escanea input y context buscando datos sensibles (emails, tokens, etc.) |
| 8 | **Schema validation** | Valida el `input` contra el JSON Schema de la tool |



| 9 | **Policy evaluation** | Evalúa las policies asociadas a la tool → `allow` o `deny` |




| 10 | **Action overrides** | Verifica si hay un override runtime activo (kill switch por tool) |
| 11 | **Tenant rate limit** | Rate limit global del tenant (org) |
| 12 | **Tool rate limit** | Rate limit específico de la tool (de la policy) |
| 13 | **URL + Egress** | SSRF protection + verifica que el host esté en la allowlist |
| 14 | **Secrets injection** | Carga secrets asociados a la tool e inyecta como headers |
| 15 | **Timeout budget** | Verifica que quede tiempo antes de ejecutar |
| 16 | **HTTP execution** | Llama al upstream (el backend real detrás de la tool) |
| 17 | **Output schema** | Valida la respuesta del upstream contra el output schema (best-effort) |
| 18 | **Audit** | Graba todo en el audit log (hash-chained) |
| 19 | **Response** | Devuelve resultado al consumer |

Ese es el valor de Nexus: el consumer hace un solo `POST /v1/run` y Nexus ejecuta 19 pasos de governance, seguridad y observabilidad antes de tocar el upstream.