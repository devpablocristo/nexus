# Prompt 09 — Polish final y preparación de lanzamiento

## Contexto del proyecto

Nexus es una plataforma SaaS multi-tenant con 8 prompts ya implementados:
- 01: User Identity (Clerk + AWS)
- 02: Billing (Stripe)
- 03: Admin Console UI
- 04: Email & Notifications (SES)
- 05: Developer Experience & CI/CD
- 06: Production Infrastructure (Terraform)
- 07: Security Hardening
- 08: Monitoring & Observability

**Score actual: 94% (160/170)**. Este es el último prompt para llevar todo a producción.

| Servicio | Stack | Puerto |
|----------|-------|--------|
| nexus-core | Go/Gin | 8080 |
| nexus-saas | Go/Gin | 8082 |
| nexus-tower | Nginx/React | 4173 |
| nexus-control-operators | Go | 8090 |
| nexus-ai-operators | Python/FastAPI | 8000 |

## Alcance obligatorio

Este prompt hereda los estándares de `docs/prompts/00_base_transversal.md`.

Todo lo definido acá es obligatorio para salida a producción:
- smoke tests
- tenant lifecycle
- páginas/estados de error
- load testing
- polish de SDKs y onboarding
- changelog y launch checklist

El orden sugerido es técnico; no reduce alcance.

## Prerequisito

Leer y respetar `docs/prompts/00_base_transversal.md` antes de ejecutar este prompt.

---

## Lo que YA existe (NO duplicar)

- Auth completa (Clerk JWT + API keys + OIDC + webhooks)
- Billing con Stripe (checkout, portal, webhooks, usage metering)
- Admin console (dashboard, tenant settings, activity log)
- Email notifications (SES/SMTP, templates HTML, preferences, deduplication)
- OpenAPI specs, Postman collections, developer portal en Tower
- CI/CD (GitHub Actions: test, build, security-scan, e2e, deploy)
- Terraform (10 módulos: networking, RDS, ElastiCache, ECS, ALB, CDN, DNS, secrets, monitoring, ECR)
- Security headers, body limits, non-root Docker, Dependabot, secret rotation runbook
- Prometheus (4 scrape targets), Grafana (3 dashboards), alerting rules, alert evaluation worker
- Retry con exponential backoff en control-operators
- SLO/SLI definitions
- 7 E2E test scripts
- Python SDK (`sdks/python-sdk/`) y TypeScript SDK (`sdks/typescript-sdk/`)
- Database migrations con rollback (13 core + 5 saas)

---

## Lo que FALTA (implementar)

### 1. Smoke test post-deploy

Crear `scripts/smoke/smoke_prod.sh` que valide el stack completo después de un deploy. Debe recibir URLs como parámetros:

```bash
#!/usr/bin/env bash
# Usage: ./smoke_prod.sh [API_BASE_URL] [SAAS_BASE_URL] [TOWER_URL]
# Example: ./smoke_prod.sh https://api.nexus.io https://saas.nexus.io https://app.nexus.io

API_URL="${1:-http://localhost:8080}"
SAAS_URL="${2:-http://localhost:8082}"
TOWER_URL="${3:-http://localhost:5174}"
```

Checks a ejecutar:
1. `GET $API_URL/readyz` → 200
2. `GET $SAAS_URL/health` → 200
3. `GET $TOWER_URL/` → 200 (HTML)
4. `GET $API_URL/metrics` → 200 (contiene `nexus_`)
5. `GET $SAAS_URL/metrics` → 200 (contiene `nexus_saas_`)
6. `GET $API_URL/v1/tools` con API key → 200 (JSON array)
7. `GET $SAAS_URL/docs` → 200 (Swagger UI)
8. Verificar headers de seguridad en respuesta de Tower (X-Content-Type-Options, CSP)

Output: tabla de resultados `[PASS/FAIL]` por check, exit code 0 si todo pasa.

### 2. Tenant lifecycle (suspend/delete + GDPR)

#### 2a. Suspend tenant

Agregar endpoint en nexus-saas:

```
PUT /v1/admin/tenants/:org_id/suspend
```

- Seta `tenant_settings.status = 'suspended'`
- Envía notificación `tenant_suspended` al owner
- Mientras esté suspendido, nexus-core debe rechazar requests de ese org (verificar en entitlements)

#### 2b. Reactivate tenant

```
PUT /v1/admin/tenants/:org_id/reactivate
```

- Seta `tenant_settings.status = 'active'`
- Envía notificación `tenant_reactivated`

#### 2c. Delete tenant (soft-delete + GDPR cleanup)

```
DELETE /v1/admin/tenants/:org_id
```

- Soft-delete: seta `tenant_settings.deleted_at = NOW()`, `status = 'deleted'`
- Datos quedan 30 días para recuperación
- Después de 30 días, un job puede hacer hard-delete (mencionar en el código pero no implementar el cron)

#### 2d. Migration

Agregar columna a `tenant_settings`:

```sql
ALTER TABLE tenant_settings
  ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'suspended', 'deleted')),
  ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
```

#### 2e. Entitlements check

En nexus-core, al consultar entitlements de nexus-saas, si `status != 'active'`, retornar error 403 "tenant suspended/deleted".

### 3. Frontend error pages

#### 3a. Página 404

Crear `tower/src/pages/NotFoundPage.tsx`:
- Diseño limpio con "404 — Page not found"
- Botón "Go to Dashboard" que navega a `/tools`

#### 3b. Página para tenant suspendido

Crear `tower/src/pages/SuspendedPage.tsx`:
- Mensaje "Your account has been suspended"
- Enlace a soporte o billing

#### 3c. Actualizar rutas

En `App.tsx`, cambiar el catch-all de `<Navigate to="/tools" />` a `<NotFoundPage />`:

```tsx
<Route path="*" element={<NotFoundPage />} />
```

#### 3d. Mejorar ErrorBoundary

Agregar al `ErrorBoundary.tsx` existente:
- Botón "Report issue" (mailto: o link externo)
- Mostrar error ID para debugging
- "Contact support" link

### 4. Load testing con k6

Crear `scripts/loadtest/k6_gateway.js`:

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 10 },   // ramp up
    { duration: '1m',  target: 50 },   // sustained
    { duration: '30s', target: 100 },  // peak
    { duration: '30s', target: 0 },    // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],  // SLO: p95 < 500ms
    http_req_failed: ['rate<0.01'],    // SLO: error rate < 1%
  },
};

export default function () {
  const url = `${__ENV.API_URL || 'http://localhost:8080'}/v1/run`;
  const payload = JSON.stringify({
    tool_name: 'echo',
    input: { message: `k6-${__ITER}` },
  });
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-NEXUS-CORE-KEY': __ENV.API_KEY || 'dev-api-key',
    },
  };

  const res = http.post(url, payload, params);
  check(res, {
    'status is 200': (r) => r.status === 200,
    'latency < 500ms': (r) => r.timings.duration < 500,
  });
  sleep(0.1);
}
```

Agregar `scripts/loadtest/README.md` con instrucciones:
```
# Install: brew install k6
# Run: k6 run --env API_URL=http://localhost:8080 --env API_KEY=xxx scripts/loadtest/k6_gateway.js
```

### 5. SDK polish

#### 5a. TypeScript SDK README

Crear `sdks/typescript-sdk/README.md`:
- Installation (`npm install @nexus/sdk`)
- Quick start (crear client, run tool)
- API reference
- Configuration options

#### 5b. Go SDK para consumidores

Crear `sdks/go-sdk/` con un client mínimo:

```go
package nexus

type Client struct {
    BaseURL string
    APIKey  string
    HTTP    *http.Client
}

func NewClient(baseURL, apiKey string) *Client { ... }

func (c *Client) RunTool(ctx context.Context, req RunRequest) (*RunResponse, error) { ... }
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) { ... }
```

Con `go.mod`, README, y ejemplo de uso.

### 6. Onboarding first-time experience

Crear `tower/src/pages/OnboardingPage.tsx`:

Un wizard de 3 pasos para nuevos tenants:
1. **Welcome** — nombre de org, confirmar plan
2. **Register first tool** — formulario simplificado para crear una tool
3. **Test it** — botón para hacer un test run con la tool creada

Ruta: `/onboarding`

Lógica:
- Si el tenant tiene 0 tools, redirigir a `/onboarding` automáticamente
- Si ya tiene tools, saltar el onboarding
- Botón "Skip" en cada paso

### 7. CHANGELOG y launch checklist

#### 7a. CHANGELOG.md

Crear `CHANGELOG.md` en la raíz:

```markdown
# Changelog

All notable changes to this project will be documented in this file.

## [0.9.0] - 2026-03-05

### Added
- User identity with Clerk (JWT, OIDC, webhooks)
- Billing with Stripe (checkout, portal, usage metering)
- Admin console (dashboard, settings, activity log)
- Email notifications (SES/SMTP, preferences, deduplication)
- Developer portal with OpenAPI, Postman, SDKs
- Production infrastructure (Terraform, ECS, RDS, CloudFront)
- Security hardening (CSP, HSTS, Dependabot, govulncheck)
- Monitoring (Prometheus, Grafana, alerting rules, SLO/SLI)
- Tenant lifecycle (suspend, reactivate, soft-delete)
- Load testing with k6
- Smoke test suite for production deploys
- Python, TypeScript, and Go SDKs

### Security
- Security headers on all services
- Non-root Docker containers
- Body size limits
- Dependency scanning in CI
```

#### 7b. Launch checklist

Crear `docs/runbooks/LAUNCH_CHECKLIST.md`:

```markdown
# Production Launch Checklist

## Pre-launch
- [ ] All Terraform modules applied (staging first, then prod)
- [ ] DNS configured (Route53 → ALB, CloudFront)
- [ ] TLS certificates provisioned (ACM)
- [ ] Clerk production instance configured
- [ ] Stripe production keys + webhooks configured
- [ ] SES production access (out of sandbox)
- [ ] Secrets in AWS Secrets Manager
- [ ] Database migrations applied
- [ ] Seed data loaded (if needed)

## Deploy
- [ ] CI green on main
- [ ] Docker images pushed to ECR
- [ ] ECS services updated
- [ ] CloudFront invalidation complete

## Post-deploy validation
- [ ] Run smoke_prod.sh against production URLs
- [ ] Verify health endpoints for all services
- [ ] Verify Prometheus targets are UP
- [ ] Verify Grafana dashboards show data
- [ ] Test user sign-up flow (Clerk)
- [ ] Test billing flow (Stripe test mode → live)
- [ ] Test tool registration + execution
- [ ] Test email delivery (SES)

## Monitoring
- [ ] CloudWatch alarms configured and SNS subscribed
- [ ] Grafana alerts configured
- [ ] On-call rotation defined
- [ ] Incident response runbook reviewed

## Security
- [ ] Rotate all development secrets
- [ ] CORS origins set to production domains only
- [ ] CSP connect-src updated with production URLs
- [ ] Verify non-root containers
- [ ] Review Dependabot PRs
```

---

## Archivos a crear

| Archivo | Descripción |
|---------|-------------|
| `scripts/smoke/smoke_prod.sh` | Smoke test post-deploy |
| `scripts/loadtest/k6_gateway.js` | Load test con k6 |
| `scripts/loadtest/README.md` | Instrucciones de load testing |
| `control-plane/migrations/0006_tenant_lifecycle.up.sql` | Migration para status + deleted_at |
| `control-plane/migrations/0006_tenant_lifecycle.down.sql` | Rollback |
| `tower/src/pages/NotFoundPage.tsx` | Página 404 |
| `tower/src/pages/SuspendedPage.tsx` | Página tenant suspendido |
| `tower/src/pages/OnboardingPage.tsx` | Wizard de onboarding |
| `sdks/go-sdk/` | Go SDK para consumidores (client.go, go.mod, README) |
| `sdks/typescript-sdk/README.md` | README del TypeScript SDK |
| `CHANGELOG.md` | Changelog del proyecto |
| `docs/runbooks/LAUNCH_CHECKLIST.md` | Checklist de lanzamiento |

## Archivos a modificar

| Archivo | Cambio |
|---------|--------|
| `control-plane/internal/admin/handler.go` | Agregar suspend/reactivate/delete endpoints |
| `control-plane/internal/admin/usecases.go` | Lógica de tenant lifecycle |
| `control-plane/internal/admin/repository.go` | Queries para suspend/delete |
| `control-plane/wire/bootstrap_routes.go` | Registrar nuevas rutas admin |
| `tower/src/app/App.tsx` | Ruta 404, onboarding, suspended |
| `tower/src/components/ErrorBoundary.tsx` | Mejorar con report/support |
| `tower/src/pages/AdminPage.tsx` | Botones suspend/reactivate/delete tenant |

---

## Criterios de éxito

### Smoke test
1. [ ] `bash scripts/smoke/smoke_prod.sh` contra stack local pasa todos los checks
2. [ ] Script acepta URLs como parámetros (configurable para staging/prod)

### Tenant lifecycle
3. [ ] `PUT /v1/admin/tenants/:org_id/suspend` → tenant queda suspendido
4. [ ] Requests de un tenant suspendido retornan 403
5. [ ] `PUT /v1/admin/tenants/:org_id/reactivate` → tenant vuelve a active
6. [ ] `DELETE /v1/admin/tenants/:org_id` → soft-delete con deleted_at
7. [ ] Migration 0006 aplica y revierte sin errores

### Frontend
8. [ ] Navegar a `/ruta-inexistente` muestra 404 page (no redirect)
9. [ ] ErrorBoundary muestra botón "Report issue"
10. [ ] Onboarding wizard aparece para tenant con 0 tools

### Load testing
11. [ ] `k6 run scripts/loadtest/k6_gateway.js` ejecuta sin errores
12. [ ] Thresholds definidos: p95 < 500ms, error rate < 1%

### SDKs
13. [ ] `sdks/go-sdk/` tiene client funcional con RunTool y ListTools
14. [ ] `sdks/typescript-sdk/README.md` existe con quick start
15. [ ] `sdks/go-sdk/README.md` existe con quick start

### Docs
16. [ ] `CHANGELOG.md` documenta todos los features implementados
17. [ ] `docs/runbooks/LAUNCH_CHECKLIST.md` tiene checklist completo

### Build & tests
18. [ ] `cd data-plane && go build ./...` ✓
19. [ ] `cd control-plane && go build ./...` ✓
20. [ ] `cd tower && npm run build` ✓
21. [ ] `make e2e` pasa sin regresiones

---

## Orden de ejecución recomendado

**Aclaración importante**: este orden existe solo para respetar dependencias técnicas. Todo el contenido del prompt sigue siendo obligatorio.

1. Tenant lifecycle (migration + API + entitlements check)
2. Frontend error pages (404, suspended, ErrorBoundary)
3. Onboarding wizard
4. Smoke test script
5. Load test script
6. Go SDK
7. TypeScript SDK README
8. CHANGELOG + Launch checklist
9. Verificar todo compila y e2e pasan
