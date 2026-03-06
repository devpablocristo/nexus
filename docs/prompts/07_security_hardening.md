# Prompt 07 — Hardening de seguridad y preparación para producción

## Contexto del proyecto

Nexus es una plataforma SaaS multi-tenant con estos servicios:

| Servicio | Stack | Puerto | DB | Notas |
|----------|-------|--------|----|-------|
| nexus-core | Go/Gin | 8080 | PostgreSQL `nexus` | Gateway principal, rate limiting Redis |
| nexus-saas | Go/Gin | 8082 | PostgreSQL `nexus_saas` | Billing, Admin, Webhooks |
| nexus-tower | Nginx + Vite/React | 4173 | — | SPA frontend |
| nexus-control-operators | Go | 8090 | File-based | Operadores deterministas |
| nexus-ai-operators | Python/FastAPI | 8000 | — | Operadores IA |

**Monorepo** con Go workspace (`go.work`), React frontend, Python service.

## Alcance obligatorio

Este prompt hereda los estándares de `docs/prompts/00_base_transversal.md`.

Todo lo definido acá es obligatorio para seguridad y readiness:
- hardening de servicios
- headers y límites
- auth protections
- dependency scanning
- secret handling
- métricas/paths sensibles

Nada de este prompt debe leerse como opcional por defecto.

## Prerequisito

Leer y respetar `docs/prompts/00_base_transversal.md` antes de ejecutar este prompt.

---

## Lo que YA existe (NO duplicar)

### Autenticación y autorización
- JWT/JWKS (Clerk) + API Key (`X-NEXUS-CORE-KEY`)
- Auth middleware en nexus-core y nexus-saas
- Scopes y permisos por org

### Rate limiting
- nexus-core: Redis/in-memory por org+tool, 60 req/min default (`NEXUS_RATE_LIMIT_DEFAULT_PER_MINUTE`)
- nexus-saas webhooks: Clerk 60/min, Stripe 120/min (in-memory)

### Validación de input
- `c.ShouldBindJSON` en todos los handlers Go
- JSON Schema para `input`/`output` de tools (`santhosh-tekuri/jsonschema/v5`)
- Pydantic en nexus-ai-operators

### Seguridad existente
- SSRF protection: allowlist + bloqueo IPs privadas/metadata (`pkg/utils/ssrf.go`)
- DLP: redacción de PII (credit_card, email, phone, JWT, api_key) (`internal/dlp/`)
- AES-GCM para secretos de tools (`pkg/utils/aesgcm.go`)
- Audit hash chain (integridad de logs)
- CORS configurable (`NEXUS_CORS_ALLOWED_ORIGINS`)
- Body limit en nexus-core: 256KB (`ginmw.BodyLimit`)
- Webhook signature verification: Svix (Clerk) + Stripe

### Docker
- nexus-core, nexus-saas, nexus-control-operators: `USER app` (uid 1000)
- Multi-stage builds con Alpine

### CI (`.github/workflows/ci.yml`)
- Go: `go vet` + `go test` + Docker build
- Python: `ruff check` + `mypy` + `pytest`
- Node: `npm run lint` + `npm run test` + `npm run build`
- E2E: `make e2e`, `make e2e-jwt`, `make e2e-operators`, `07_ai_operators.sh`

---

## Lo que FALTA (esto es lo que hay que implementar)

### 1. Security headers — Nginx (nexus-tower)

**Archivo:** `nexus-tower/nginx.conf`

Estado actual — solo tiene `Cache-Control` en `/assets/`. Agregar en el bloque `server`:

```nginx
# Dentro del bloque server {}
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "0" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: https:; connect-src 'self' ${VITE_NEXUS_CORE_URL} ${VITE_NEXUS_SAAS_URL} https://*.clerk.accounts.dev https://api.clerk.com; frame-src 'self' ${VITE_NEXUS_GRAFANA_URL}; frame-ancestors 'self';" always;
```

**Importante para CSP:**
- `frame-src` debe permitir la URL de Grafana (para los iframes de monitoring)
- `connect-src` debe permitir las URLs de los backends y Clerk
- `style-src 'unsafe-inline'` necesario para estilos inline de la app
- Si Clerk usa scripts externos, agregar su dominio a `script-src`

Para hacer la CSP configurable, usar `envsubst` en el entrypoint de nginx o un template.

### 2. Security headers — Go services (nexus-core y nexus-saas)

Crear un nuevo middleware Gin en `pkgs/go-pkg/http/middlewares/gin/security_headers.go`:

```go
func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        c.Header("Cache-Control", "no-store")
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Next()
    }
}
```

Registrar en ambos servicios (en `bootstrap_routes.go`), después de Recovery y antes de CORS.

### 3. Body limit faltante en nexus-saas

**Bug actual:** `nexus-saas/wire/bootstrap_routes.go` recibe `httpCfg` pero NO aplica `ginmw.BodyLimit`. Línea 56 tiene `_ = httpCfg`.

**Fix:** Agregar `ginmw.BodyLimit(httpCfg.MaxBodyBytes)` al middleware chain, igual que nexus-core.

### 4. nexus-ai-operators — Hardening

**Archivo:** `nexus-ai-operators/app/main.py` y nuevos middlewares.

#### 4a. Non-root Docker user

Editar `nexus-ai-operators/Dockerfile` para agregar:

```dockerfile
RUN adduser -D -u 1000 app
USER app
```

#### 4b. Rate limiting

Agregar rate limiting con `slowapi` o implementar manualmente:

```python
# En endpoints sensibles: /v1/assistant/query (LLM calls son costosos)
# Límite sugerido: 30 req/min por API key
```

#### 4c. Body size limit

Configurar en FastAPI/uvicorn:

```python
# En main.py o como middleware
from starlette.middleware.trustedhost import TrustedHostMiddleware
# uvicorn --limit-request-line 8190 --limit-max-header-size 8190
```

O middleware custom que rechace bodies > 1MB.

#### 4d. CORS

Agregar `CORSMiddleware` explícito de FastAPI, no dejarlo abierto por defecto.

### 5. nexus-tower Nginx — Non-root (opcional pero recomendado)

Cambiar a `nginx:1.27-alpine` con usuario no-root:

```dockerfile
FROM nginx:1.27-alpine
RUN chown -R nginx:nginx /usr/share/nginx/html /var/cache/nginx /var/log/nginx /etc/nginx/conf.d
USER nginx
```

Ajustar `listen 4173` a `listen 8080` si es necesario (puertos < 1024 requieren root).

### 6. Dependency scanning en CI

Agregar un job nuevo en `.github/workflows/ci.yml`:

```yaml
  security-scan:
    name: Security Scan
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # Go vulnerability check
      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"
      - name: Go vulnerability check (nexus-core)
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          cd nexus-core && govulncheck ./...
      - name: Go vulnerability check (nexus-saas)
        run: cd nexus-saas && govulncheck ./...
      - name: Go vulnerability check (nexus-control-operators)
        run: cd nexus-control-operators && govulncheck ./...

      # Python audit
      - uses: actions/setup-python@v5
        with:
          python-version: "3.11"
      - name: Python dependency audit
        run: |
          pip install pip-audit
          cd nexus-ai-operators && pip-audit .

      # Node audit
      - uses: actions/setup-node@v4
        with:
          node-version: 20
      - name: Node dependency audit
        run: cd nexus-tower && npm audit --audit-level=high
```

### 7. Dependabot

Crear `.github/dependabot.yml`:

```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: /nexus-core
    schedule:
      interval: weekly
    open-pull-requests-limit: 5

  - package-ecosystem: gomod
    directory: /nexus-saas
    schedule:
      interval: weekly
    open-pull-requests-limit: 5

  - package-ecosystem: gomod
    directory: /nexus-control-operators
    schedule:
      interval: weekly
    open-pull-requests-limit: 5

  - package-ecosystem: gomod
    directory: /pkgs/go-pkg
    schedule:
      interval: weekly
    open-pull-requests-limit: 5

  - package-ecosystem: pip
    directory: /nexus-ai-operators
    schedule:
      interval: weekly
    open-pull-requests-limit: 5

  - package-ecosystem: npm
    directory: /nexus-tower
    schedule:
      interval: weekly
    open-pull-requests-limit: 5

  - package-ecosystem: docker
    directory: /nexus-core
    schedule:
      interval: monthly

  - package-ecosystem: docker
    directory: /nexus-saas
    schedule:
      interval: monthly

  - package-ecosystem: docker
    directory: /nexus-tower
    schedule:
      interval: monthly

  - package-ecosystem: docker
    directory: /nexus-ai-operators
    schedule:
      interval: monthly

  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 3
```

### 8. Gin DTO validation tags

Agregar `binding:"required"` a campos obligatorios en los DTOs. Ejemplo en `nexus-core/internal/gateway/handler/dto/dto.go`:

```go
// Antes (actual):
type RunRequest struct {
    RequestID string         `json:"request_id"`
    ToolName  string         `json:"tool_name"`
    ToolID    string         `json:"tool_id"`
    Input     map[string]any `json:"input"`
    Context   map[string]any `json:"context"`
}

// Después:
type RunRequest struct {
    RequestID string         `json:"request_id"`
    ToolName  string         `json:"tool_name" binding:"required_without=ToolID"`
    ToolID    string         `json:"tool_id" binding:"required_without=ToolName"`
    Input     map[string]any `json:"input" binding:"required"`
    Context   map[string]any `json:"context"`
}
```

Revisar TODOS los DTOs en:
- `nexus-core/internal/*/handler/dto/dto.go`
- `nexus-saas/internal/*/handler/dto/` o directamente en `handler.go`

Y agregar tags `binding:"required"` donde el campo es obligatorio. No romper backward-compat: si un campo era opcional antes, dejarlo así.

### 9. CORS hardening en docker-compose

**Bug actual:** `docker-compose.yml` tiene `NEXUS_CORS_ALLOWED_ORIGINS: "${NEXUS_CORS_ALLOWED_ORIGINS:-*}"`. El default `*` es inseguro.

**Fix:** Cambiar el default a los orígenes de desarrollo:

```yaml
NEXUS_CORS_ALLOWED_ORIGINS: "${NEXUS_CORS_ALLOWED_ORIGINS:-http://localhost:5173,http://localhost:5174}"
```

En production (Terraform/deploy), usar el dominio real.

### 10. Secret rotation runbook

Crear `docs/runbooks/SECRET_ROTATION.md` con procedimientos para rotar:

1. **NEXUS_MASTER_KEY** (AES-GCM para secrets de tools)
   - Generar nueva key, re-cifrar secrets existentes, actualizar en Secrets Manager, deploy
2. **NEXUS_CORE_INTERNAL_KEY / NEXUS_SAAS_INTERNAL_KEY** (API keys internas)
   - Generar nuevas, actualizar en ambos servicios simultáneamente
3. **Clerk keys** (CLERK_SECRET_KEY, CLERK_WEBHOOK_SECRET)
   - Rotar desde dashboard de Clerk, actualizar en Secrets Manager
4. **Stripe keys** (STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET)
   - Rotar desde dashboard de Stripe, actualizar en Secrets Manager
5. **Database credentials**
   - Rotar via RDS, actualizar connection strings en Secrets Manager

Incluir sección de "Emergency rotation" para compromisos.

### 11. Proteger `/metrics` endpoints

El endpoint `/metrics` de nexus-ai-operators (Prometheus) está expuesto sin auth. Opciones:
- Moverlo a un puerto interno separado (preferido)
- O protegerlo con la misma API key

En nexus-core y nexus-saas `/metrics` también está sin auth, pero en producción solo es accesible dentro del VPC (Prometheus scraping). Documentar que NO debe exponerse públicamente.

---

## Archivos a crear

| Archivo | Descripción |
|---------|-------------|
| `pkgs/go-pkg/http/middlewares/gin/security_headers.go` | Middleware de headers de seguridad |
| `pkgs/go-pkg/http/middlewares/gin/security_headers_test.go` | Tests del middleware |
| `.github/dependabot.yml` | Configuración de Dependabot |
| `docs/runbooks/SECRET_ROTATION.md` | Runbook de rotación de secrets |

## Archivos a modificar

| Archivo | Cambio |
|---------|--------|
| `nexus-tower/nginx.conf` | Agregar security headers y CSP |
| `nexus-core/wire/bootstrap_routes.go` | Registrar `SecurityHeaders()` middleware |
| `nexus-saas/wire/bootstrap_routes.go` | Registrar `SecurityHeaders()` + `BodyLimit` middleware |
| `nexus-ai-operators/Dockerfile` | Agregar `USER app` (non-root) |
| `nexus-ai-operators/app/main.py` | CORS middleware, body limit |
| `docker-compose.yml` | Cambiar CORS default de `*` a orígenes locales |
| `.github/workflows/ci.yml` | Agregar job `security-scan` |
| `nexus-core/internal/*/handler/dto/dto.go` | Agregar `binding:"required"` tags |
| `nexus-saas/internal/*/handler/dto/*.go` | Agregar `binding:"required"` tags |

---

## Criterios de éxito

1. **Headers de seguridad:**
   - [ ] `curl -I http://localhost:5174` muestra X-Frame-Options, X-Content-Type-Options, CSP, Referrer-Policy
   - [ ] `curl -I http://localhost:8080/readyz` muestra X-Content-Type-Options, X-Frame-Options
   - [ ] `curl -I http://localhost:8082/health` muestra los mismos headers
   - [ ] Los iframes de Grafana en `/monitoring` siguen funcionando (CSP permite frame-src)

2. **Body limit en nexus-saas:**
   - [ ] `curl -X POST http://localhost:8082/v1/admin/bootstrap -d @big_payload.json` retorna 413 si > 256KB

3. **Docker non-root:**
   - [ ] `docker compose exec nexus-ai-operators whoami` retorna `app`, no `root`
   - [ ] `docker compose exec nexus-tower whoami` retorna `nginx` o `app`

4. **CI security scanning:**
   - [ ] Job `security-scan` en CI ejecuta `govulncheck`, `pip-audit`, `npm audit`
   - [ ] El pipeline NO falla por vulnerabilidades conocidas de bajo riesgo

5. **Dependabot:**
   - [ ] `.github/dependabot.yml` existe con ecosistemas: gomod, pip, npm, docker, github-actions

6. **CORS:**
   - [ ] docker-compose.yml NO tiene `*` como default de CORS
   - [ ] En producción, CORS solo permite el dominio real

7. **DTO validation:**
   - [ ] Enviar `POST /v1/run` sin `input` retorna 400 (no 500)
   - [ ] Enviar requests sin campos obligatorios retorna errores claros

8. **Secret rotation:**
   - [ ] `docs/runbooks/SECRET_ROTATION.md` documenta el procedimiento para cada secret

9. **Compilación y tests:**
   - [ ] `cd nexus-core && go build ./...` ✓
   - [ ] `cd nexus-saas && go build ./...` ✓
   - [ ] `cd nexus-tower && npm run build` ✓
   - [ ] `cd nexus-ai-operators && pytest -q` ✓
   - [ ] `make e2e` pasa (los e2e existentes no se rompen)

---

## Orden de ejecución recomendado

**Aclaración importante**: este orden existe solo para respetar dependencias técnicas. Todo el contenido del prompt sigue siendo obligatorio.

1. Security headers middleware Go → registrar en ambos servicios
2. Body limit en nexus-saas
3. Nginx security headers + CSP
4. Docker non-root para ai-operators y tower
5. CORS default fix
6. DTO validation tags
7. CI security-scan job
8. Dependabot config
9. Secret rotation runbook
10. Verificar que todos los tests y e2e pasan
