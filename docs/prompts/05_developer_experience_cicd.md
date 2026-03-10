# Prompt 05 — Experiencia de desarrollo y CI/CD

## Contexto del proyecto

Nexus es una plataforma SaaS (Go + React/TypeScript) compuesta por:

| Servicio | Stack | Puerto | Descripción |
|----------|-------|--------|-------------|
| `nexus-core` | Go/Gin | 8080 | API Gateway: tools, policies, egress, secrets, audit, run |
| `nexus-saas` | Go/Gin | 8082 | SaaS layer: billing, admin, users, incidents, notifications |
| `nexus-tower` | React/Vite | 5173 | Frontend SPA |
| `nexus-control-operators` | Go | — | Operadores deterministas |
| `nexus-ai-operators` | Python/FastAPI | 8000 | Operadores IA |

## Alcance obligatorio

Este prompt hereda los estándares de `docs/prompts/00_base_transversal.md`.

Todo lo definido acá es obligatorio para DX y release engineering:
- OpenAPI y contratos
- SDKs y developer portal
- CI/CD
- e2e y automatización de calidad
- alineación entre docs, builds y artefactos publicados

El orden sugerido es solo técnico.

## Prerequisito

Leer y respetar `docs/prompts/00_base_transversal.md` antes de ejecutar este prompt.

---

## Lo que YA existe

### OpenAPI & Swagger

- `pkgs/contracts/openapi.nexus-core.snapshot.yaml` — OpenAPI 3.0.3 completo para nexus-core (~2480 líneas)
- `nexus-core` sirve Swagger UI en `GET /docs` y el spec en `GET /openapi.yaml`
- **nexus-saas NO tiene OpenAPI spec** — solo hace proxy del spec de nexus-core

### SDKs

- `sdks/python-sdk/` — `NexusClient`, `AsyncNexusClient`, integrations LangChain + OpenAI Agents
- `sdks/typescript-sdk/` — `NexusClient` con `run`, `simulate`, `tools`, `policies`, `audit`, `approvals`
- Ambos SDKs apuntan a **nexus-core** (gateway). No hay SDK para nexus-saas.

### Postman

- `docs/postman/nexus-core.postman_collection.json` — colección para nexus-core
- **No hay colección Postman para nexus-saas**

### CI/CD

- `.github/workflows/ci.yml` con 6 jobs: nexus-core, nexus-control-operators, nexus-ai-operators, nexus-tower, docker-build, e2e
- **Falta job para nexus-saas** (tests y Docker build)
- **Bug**: el job e2e usa `make jwt-e2e` pero el Makefile define `e2e-jwt`
- **Falta**: e2e para nexus-saas, ai-operators, notifications

### Developer portal

- No existe página `/developer` ni `/docs` en nexus-tower
- La documentación está dispersa en `docs/DOC.md`, READMEs individuales, y `sdks/*/README.md`

---

## Qué implementar

### Fase 1 — OpenAPI spec para nexus-saas

Crear `pkgs/contracts/openapi.nexus-saas.snapshot.yaml` (OpenAPI 3.0.3).

**Cómo obtener las rutas**: leer el método `Register()` de cada handler en `control-plane/internal/*/handler.go` y los DTOs en `handler/dto/dto.go`.

#### Endpoints de nexus-saas a documentar

**Públicos (sin auth):**

| Método | Ruta | Tag | Handler |
|--------|------|-----|---------|
| GET | /health | Health | inline |
| GET | /healthz | Health | inline |
| GET | /readyz | Health | inline |
| POST | /v1/orgs | Onboarding | org |
| GET | /v1/auth/oidc/config | Auth | identity |
| GET | /v1/auth/oidc/authorize | Auth | identity |
| GET | /v1/auth/oidc/callback | Auth | identity |
| POST | /v1/webhooks/clerk | Webhooks | clerkwebhook |
| POST | /v1/webhooks/stripe | Webhooks | billing |

**Internos (M2M, no public):**

| Método | Ruta | Tag | Handler |
|--------|------|-----|---------|
| POST | /internal/usage/events | Internal | contracts |
| POST | /internal/events | Internal | contracts |
| GET | /internal/entitlements/:org_id | Internal | contracts |
| GET | /internal/runtime-overrides/:org_id/:tool_name | Internal | contracts |

**Autenticados (JWT/API key):**

| Método | Ruta | Tag | Handler |
|--------|------|-----|---------|
| GET | /v1/admin/bootstrap | Admin | admin |
| GET | /v1/admin/tenant-settings | Admin | admin |
| PUT | /v1/admin/tenant-settings | Admin | admin |
| GET | /v1/admin/activity | Admin | admin |
| GET | /v1/billing/status | Billing | billing |
| POST | /v1/billing/checkout | Billing | billing |
| POST | /v1/billing/portal | Billing | billing |
| GET | /v1/billing/usage | Billing | billing |
| GET | /v1/events | Events | events |
| POST | /v1/actions/apply | Actions | actions |
| POST | /v1/actions/rollback | Actions | actions |
| GET | /v1/actions | Actions | actions |
| POST | /v1/incidents | Incidents | incidents |
| GET | /v1/incidents | Incidents | incidents |
| GET | /v1/incidents/:id | Incidents | incidents |
| POST | /v1/incidents/:id/close | Incidents | incidents |
| GET | /v1/notifications/preferences | Notifications | notifications |
| PUT | /v1/notifications/preferences | Notifications | notifications |
| GET | /v1/alert-rules | Alerts | alerts |
| POST | /v1/alert-rules | Alerts | alerts |
| DELETE | /v1/alert-rules/:id | Alerts | alerts |
| GET | /v1/sessions/:session_id | Sessions | session |
| POST | /v1/policy-proposals | PolicyProposals | policyproposal |
| GET | /v1/policy-proposals | PolicyProposals | policyproposal |
| POST | /v1/policy-proposals/:id/approve | PolicyProposals | policyproposal |
| POST | /v1/policy-proposals/:id/reject | PolicyProposals | policyproposal |
| POST | /v1/policy-proposals/:id/shadow | PolicyProposals | policyproposal |
| POST | /v1/assistant/query | Assistant | assistant |
| POST | /v1/assistant/tick | Assistant | assistant |
| GET | /v1/users/me | Users | users |
| GET | /v1/orgs/:org_id/members | Users | users |
| GET | /v1/orgs/:org_id/api-keys | Users | users |
| POST | /v1/orgs/:org_id/api-keys | Users | users |
| DELETE | /v1/orgs/:org_id/api-keys/:id | Users | users |
| POST | /v1/orgs/:org_id/api-keys/:id/rotate | Users | users |
| GET | /v1/audit | Audit | coreproxy |
| GET | /v1/audit/export | Audit | coreproxy |
| GET | /v1/approvals | Approvals | coreproxy |
| GET | /v1/approvals/:id | Approvals | coreproxy |
| POST | /v1/approvals/:id/approve | Approvals | coreproxy |
| POST | /v1/approvals/:id/reject | Approvals | coreproxy |

**Para cada endpoint**, leer el DTO correspondiente en `handler/dto/dto.go` del módulo y documentar:
- Request body (schemas)
- Response body (schemas)
- Path/query parameters
- Security requirements
- Response codes

**Formato del spec**: seguir exactamente el mismo formato de `pkgs/contracts/openapi.nexus-core.snapshot.yaml`. Misma estructura de info, servers, security schemes, tags, paths, components.

**Security schemes**:

```yaml
securityDefinitions:
  BearerAuth:
    type: http
    scheme: bearer
    bearerFormat: JWT
  NexusApiKey:
    type: apiKey
    in: header
    name: X-NEXUS-CORE-KEY
```

#### Servir el spec en nexus-saas

1. En `control-plane/wire/bootstrap_routes.go`, agregar antes del return:

```go
r.GET("/openapi.yaml", func(c *gin.Context) {
    c.File("docs/openapi.yaml")
})
r.GET("/docs", func(c *gin.Context) {
    c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML))
})
```

2. Copiar el spec snapshot a `control-plane/docs/openapi.yaml` durante el Docker build.

3. Agregar Swagger UI HTML (mismo patrón que nexus-core).

---

### Fase 2 — Postman collection para nexus-saas

Crear `docs/postman/nexus-saas.postman_collection.json` con:

- Variables: `base_url` (default `http://localhost:8082`), `api_key`, `org_id`
- Carpetas por tag: Admin, Billing, Events, Actions, Incidents, Notifications, Alerts, Users, PolicyProposals, Assistant, Audit
- Cada request con body de ejemplo, headers correctos, descripción

Seguir el mismo formato de `docs/postman/nexus-core.postman_collection.json`.

---

### Fase 3 — Fix CI/CD

#### 3.1 Agregar job `nexus-saas` al CI

En `.github/workflows/ci.yml`, agregar:

```yaml
  nexus-saas:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: control-plane
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
      - name: Vet
        run: go vet ./...
      - name: Test
        run: go test ./...
      - name: Docker build (verify only)
        run: docker build -t nexus-saas:ci .
```

Agregar `nexus-saas` a la lista `needs` del job `docker-build`:

```yaml
  docker-build:
    needs: [nexus-core, nexus-saas, nexus-control-operators, nexus-ai-operators, nexus-tower]
```

#### 3.2 Corregir target e2e JWT

En el job `e2e` del CI, cambiar:

```yaml
      - name: Run JWT e2e tests
        run: make jwt-e2e
```

Por:

```yaml
      - name: Run JWT e2e tests
        run: make e2e-jwt     # ← correcto, coincide con Makefile
```

#### 3.3 Agregar targets e2e faltantes al CI

Agregar al job `e2e`:

```yaml
      - name: Run e2e operators
        run: make e2e-operators
      - name: Run e2e AI operators
        run: bash scripts/e2e/07_ai_operators.sh
```

#### 3.4 Agregar make target para e2e completo

En el `Makefile`, agregar:

```makefile
e2e-all:
	bash scripts/e2e/01_run_echo.sh
	bash scripts/e2e/03_full_core_e2e.sh
	bash scripts/e2e/04_core_gateway_isolated.sh
	bash scripts/e2e/05_core_jwt_auth.sh
	bash scripts/e2e/06_control_operators.sh
	bash scripts/e2e/07_ai_operators.sh
```

---

### Fase 4 — Developer Portal en nexus-tower

#### 4.1 Nueva página `/developer`

Crear `tower/src/pages/DeveloperPage.tsx`:

```
┌────────────────────────────────────────────────────────┐
│  Developer Portal                                       │
├────────────────────────────────────────────────────────┤
│                                                          │
│  ┌─ Getting Started ──────────────────────────────────┐ │
│  │  1. Create an API key in Settings → API Keys       │ │
│  │  2. Register your first tool via POST /v1/tools    │ │
│  │  3. Make your first request: POST /v1/run          │ │
│  │                                                     │ │
│  │  [View full guide →]                               │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌─ API Reference ────────────────────────────────────┐ │
│  │                                                     │ │
│  │  ┌──────────────────┐  ┌──────────────────┐        │ │
│  │  │  Nexus Core API  │  │  Nexus SaaS API  │        │ │
│  │  │  Gateway, Tools  │  │  Billing, Admin  │        │ │
│  │  │  [Open Docs →]   │  │  [Open Docs →]   │        │ │
│  │  └──────────────────┘  └──────────────────┘        │ │
│  │                                                     │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌─ SDKs ─────────────────────────────────────────────┐ │
│  │                                                     │ │
│  │  ┌──────────────────┐  ┌──────────────────┐        │ │
│  │  │  Python SDK      │  │  TypeScript SDK  │        │ │
│  │  │  pip install     │  │  npm install     │        │ │
│  │  │  nexus-sdk       │  │  @nexus/sdk      │        │ │
│  │  │                  │  │                  │        │ │
│  │  │  code example    │  │  code example    │        │ │
│  │  └──────────────────┘  └──────────────────┘        │ │
│  │                                                     │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌─ Quick Reference ──────────────────────────────────┐ │
│  │                                                     │ │
│  │  Base URLs:                                         │ │
│  │    Core API: {VITE_NEXUS_CORE_URL}                 │ │
│  │    SaaS API: {VITE_NEXUS_SAAS_URL}                 │ │
│  │                                                     │ │
│  │  Authentication:                                    │ │
│  │    Header: X-NEXUS-CORE-KEY: <your-api-key>        │ │
│  │    Or: Authorization: Bearer <jwt>                  │ │
│  │                                                     │ │
│  │  Key endpoints:                                     │ │
│  │    POST /v1/run     — Execute a tool                │ │
│  │    GET  /v1/tools   — List registered tools         │ │
│  │    POST /v1/tools   — Register a new tool           │ │
│  │    GET  /v1/audit   — View audit log                │ │
│  │                                                     │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌─ Downloads ────────────────────────────────────────┐ │
│  │  Postman Collection (Core)   [Download .json]      │ │
│  │  Postman Collection (SaaS)   [Download .json]      │ │
│  │  OpenAPI Spec (Core)         [Download .yaml]      │ │
│  │  OpenAPI Spec (SaaS)         [Download .yaml]      │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
└────────────────────────────────────────────────────────┘
```

La página es estática (no necesita API calls). Lee las URLs de `VITE_NEXUS_CORE_URL` y `VITE_NEXUS_SAAS_URL`.

Los links a "Open Docs" abren la Swagger UI del servicio correspondiente en nueva pestaña:
- Core: `{VITE_NEXUS_CORE_URL}/docs`
- SaaS: `{VITE_NEXUS_SAAS_URL}/docs`

Los code examples de SDKs son snippets estáticos:

Python:
```python
from nexus_sdk import NexusClient

client = NexusClient(base_url="...", api_key="...")
result = client.run(tool="my-tool", payload={"prompt": "hello"})
print(result)
```

TypeScript:
```typescript
import { NexusClient } from '@nexus/sdk';

const client = new NexusClient({ baseUrl: '...', apiKey: '...' });
const result = await client.run('my-tool', { prompt: 'hello' });
console.log(result);
```

#### 4.2 Ruta y navegación

En `App.tsx`, agregar:
```tsx
<Route path="/developer" element={<DeveloperPage />} />
```

En `Shell.tsx`, agregar al array `navItems`:
```typescript
{ to: '/developer', label: 'Developer' },
```

---

### Fase 5 — Mejoras menores

#### 5.1 Copiar OpenAPI spec en Docker build de nexus-saas

En `control-plane/Dockerfile`, agregar step para copiar el spec:

```dockerfile
COPY docs/openapi.yaml /app/docs/openapi.yaml
```

Crear el archivo `control-plane/docs/openapi.yaml` como symlink o copia de `pkgs/contracts/openapi.nexus-saas.snapshot.yaml`.

#### 5.2 Verificar que nexus-core copia su spec en Docker

En `data-plane/Dockerfile`, verificar que `docs/openapi.yaml` se copia al contenedor. Si no existe, crearlo como copia de `pkgs/contracts/openapi.nexus-core.snapshot.yaml`.

#### 5.3 Actualizar .env.example

Si hay nuevas variables (no debería haberlas para este prompt), agregarlas.

---

## Reglas de implementación

1. **OpenAPI spec**: leer los DTOs (`handler/dto/dto.go`) de CADA módulo de nexus-saas para obtener request/response schemas exactos. NO inventar campos.
2. **Formato**: seguir exactamente el formato de `pkgs/contracts/openapi.nexus-core.snapshot.yaml` (OpenAPI 3.0.3, misma estructura).
3. **CI/CD**: no romper nada existente. Solo agregar y corregir.
4. **Frontend**: seguir los patrones existentes de nexus-tower (React, TanStack Query donde aplique, CSS existente).
5. **No agregar dependencias npm**: la página Developer es estática.
6. **No agregar dependencias Go**: Swagger UI se carga desde CDN (mismo patrón que nexus-core).
7. **Postman**: seguir el formato exacto de `docs/postman/nexus-core.postman_collection.json`.

---

## Criterios de éxito

- [ ] `pkgs/contracts/openapi.nexus-saas.snapshot.yaml` creado con TODOS los endpoints de nexus-saas
- [ ] nexus-saas sirve `/openapi.yaml` (su propio spec, no proxy)
- [ ] nexus-saas sirve `/docs` (Swagger UI apuntando a su spec)
- [ ] `docs/postman/nexus-saas.postman_collection.json` creado con todos los endpoints
- [ ] CI: job `nexus-saas` agregado (vet, test, Docker build)
- [ ] CI: `make jwt-e2e` corregido a `make e2e-jwt`
- [ ] CI: e2e-operators y e2e AI-operators agregados
- [ ] Makefile: target `e2e-all` agregado
- [ ] nexus-tower: `DeveloperPage` con getting-started, API ref, SDKs, quick ref, downloads
- [ ] nexus-tower: ruta `/developer` en App.tsx
- [ ] nexus-tower: nav item "Developer" en Shell.tsx
- [ ] Docker: nexus-saas copia `docs/openapi.yaml` en build
- [ ] Docker: nexus-core copia `docs/openapi.yaml` en build (verificar)
- [ ] `go test ./...` en nexus-saas pasa
- [ ] `go build ./...` en nexus-saas compila
- [ ] `npm run build` en nexus-tower pasa
- [ ] Tests e2e existentes (01-07) siguen pasando

---

## Orden de ejecución recomendado

**Aclaración importante**: este orden existe solo para respetar dependencias técnicas. Todo el contenido del prompt sigue siendo obligatorio.

1. Leer TODOS los DTOs de nexus-saas (`handler/dto/dto.go` de cada módulo)
2. Crear `pkgs/contracts/openapi.nexus-saas.snapshot.yaml`
3. Copiar a `control-plane/docs/openapi.yaml`
4. Agregar Swagger UI + spec serving en nexus-saas `bootstrap_routes.go`
5. Actualizar Dockerfile de nexus-saas para copiar spec
6. Verificar Dockerfile de nexus-core
7. Crear `docs/postman/nexus-saas.postman_collection.json`
8. Fix CI: agregar job nexus-saas, corregir jwt-e2e, agregar e2e-operators
9. Agregar `e2e-all` al Makefile
10. Crear `DeveloperPage.tsx` en nexus-tower
11. Agregar ruta y nav item
12. Verificar builds y tests
