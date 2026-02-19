## Qué hace la app (`nexus-gateway`)

`nexus-gateway` es un **Agent Tool Gateway multi-tenant**: una capa de control para que **agentes de IA** (o cualquier caller) puedan **invocar herramientas/APIs HTTP** de forma **segura, gobernada, auditable y observable**.

Es como un **“portero y registro”** para que las IAs de una empresa puedan usar herramientas reales (APIs) sin hacer lío.

En vez de dejar que un agente de IA llame directamente a sistemas sensibles (por ejemplo pagos, CRM, tickets, bases internas), lo hace **a través de Nexus**. Nexus decide **si se permite o no**, **con qué límites**, y deja todo **registrado** para que después puedas auditar qué pasó.

En vez de que el agente llame directo a Jira/Slack/GitHub/tu API interna (con tokens y permisos peligrosos), el agente llama a **Nexus**, y Nexus decide si puede, bajo qué condiciones, con qué límites y cómo queda registrado. Además expone **MCP nativo** para que agentes consuman tools vía JSON-RPC.

### Qué te da, explicado simple

* **Control**: definís reglas tipo “la IA puede leer esto, pero no puede escribir aquello” o “si detecta datos sensibles, bloquear”.
* **Seguridad**: evita que la IA “se vaya” a destinos peligrosos o a redes internas por error o abuso.
* **Protección de secretos**: la IA no ve tokens ni contraseñas; Nexus los guarda y los usa por detrás cuando hace falta.
* **Evita duplicados**: si la IA reintenta una acción (por ejemplo una transferencia), Nexus puede evitar que se ejecute dos veces.
* **Límites y frenos**: podés limitar cuántas veces por minuto puede ejecutar algo, cuánto tamaño de datos puede mandar, y cuánto tiempo tiene para responder.
* **Registro y evidencia**: guarda un historial (sin datos sensibles en claro) de cada intento: qué quiso hacer, si se permitió o no, cuánto tardó, qué devolvió.
* **Export para compliance**: podés exportar ese historial para herramientas de seguridad/monitoreo (SIEM).

### Cómo se usa en la práctica

1. Conectás tus “tools” (APIs internas o externas) en Nexus.
2. Definís reglas de uso.
3. El agente de IA llama a Nexus diciendo “quiero ejecutar la tool X con estos datos”.
4. Nexus valida, decide, ejecuta si corresponde, y devuelve el resultado.

**Resultado:** podés poner agentes en producción con mucha más tranquilidad, porque no están sueltos: están “encarrilados” por un sistema que controla, limita y deja evidencia.

---

## Cómo correr y probar en local (Docker Compose)

Requisitos: `docker compose`, `make`, `curl`, `jq`.

1) Levantar stack + migraciones + seed:

```bash
cp .env.example .env
make up
make migrate-up
make seed
```

2) Probar `POST /v1/run` (REST):

```bash
export NEXUS_API_KEY="<seed-output-value>"
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"echo","input":{"hello":"world"},"context":{"user_id":"u_1"}}' \
  http://localhost:8080/v1/run | jq
```

3) E2E automatizado:

```bash
make e2e
make jwt-e2e
make qa
```

### Nota importante: SSRF vs Docker (mock-tools)

En Docker Compose, `mock-tools` resuelve a una IP privada tipo `172.18.x.x` (red bridge).
Con la protección SSRF activada (default), **las IPs privadas se bloquean por diseño**, así que
las tools que apunten a `http://mock-tools:8081/...` van a fallar con `EGRESS_DENIED` + mensaje SSRF.

Opciones:

* **Demo local (manual)**: setear `NEXUS_DISABLE_SSRF_PROTECTION=true` en `.env` (solo dev/test).
* **E2E (scripts)**: `scripts/e2e.sh` y `scripts/e2e_jwt.sh` ya fuerzan `NEXUS_DISABLE_SSRF_PROTECTION=true` durante el run para que funcione en Docker.

---

## Flujo real de ejecución (`POST /v1/run`) — en el orden exacto del código

1. **Autenticación (middleware)**: API key hash o JWT/JWKS → resuelve `org_id` + principal.
2. **Timeout budget**: se crea al inicio y se consume por etapas (`budget.Consume(...)`).
3. **Resolución del tool**: lookup del tool por `tool_name` dentro del `org_id`.
4. **Idempotency check (temprano)**: replay/conflict/in-progress/staleness.

   * Si es **REPLAY**, devuelve la respuesta cacheada y **salta todo el pipeline** (DLP/schema/policies/egress/etc.).
5. **Contexto enriquecido**: propaga `actor`, `role`, `scopes`, `auth_method` al contexto para policies/audit.
6. **DLP summarize**: detecta `email`, `phone`, `credit_card` (Luhn), `jwt`, `api_key`, `national_id` (`\b[0-9]{8,12}\b`) y expone `context.dlp.*`.
7. **Validación de input (JSON Schema)**: valida `input` vs `tool.input_schema`.
8. **Policy evaluation (Policy DSL)**: evalúa reglas con `input.*`, `context.*` (incluyendo `context.dlp.*`) y `tool.*`.

   * Si **ninguna policy matchea**, el default es **hardcodeado**:

     * `tool.action_type == write` → **deny** (“default deny for write tool”)
     * `tool.action_type == read` → **allow** (“default allow for read tool”)
9. **Límites por policy**: `rate_limit.per_minute`, `max_bytes_input`, `max_bytes_context`, `require_idempotency`.
10. **Rate limiting**: Redis o in-memory (con cleanup).
11. **SSRF check + Egress check**: bloqueo de rangos privados/metadata/ULA + allowlist por tool (**default-deny** si no hay reglas).
12. **Inyección de secrets**: vault cifrado (AES-GCM) por org/tool, inyecta `header` o `bearer`.
13. **Ejecución HTTP**: GET→query (solo flat primitives), POST/PUT/PATCH→JSON body, timeout derivado del budget; retries **read configurable**, **write = 0**.
14. **Validación de output schema (si está definido)**: es **estricta**; si no valida → `OUTPUT_SCHEMA_INVALID` (502) y marca idempotency como failed.
15. **Auditoría**: append-only redactada + **hash-chain** tamper-evident + DLP summary.
16. **Respuesta normalizada**: `request_id`, `decision`, `status`, `latency_ms`, output/error.

---

## Features (lista completa)

### 1) Multi-tenancy y aislamiento

* Todo se filtra por `org_id` (aislamiento multi-tenant).
* API keys guardadas solo como **hash SHA-256** (no plaintext).

### 2) Auth e identidad

* API key: `X-NEXUS-GATEWAY-KEY`.
* JWT + JWKS (modo JWT): Bearer token + validación de claims.
* Contexto: `actor`, `role`, `scopes`, y `auth_method` (`api_key` o `jwt`) para decisiones seguras.

### 3) Tool Registry (catálogo de herramientas)

* CRUD: `POST/GET /v1/tools`, `GET/PUT /v1/tools/:name`.
* Campos del tool:

  * `action_type`: `read|write`
  * `classification`: **string libre** (convención típica `internal|external`, pero no es enum)
  * `sensitivity`: **string libre** (convención típica `low|medium|high`, pero no es enum)
  * `method`, `url`
  * `input_schema` (JSON Schema)
  * `output_schema` (opcional; si existe, se valida estrictamente)

### 4) Policies (Policy DSL)

* Composición: `all`, `any`, `not`.
* Operadores (12): `exists`, `not_exists`, `eq`, `neq`, `lt`, `lte`, `gt`, `gte`, `in`, `contains`, `regex`.
* Paths: `input.*`, `context.*`, `tool.*`.
* **Prioridad** + first-match-wins.
* Defaults hardcodeados si no hay match: **read allow / write deny**.

### 5) DLP para guardrails

* Detecta: `email`, `phone`, `credit_card`, `jwt`, `api_key`, `national_id`.
* Expone `context.dlp.*` para policies; guarda solo resumen.

### 6) Egress + SSRF hardening

* Egress allowlist por tool (`/v1/tools/:name/egress-rules`).
* **Default-deny** si no hay reglas.
* SSRF 3 capas: pre-check + Safe Dialer (anti DNS rebinding) + no redirects.
* Bloquea private/loopback/link-local/metadata + IPv6 ULA (`fc00::/7`).
* Flag (solo dev/test): `NEXUS_DISABLE_SSRF_PROTECTION=true` desactiva checks SSRF para poder correr demos/E2E en Docker con `mock-tools` (IP privada). No usar en prod.

### 7) Secrets Vault

* Secrets AES-GCM por org/tool.
* Inyección `header`/`bearer`.
* Endpoints: `GET/POST/DELETE /v1/tools/:name/secrets` (sin exponer plaintext).

### 8) Ejecución HTTP controlada

* Control por:

  * Config global (`HTTPTimeoutMS`, `HTTPMaxResponseBytes`, `HTTPRetries`)
  * Budget per-request (`X-Timeout-Ms`)
  * Límites por policy
* GET→query params solo flat primitives; POST/PUT/PATCH→JSON body.
* Retries: read configurable, write=0.

### 9) Rate limiting

* Redis distribuido o in-memory con cleanup.

### 10) Idempotency

* `Idempotency-Key` con persistencia y atomicidad (`ON CONFLICT DO NOTHING`).
* Replay/in-progress/conflict/failed terminal + staleness.

### 11) Timeout budget

* Clamp min/max configurables; consumido por etapas.

### 12) Auditoría + Compliance-friendly

* Audit append-only redactada + hash-chain.
* `GET /v1/audit` con filtros.
* `GET /v1/audit/export` streaming `jsonl|csv` + contract tests.

### 13) MCP nativo

* `/mcp` JSON-RPC 2.0: `tools/list`, `tools/get`, `tools/call`.
* Misma pipeline de control (idempotency temprana, DLP→policy, SSRF/egress, audit, etc.).
