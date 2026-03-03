## ¿Qué es Nexus?

En una frase: es el “portero” entre agentes de IA y herramientas reales: decide qué se ejecuta, con qué límites, y deja todo registrado.

**Nexus** es un **gateway de control** para que agentes de IA ejecuten herramientas (APIs HTTP) de forma segura y gobernada.

En lugar de que un agente llame directo a una API (pagos, CRM, etc.), el agente llama a Nexus. Nexus decide si permite o no la ejecución, aplica reglas y límites, y registra todo.

---

## Las 3 piezas

1. **nexus-core**  
   - Recibe pedidos de ejecución (`POST /v1/run`, MCP, A2A).  
   - Valida auth, políticas, DLP, rate limits, egress/SSRF.  
   - Ejecuta la tool HTTP si todo pasa.  
   - Audita cada intento (allow/deny) con hash-chain.

2. **nexus-operator**  
   - Lee eventos de core (`GET /v1/events`).  
   - Si detecta muchas denegaciones (riesgo alto), aplica throttles, crea incidentes o propone políticas nuevas.

3. **nexus-tower**  
   - UI de operación: eventos, acciones, incidentes, propuestas de políticas.  
   - Incluye simulador “Door Jam” para probar agentes en un grid.

---

## Reglas, límites y controles en una ejecución

### 1. Autenticación (antes de llegar al gateway)
- **API key o JWT obligatoria**: sin key → 401; key inválida → 401.
- **Scopes por endpoint**: sin scope → 403; excepción: admin/secops pasan siempre.

### 2. Resolución de tool
- Tool inexistente → 404 tool not found.

### 3. Idempotencia (solo tools WRITE)
- Policy exige `require_idempotency` y no mandás `Idempotency-Key` → 400.
- Misma key con otro payload → 409 conflict.
- Misma key en progreso → 409 in progress.
- Stale in-progress → se limpia y se trata como nueva.

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
- Límites por policy: rate_limit.per_minute, max_bytes_input, max_bytes_context, require_idempotency.
- Default: read → allow; write → deny.

### 8. Límites de tamaño (cuando policy matchea allow)
- max_bytes_input → 403 input too large.
- max_bytes_context → 403 context too large.

### 9. Action overrides (runtime)
- Acción activa que deniega → 403 blocked by active action override.
- Puede bajar rate limit tenant/tool.

### 10. Rate limit por tenant
- Tenant supera run_rpm → 403 tenant run rate limit exceeded.

### 11. Rate limit por tool
- Tool supera rate limit (policy o override) → 403 rate limit exceeded.

### 12. URL y egress (SSRF + allowlist)
- URL no parseable → 400 invalid tool url.
- SSRF activo: bloquea IPs privadas, loopback, link-local, metadata (169.254.169.254), IPv6 ULA.
- Host no en egress allowlist de la tool → 403 egress host denied.

### 13. Timeout budget
- Presupuesto agotado antes de ejecutar → 408 timeout budget exhausted.

### 14. Ejecución HTTP
- Fallo → 502 con código de error (timeout, 5xx, etc.).
- Retries: solo tools read; write no reintenta.

### 15. Validación de schema de salida
- Tool define output_schema y la respuesta no cumple → 502 tool output does not match schema.

### 16. Auditoría
- Siempre se registra cada intento (allow/deny/error) con hash-chain, redacción de datos sensibles y DLP summary.

---

## Resumen

| Categoría | Qué hace |
|----------|----------|
| Auth | API key, JWT, scopes por endpoint |
| Idempotencia | Requerida en writes, replay, conflict, in-progress |
| Tool | Enabled, kind, schema input/output |
| DLP | Detección PII, expuesto en context.dlp |
| Políticas | Condiciones + allow/deny + límites por policy |
| Límites | max_bytes_input, max_bytes_context, rate_limit |
| Overrides | Deny temporal por acciones activas |
| Rate limits | Tenant y por tool |
| Egress/SSRF | Allowlist por tool, bloqueo IPs privadas |
| Timeout | Budget consumido por etapas |
| Auditoría | Siempre, con hash-chain y redacción |

---

## Estructura de directorios de nexus-core


### 1. `cmd/` — Entry points

| Directorio | Responsabilidad |
|------------|-----------------|
| `cmd/api` | API HTTP principal (Gin), health, docs, rutas `/v1/*` |
| `cmd/cleanup-idempotency` | Job para limpiar registros de idempotencia expirados |
| `cmd/config` | Carga de configuración desde env (DB, HTTP, auth, OIDC, etc.) |
| `cmd/migrate` | Ejecución de migraciones SQL |
| `cmd/mock-tools` | Servidor mock de tools para pruebas |
| `cmd/ops-workers` | Proceso que ejecuta los agentes (sentry, diagnostician, comms, etc.) |

### 2. `internal/` — Módulos por dominio

#### Gateway y control de ejecución

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/gateway` | Orquestación de `POST /v1/run`: auth, tool, DLP, políticas, egress, ejecución HTTP, idempotencia, auditoría |
| `internal/gateway/executor/http` | Ejecutor HTTP hacia upstream (timeouts, retries, circuit breaker) |
| `internal/gateway/executor/circuitbreaker` | Circuit breaker por host upstream (closed/open/half-open) |
| `internal/gateway/executor/ratelimit` | Rate limiting (in-memory y Redis) |
| `internal/gateway/executor/telemetry` | Métricas de ejecución |

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

#### Ops y agentes

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/ops/actionengine` | Motor de acciones: proponer, dry-run, aplicar, rollback |
| `internal/ops/eventstore` | Store de eventos (append, stream, schema validation) |
| `internal/ops/comms` | Borradores de comunicación (drafts, aprobación) |
| `internal/ops/diagnosis` | Reportes de diagnóstico por incidente |
| `internal/ops/tenant` | Perfiles de tenant (límites, cost model) |
| `internal/ops/llm` | Cliente LLM (mock, Ollama, cloud) para agentes |
| `internal/ops/schemas` | JSON Schemas para acciones, eventos y LLM |

#### Agentes (workers)

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/agents/sentry` | Detección de anomalías (EWMA, baselines) |
| `internal/agents/coordinator` | Orquestación de incidentes y cooldown |
| `internal/agents/diagnostician` | Diagnóstico con LLM (root cause, acciones sugeridas) |
| `internal/agents/mitigation` | Aplicación de acciones recomendadas |
| `internal/agents/recovery` | Verificación y rollback automático |
| `internal/agents/comms` | Borradores de comunicación con LLM |
| `internal/agents/executive_qa` | Q&A para operadores con LLM |
| `internal/agents/runtime` | Tests E2E del flujo de agentes |

#### Aprobaciones, alertas y sesiones

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/approval` | Workflow de aprobación humana (HITL): crear, listar, aprobar, rechazar, expirar |
| `internal/alerts` | Reglas de alerta con webhook: deny_rate, error_rate, rate_limited_count; métricas desde audit |
| `internal/session` | Tracking de sesiones de agente: calls, writes, denials por session_id |

#### Otros módulos

| Directorio | Responsabilidad |
|------------|-----------------|
| `internal/admin` | Admin API (activity events, hard limits) |
| `internal/assistant` | Asistente para operadores |
| `internal/org` | Organizaciones (multi-tenant) + onboarding (`POST /v1/orgs`) |
| `internal/mcp` | Endpoint MCP JSON-RPC (`tools/list`, `tools/call`) |
| `internal/a2a` | Protocolo Agent-to-Agent |
| `internal/world` | Integración con sim-engine (observe, move) |
| `internal/toollab` | Módulo en preparación (toollab) |
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
| `migrations` | Migraciones SQL (up/down) |
| `docs` | Documentación (ARCHITECTURE, runbooks, OpenAPI, admin UI) |
| `monitoring/grafana` | Dashboards y provisioning |
| `monitoring/prometheus` | Configuración de Prometheus |
| `scripts` | Scripts de DB, demo, e2e |
| `testdata` | Datos de prueba (eventos, LLM) |
| `third_party` | Código de terceros (ej. toollab-adapter-go) |