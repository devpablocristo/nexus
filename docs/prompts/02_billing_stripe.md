# Prompt 02 — Billing & Payments con Stripe

## Contexto del proyecto

Nexus es una plataforma SaaS (Go + React/TypeScript) compuesta por:

| Servicio | Stack | Puerto | Descripción |
|----------|-------|--------|-------------|
| `nexus-core` | Go/Gin | 8080 | Gateway pipeline, auth, tools, policies, egress, audit |
| `nexus-saas` | Go/Gin | 8081 | Eventos, incidentes, acciones, admin, contratos, users |
| `nexus-tower` | React/Vite | 5173 | SPA frontend (Clerk auth) |
| `nexus-control-operators` | Go | 8082 | Agentes deterministas (sentry, coordinator, mitigation, recovery) |
| `nexus-ai-operators` | Python/FastAPI | 8090 | Agentes IA (observer, analyst, executor, assistant) |

**Stack decidido**: AWS + Clerk (identity) + **Stripe** (billing).

---

## Lo que YA existe (no reescribir, conectar)

### 1. Planes en DB

```sql
-- nexus-saas/migrations/0001_saas_core_tables.up.sql
CREATE TABLE tenant_settings (
    org_id uuid PRIMARY KEY REFERENCES orgs(id),
    plan_code text NOT NULL,              -- 'starter' | 'growth' | 'enterprise'
    hard_limits_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_by text NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now()
);
```

### 2. Hard limits por plan (nexus-saas/internal/admin/usecases.go)

| Plan | tools_max | run_rpm | audit_retention_days |
|------|-----------|---------|---------------------|
| starter | 20 | 300 | 30 |
| growth | 75 | 1,200 | 90 |
| enterprise | 250 | 5,000 | 365 |

### 3. Usage metering (4 contadores)

| Counter | Se incrementa en |
|---------|-------------------|
| `api_calls` | Middleware en core y saas |
| `events_ingested` | `events/usecases.go` (Append) |
| `incidents_opened` | `incidents/usecases.go` (Create) |
| `actions_executed` | `actions/usecases.go` (Execute) |

Tablas: `org_usage_counters` (org_id, period, counter, value) con dedup en `saas_usage_event_dedup`.

### 4. Entitlements API (interno, M2M)

```
GET /internal/entitlements/:org_id
→ { org_id, plan_code, hard_limits }

Auth: header X-NEXUS-SAAS-KEY
```

nexus-core consume esto para aplicar `run_rpm` rate-limit por tenant.

### 5. Admin API (nexus-saas, /v1/admin/*)

- `GET /v1/admin/tenant-settings` — lee plan + limits del org actual
- `PUT /v1/admin/tenant-settings` — actualiza plan + limits
- `GET /v1/admin/bootstrap` — datos de bootstrapping
- `GET /v1/admin/activity` — activity log

### 6. Frontend actual (nexus-tower)

- Clerk auth (login, signup, orgs, UserButton)
- Tools CRUD, Audit Log, Monitoring, Secrets, Policies, Incidents, Events, Assistant, API Keys
- **NO existe**: página de billing, usage dashboard, upgrade flow, ni pricing

---

## Qué implementar

### Fase 1 — Stripe backend (nexus-saas)

#### 1.1 Nuevas columnas en `tenant_settings`

Migración `nexus-saas/migrations/0004_stripe_billing.up.sql`:

```sql
ALTER TABLE tenant_settings
  ADD COLUMN IF NOT EXISTS stripe_customer_id text UNIQUE,
  ADD COLUMN IF NOT EXISTS stripe_subscription_id text UNIQUE,
  ADD COLUMN IF NOT EXISTS billing_status text NOT NULL DEFAULT 'trialing'
    CHECK (billing_status IN ('trialing','active','past_due','canceled','unpaid'));

CREATE INDEX IF NOT EXISTS idx_tenant_settings_stripe_customer
  ON tenant_settings(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;
```

#### 1.2 Nuevo módulo `nexus-saas/internal/billing/`

Estructura hexagonal (igual que los otros módulos):

```
internal/billing/
├── handler.go              # Endpoints HTTP
├── handler/dto/dto.go      # Request/Response DTOs
├── usecases.go             # Lógica de negocio
├── repository.go           # Persistencia
├── repository/models/      # GORM models
├── stripe_client.go        # Wrapper del SDK de Stripe
├── webhook_handler.go      # POST /v1/webhooks/stripe
└── usecases/domain/
    └── entities.go         # Domain types
```

#### 1.3 Stripe Products & Prices (crear via Stripe Dashboard o API)

| Product | Price ID (env var) | Tipo | Precio sugerido |
|---------|-------------------|------|-----------------|
| Nexus Starter | `STRIPE_PRICE_STARTER` | Flat monthly | $0 (free tier) |
| Nexus Growth | `STRIPE_PRICE_GROWTH` | Flat monthly | $99/mo |
| Nexus Enterprise | `STRIPE_PRICE_ENTERPRISE` | Flat monthly | $499/mo |

#### 1.4 Endpoints de billing (auth: JWT Clerk, org_id del contexto)

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/v1/billing/status` | Plan actual, billing_status, current_period_end, usage del periodo |
| `POST` | `/v1/billing/checkout` | Crea Stripe Checkout Session → devuelve `{ url }` |
| `POST` | `/v1/billing/portal` | Crea Stripe Customer Portal Session → devuelve `{ url }` |
| `GET` | `/v1/billing/usage` | Usage counters del periodo actual (api_calls, events, incidents, actions) |
| `POST` | `/v1/webhooks/stripe` | Webhook de Stripe (sin auth JWT, verificar signature) |

#### 1.5 Flujo de checkout

```
Tower (frontend)                    nexus-saas                     Stripe
       │                                │                             │
       ├── POST /v1/billing/checkout ──→│                             │
       │   { price_id: "growth" }       │                             │
       │                                ├── stripe.CheckoutSession ──→│
       │                                │   { customer, price,        │
       │                                │     success_url, cancel_url,│
       │                                │     metadata: {org_id} }    │
       │   ←── { url } ────────────────┤                             │
       │                                │                             │
       ├── window.location = url ──────────────────────────────────→│
       │                                │                             │
       │   (user paga en Stripe)        │                             │
       │                                │                             │
       │                                │←── webhook ─────────────────┤
       │                                │   checkout.session.completed │
       │                                │                             │
       │                                ├── UPDATE tenant_settings    │
       │                                │   SET plan_code = 'growth', │
       │                                │   stripe_customer_id = ..., │
       │                                │   stripe_subscription_id=.. │
       │                                │   billing_status = 'active' │
       │                                │                             │
       ├── redirect → /billing?ok ─────────────────────────────────←│
```

#### 1.6 Webhook handler de Stripe

Eventos a manejar:

| Evento | Acción |
|--------|--------|
| `checkout.session.completed` | Crear/actualizar customer, subscription, plan_code, billing_status='active' |
| `customer.subscription.updated` | Actualizar plan_code si cambió de price, actualizar billing_status |
| `customer.subscription.deleted` | billing_status='canceled', plan_code='starter', resetear hard_limits |
| `invoice.payment_succeeded` | billing_status='active' |
| `invoice.payment_failed` | billing_status='past_due' |

Verificación de firma: usar `stripe.ConstructEvent(payload, sigHeader, webhookSecret)`.

#### 1.7 Lógica de negocio (usecases.go)

```go
func (u *Usecases) GetBillingStatus(ctx context.Context, orgID uuid.UUID) (BillingStatus, error)
func (u *Usecases) CreateCheckoutSession(ctx context.Context, orgID uuid.UUID, planCode string, successURL, cancelURL string) (string, error)
func (u *Usecases) CreatePortalSession(ctx context.Context, orgID uuid.UUID, returnURL string) (string, error)
func (u *Usecases) GetUsageSummary(ctx context.Context, orgID uuid.UUID) (UsageSummary, error)
func (u *Usecases) HandleWebhookEvent(ctx context.Context, event stripe.Event) error
```

#### 1.8 stripe_client.go

Wrapper ligero sobre el Stripe Go SDK (`github.com/stripe/stripe-go/v81`):

```go
type StripeClient struct {
    secretKey string
}

func (c *StripeClient) CreateCheckoutSession(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
func (c *StripeClient) CreatePortalSession(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error)
func (c *StripeClient) ConstructWebhookEvent(payload []byte, sigHeader, secret string) (stripe.Event, error)
```

---

### Fase 2 — Frontend billing (nexus-tower)

#### 2.1 Nuevas páginas

| Ruta | Componente | Descripción |
|------|-----------|-------------|
| `/billing` | `BillingPage.tsx` | Plan actual, status, usage, botones upgrade/manage |
| `/billing/success` | Redirect a `/billing` con toast "Plan upgraded" |

#### 2.2 BillingPage.tsx — Secciones

```
┌─────────────────────────────────────────────────┐
│ Billing                                          │
├─────────────────────────────────────────────────┤
│                                                   │
│  Current Plan: Growth          Status: Active     │
│  Next billing: March 15, 2026                     │
│                                                   │
│  [Manage Subscription]  [Upgrade Plan]            │
│                                                   │
├─────────────────────────────────────────────────┤
│  Usage This Period (Feb 2026)                     │
│                                                   │
│  API Calls      12,450 / ∞                       │
│  ████████████░░░░░░░░                             │
│                                                   │
│  Events          3,200 / ∞                       │
│  ██████░░░░░░░░░░░░░░                             │
│                                                   │
│  Incidents          18 / ∞                       │
│  █░░░░░░░░░░░░░░░░░░░                             │
│                                                   │
│  Actions            45 / ∞                       │
│  ██░░░░░░░░░░░░░░░░░░                             │
│                                                   │
├─────────────────────────────────────────────────┤
│  Plan Limits                                      │
│                                                   │
│  Tools: 75          Rate: 1,200 rpm               │
│  Audit retention: 90 days                         │
│                                                   │
├─────────────────────────────────────────────────┤
│  Plans                                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────────┐   │
│  │ Starter  │  │ Growth   │  │ Enterprise   │   │
│  │ Free     │  │ $99/mo   │  │ $499/mo      │   │
│  │ 20 tools │  │ 75 tools │  │ 250 tools    │   │
│  │ 300 rpm  │  │ 1200 rpm │  │ 5000 rpm     │   │
│  │ 30d audit│  │ 90d audit│  │ 365d audit   │   │
│  │          │  │ ✓ Current│  │ [Upgrade]    │   │
│  └──────────┘  └──────────┘  └──────────────┘   │
│                                                   │
└─────────────────────────────────────────────────┘
```

#### 2.3 API client (nexus-tower/src/lib/api.ts)

```typescript
export async function getBillingStatus(): Promise<BillingStatus>
export async function createCheckoutSession(planCode: string): Promise<{ url: string }>
export async function createPortalSession(): Promise<{ url: string }>
export async function getUsageSummary(): Promise<UsageSummary>
```

#### 2.4 Tipos (nexus-tower/src/lib/types.ts)

```typescript
interface BillingStatus {
  plan_code: 'starter' | 'growth' | 'enterprise';
  billing_status: 'trialing' | 'active' | 'past_due' | 'canceled' | 'unpaid';
  current_period_end?: string;
  hard_limits: {
    tools_max: number;
    run_rpm: number;
    audit_retention_days: number;
  };
}

interface UsageSummary {
  period: string;
  counters: {
    api_calls: number;
    events_ingested: number;
    incidents_opened: number;
    actions_executed: number;
  };
}
```

#### 2.5 Navegación

Agregar a `Shell.tsx` entre "API Keys" y "Organizations":

```typescript
{ to: '/billing', label: 'Billing' },
```

---

### Fase 3 — Config y wiring

#### 3.1 Variables de entorno (agregar a .env.example)

```env
# ── Stripe ──
STRIPE_SECRET_KEY=sk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...
STRIPE_PRICE_STARTER=price_xxx_starter
STRIPE_PRICE_GROWTH=price_xxx_growth
STRIPE_PRICE_ENTERPRISE=price_xxx_enterprise

# URLs para Stripe redirect
TOWER_BASE_URL=http://localhost:5173
```

#### 3.2 Config (nexus-saas/cmd/config/service.go)

Agregar campos:

```go
StripeSecretKey     string
StripeWebhookSecret string
StripePriceStarter  string
StripePriceGrowth   string
StripePriceEnterprise string
TowerBaseURL        string
```

#### 3.3 Wire (nexus-saas/wire/)

- Crear `billing_providers.go` con `BillingSet`
- Registrar ruta en `bootstrap_routes.go`:
  - `billingHandler.Register(v1)` → endpoints con auth
  - `billingHandler.RegisterWebhook(engine)` → webhook sin auth JWT

#### 3.4 Docker Compose

Pasar a `nexus-saas`:

```yaml
environment:
  STRIPE_SECRET_KEY: ${STRIPE_SECRET_KEY:-}
  STRIPE_WEBHOOK_SECRET: ${STRIPE_WEBHOOK_SECRET:-}
  STRIPE_PRICE_STARTER: ${STRIPE_PRICE_STARTER:-}
  STRIPE_PRICE_GROWTH: ${STRIPE_PRICE_GROWTH:-}
  STRIPE_PRICE_ENTERPRISE: ${STRIPE_PRICE_ENTERPRISE:-}
  TOWER_BASE_URL: ${TOWER_BASE_URL:-http://nexus-tower:5173}
```

---

### Fase 4 — Sincronización plan → hard_limits

Cuando Stripe cambia el plan (vía webhook), actualizar automáticamente `hard_limits_json` en `tenant_settings` usando `defaultHardLimits(planCode)` que ya existe en `admin/usecases.go`.

Flujo:

```
Stripe webhook → billing.HandleWebhookEvent
  → admin.UpsertTenantSettings(orgID, newPlanCode, defaultHardLimits(newPlanCode))
```

nexus-core ya consume los limits vía `GET /internal/entitlements/:org_id`, así que los cambios se propagan automáticamente al gateway rate-limiter.

---

## Reglas de implementación

1. **No reescribir** lo que ya existe — conectar. El usage metering, entitlements, admin API y hard limits ya funcionan.
2. **Stripe Test Mode** para desarrollo. Las Price IDs se configuran por env var.
3. **Webhook verification** obligatoria con `stripe.ConstructWebhookEvent`.
4. **Idempotencia** en webhooks: verificar si el `stripe_subscription_id` ya está actualizado antes de escribir.
5. **Graceful degradation**: si `STRIPE_SECRET_KEY` está vacío, billing endpoints devuelven 503. El sistema sigue funcionando sin Stripe (free tier perpetuo).
6. **No mover lógica existente** de admin o metering. Solo consumirla desde el nuevo módulo billing.
7. **Tests**: unitarios para usecases y webhook handler. Mockear Stripe SDK.
8. **Un solo módulo nuevo**: `internal/billing/`. No tocar la estructura existente de admin, contracts ni usagemetering excepto para imports.

---

## Criterios de éxito

- [ ] `GET /v1/billing/status` devuelve plan, status y usage del org autenticado
- [ ] `POST /v1/billing/checkout` crea Stripe Checkout Session y devuelve URL
- [ ] `POST /v1/billing/portal` crea Customer Portal Session y devuelve URL
- [ ] `GET /v1/billing/usage` devuelve los 4 contadores del periodo actual
- [ ] `POST /v1/webhooks/stripe` procesa los 5 eventos listados
- [ ] Webhook actualiza `plan_code`, `billing_status` y `hard_limits_json` automáticamente
- [ ] Cambio de plan se propaga a nexus-core vía entitlements API (sin cambios en core)
- [ ] BillingPage en Tower muestra plan, usage y permite upgrade/manage
- [ ] Checkout redirect funciona: Tower → Stripe → Tower/billing?ok
- [ ] Portal redirect funciona: Tower → Stripe Portal → Tower
- [ ] Sin Stripe keys: billing endpoints 503, resto del sistema funciona normal
- [ ] Tests unitarios para billing usecases y webhook handler pasan
- [ ] Tests e2e existentes (01-07) siguen pasando sin cambios
- [ ] `go test ./...` en nexus-saas pasa

---

## Dependencias Go a agregar

```
github.com/stripe/stripe-go/v81
```

## Dependencias npm (ninguna nueva)

El frontend solo usa `fetch` + redirects a Stripe-hosted pages.

---

## Orden de ejecución recomendado

1. Migración SQL (`0004_stripe_billing`)
2. `stripe_client.go` (wrapper SDK)
3. `repository.go` (CRUD tenant_settings con campos Stripe)
4. `usecases.go` (lógica checkout, portal, webhook)
5. `webhook_handler.go` (POST /v1/webhooks/stripe)
6. `handler.go` (endpoints billing)
7. Wire: providers + routes
8. Config: env vars + docker-compose
9. Frontend: tipos, API client, BillingPage
10. Tests unitarios
11. Verificar e2e existentes
