# Prompt 04 — Notificaciones por email e in-app con AWS SES

## Contexto del proyecto

Nexus es una plataforma SaaS (Go + React/TypeScript) compuesta por:

| Servicio | Stack | Puerto |
|----------|-------|--------|
| `nexus-core` | Go/Gin | 8080 |
| `nexus-saas` | Go/Gin | 8082 |
| `nexus-tower` | React/Vite | 5173 |

**Stack decidido**: AWS + Clerk (identity) + Stripe (billing) + **AWS SES** (email).

## Alcance obligatorio

Este prompt hereda los estándares de `docs/prompts/00_base_transversal.md`.

Todo lo definido acá es obligatorio para el subsistema de notificaciones:
- email transaccional
- preferencias por usuario/org
- integración con auth, billing, alerts y eventos
- senders, templates, wiring, frontend y criterios operativos

La secuencia de implementación es técnica; no reduce el alcance final.

## Prerequisito

Leer y respetar `docs/prompts/00_base_transversal.md` antes de ejecutar este prompt.

---

## Lo que YA existe (hooks de eventos para conectar)

### Clerk webhooks (control-plane/internal/clerkwebhook/)

| Evento | Handler actual |
|--------|---------------|
| `user.created` | `onUserUpsert` — sincroniza usuario a DB |
| `user.updated` | `onUserUpsert` — actualiza usuario |
| `organization.created` | `onOrganizationCreated` — sincroniza org |
| `organizationMembership.created` | `onOrganizationMembershipCreated` — agrega miembro |

### Stripe webhooks (control-plane/internal/billing/)

| Evento | Handler actual |
|--------|---------------|
| `checkout.session.completed` | Aplica suscripción activa |
| `customer.subscription.deleted` | Vuelve a plan starter |
| `invoice.payment_succeeded` | billing_status = active |
| `invoice.payment_failed` | billing_status = past_due |

### Event store (control-plane/internal/events/)

Eventos existentes en la tabla `events`:

| event_type | Origen |
|------------|--------|
| `incident.opened` | Incidents usecase |
| `incident.closed` | Incidents usecase |
| `action.applied` | Actions usecase |
| `action.rolled_back` | Actions usecase |
| `action.expired` | Actions usecase |

### Alerts (control-plane/internal/alerts/)

- Alert rules con `webhook_url` que disparan cuando se supera un umbral
- Métricas: `deny_rate`, `error_rate`, `rate_limited_count`
- `EvaluateAll()` existe pero NO hay cron/worker que lo invoque

### Usuarios en DB

Tabla `users` con `id`, `external_id`, `email`, `name`, `avatar_url`.
Tabla `org_members` con `org_id`, `user_id`, `role`.

---

## Qué implementar

### Fase 1 — Módulo de notificaciones (nexus-saas)

#### 1.1 Nuevo módulo `control-plane/internal/notifications/`

```
internal/notifications/
├── handler.go              # GET/PUT /v1/notifications/preferences
├── handler/dto/dto.go      # DTOs
├── usecases.go             # Lógica: envío, preferencias, throttle
├── repository.go           # Persistencia de preferencias
├── repository/models/      # GORM models
├── sender.go               # Interface + implementación SES
├── sender_smtp.go          # Implementación SMTP (dev/test)
├── templates.go            # Templates HTML de email
├── usecases/domain/
│   └── entities.go         # Tipos: NotificationType, Preference, etc.
└── usecases_test.go        # Tests
```

#### 1.2 Migración SQL

`control-plane/migrations/0005_notification_preferences.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS notification_preferences (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type text NOT NULL,
    channel text NOT NULL DEFAULT 'email',
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(user_id, notification_type, channel)
);

CREATE INDEX IF NOT EXISTS idx_notification_prefs_user
    ON notification_preferences(user_id);

CREATE TABLE IF NOT EXISTS notification_log (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    notification_type text NOT NULL,
    channel text NOT NULL DEFAULT 'email',
    recipient text NOT NULL,
    subject text NOT NULL,
    status text NOT NULL DEFAULT 'sent',
    error_message text NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notification_log_org_created
    ON notification_log(org_id, created_at DESC);
```

#### 1.3 Tipos de notificación

| Tipo | Trigger | Destinatario | Asunto |
|------|---------|-------------|--------|
| `welcome` | Clerk `user.created` | El nuevo usuario | "Welcome to Nexus" |
| `plan_upgraded` | Stripe `checkout.session.completed` | Admin(s) de la org | "Your plan has been upgraded to {plan}" |
| `payment_failed` | Stripe `invoice.payment_failed` | Admin(s) de la org | "Action required: payment failed" |
| `subscription_canceled` | Stripe `customer.subscription.deleted` | Admin(s) de la org | "Your subscription has been canceled" |
| `incident_opened` | Event `incident.opened` | Todos los miembros de la org | "Incident: {title}" |
| `incident_closed` | Event `incident.closed` | Todos los miembros de la org | "Incident resolved: {title}" |

#### 1.4 Interface del sender

```go
type EmailSender interface {
    Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}
```

Dos implementaciones:
- `SESSender` — usa AWS SES SDK v2 (`github.com/aws/aws-sdk-go-v2/service/ses`)
- `SMTPSender` — usa `net/smtp` estándar (para dev local con MailHog o similar)
- `NoopSender` — no envía nada, solo logea (default cuando no hay config)

Selección por env var `NOTIFICATION_BACKEND`:
- `ses` → `SESSender`
- `smtp` → `SMTPSender`
- vacío o `noop` → `NoopSender` (graceful degradation)

#### 1.5 Usecases

```go
type Usecases struct {
    repo   *Repository
    sender EmailSender
    logger zerolog.Logger
}

func (u *Usecases) Notify(ctx context.Context, orgID uuid.UUID, notifType NotificationType, data TemplateData) error
func (u *Usecases) NotifyUser(ctx context.Context, userID uuid.UUID, notifType NotificationType, data TemplateData) error
func (u *Usecases) GetPreferences(ctx context.Context, userID uuid.UUID) ([]Preference, error)
func (u *Usecases) UpdatePreference(ctx context.Context, userID uuid.UUID, notifType string, enabled bool) error
```

`Notify(orgID, ...)`:
1. Busca miembros de la org con role=admin (o todos para incidents)
2. Para cada miembro, verifica si tiene la preferencia habilitada
3. Renderiza template con datos
4. Envía email via `EmailSender`
5. Registra en `notification_log`

`NotifyUser(userID, ...)`:
1. Busca email del usuario
2. Verifica preferencia
3. Envía y registra

#### 1.6 Templates HTML

Templates embebidos en Go con `html/template`. Diseño minimalista:

```
┌─────────────────────────────────────────────┐
│                                               │
│    N E X U S                                  │
│                                               │
│    {Título del email}                         │
│                                               │
│    {Cuerpo del mensaje}                       │
│                                               │
│    {Call to action button}                    │
│                                               │
│    ─────────────────────────────────────────  │
│    Nexus · You're receiving this because      │
│    you're a member of {org_name}.             │
│    Manage preferences → {url}                 │
│                                               │
└─────────────────────────────────────────────┘
```

Cada template recibe un `TemplateData`:

```go
type TemplateData struct {
    RecipientName string
    OrgName       string
    PlanCode      string
    IncidentTitle string
    IncidentSeverity string
    ActionURL     string
    PreferencesURL string
    Extra         map[string]string
}
```

#### 1.7 Endpoints HTTP

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/v1/notifications/preferences` | Lista preferencias del usuario autenticado |
| `PUT` | `/v1/notifications/preferences` | Actualiza preferencias (array de {type, enabled}) |

Auth: JWT (Clerk) o API key.

#### 1.8 Conectar con hooks existentes

**En clerkwebhook/handler.go** — después de `onUserUpsert` para `user.created`:

```go
if evt.Type == "user.created" {
    go u.notifications.NotifyUser(ctx, userDBID, "welcome", data)
}
```

**En billing/usecases.go** — al final de cada webhook handler:

```go
// en handleCheckoutCompleted, después de applySubscriptionState:
go u.notifications.Notify(ctx, orgID, "plan_upgraded", data)

// en handleSubscriptionDeleted:
go u.notifications.Notify(ctx, orgID, "subscription_canceled", data)

// en handleInvoicePayment con status past_due:
go u.notifications.Notify(ctx, orgID, "payment_failed", data)
```

**Nuevo: Event consumer para incidents** — agregar un hook en incidents/usecases.go:

```go
// en Create, después de events.Append:
go u.notifications.Notify(ctx, orgID, "incident_opened", data)

// en Close:
go u.notifications.Notify(ctx, orgID, "incident_closed", data)
```

**Importante**: usar `go` (goroutine) para no bloquear el request principal. Errores de envío se logean pero no fallan el request.

---

### Fase 2 — NotificationPort (desacoplamiento)

Para evitar dependencias circulares, definir un port:

```go
// En notifications/usecases.go
type NotificationPort interface {
    Notify(ctx context.Context, orgID uuid.UUID, notifType string, data map[string]string) error
    NotifyUser(ctx context.Context, userExternalID string, notifType string, data map[string]string) error
}
```

Los módulos que envían notificaciones (billing, clerkwebhook, incidents) reciben `NotificationPort` como dependencia en su constructor. Si el port es nil (notifications no configurado), no envían nada.

---

### Fase 3 — Frontend (nexus-tower)

#### 3.1 Página de preferencias de notificaciones

Ruta: `/settings/notifications`

```
┌────────────────────────────────────────────────────────┐
│ Notification Preferences                                │
│ Choose which emails you want to receive.                │
├────────────────────────────────────────────────────────┤
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │ Welcome Email                        [✓ enabled]   │ │
│  │ Sent when you first join Nexus                     │ │
│  ├────────────────────────────────────────────────────┤ │
│  │ Plan Upgraded                        [✓ enabled]   │ │
│  │ Sent when your organization's plan changes         │ │
│  ├────────────────────────────────────────────────────┤ │
│  │ Payment Failed                       [✓ enabled]   │ │
│  │ Sent when a payment attempt fails                  │ │
│  ├────────────────────────────────────────────────────┤ │
│  │ Subscription Canceled                [✓ enabled]   │ │
│  │ Sent when your subscription is canceled            │ │
│  ├────────────────────────────────────────────────────┤ │
│  │ Incident Opened                      [✓ enabled]   │ │
│  │ Sent when a new incident is detected               │ │
│  ├────────────────────────────────────────────────────┤ │
│  │ Incident Resolved                    [✓ enabled]   │ │
│  │ Sent when an incident is closed                    │ │
│  └────────────────────────────────────────────────────┘ │
│                                                          │
│                                      [Save Preferences]  │
│                                                          │
└────────────────────────────────────────────────────────┘
```

#### 3.2 Tipos y API client

Nuevos tipos en `types.ts`:

```typescript
export type NotificationPreference = {
  notification_type: string;
  channel: string;
  enabled: boolean;
};
```

Nuevas funciones en `api.ts`:

```typescript
export async function getNotificationPreferences(): Promise<{ items: NotificationPreference[] }>
export async function updateNotificationPreferences(items: Array<{ notification_type: string; enabled: boolean }>): Promise<void>
```

#### 3.3 Navegación

Agregar a `Shell.tsx` entre "Profile" (el último item):

```typescript
{ to: '/settings/notifications', label: 'Notifications' },
```

Agregar ruta en `App.tsx`:

```tsx
<Route path="/settings/notifications" element={<NotificationPreferencesPage />} />
```

---

### Fase 4 — Config y wiring

#### 4.1 Variables de entorno

```env
# ── Notifications ──
NOTIFICATION_BACKEND=noop
# SES config (when NOTIFICATION_BACKEND=ses)
AWS_REGION=us-east-1
AWS_SES_FROM_EMAIL=noreply@nexus.example.com
AWS_SES_FROM_NAME=Nexus
# SMTP config (when NOTIFICATION_BACKEND=smtp)
SMTP_HOST=localhost
SMTP_PORT=1025
SMTP_FROM_EMAIL=noreply@nexus.local
SMTP_USERNAME=
SMTP_PASSWORD=
```

#### 4.2 Config (control-plane/cmd/config/service.go)

```go
// Notifications
NotificationBackend string
AWSRegion           string
SESFromEmail        string
SESFromName         string
SMTPHost            string
SMTPPort            int
SMTPFromEmail       string
SMTPUsername         string
SMTPPassword        string
```

#### 4.3 Wire

- `NotificationsSet` en `wire/notification_providers.go`
- Inyectar `NotificationPort` en billing usecases, clerkwebhook handler, incidents usecases
- Registrar handler HTTP en `bootstrap_routes.go`

#### 4.4 Docker Compose (dev)

Agregar MailHog para dev local:

```yaml
  mailhog:
    image: mailhog/mailhog:v1.0.1
    ports:
      - "1025:1025"   # SMTP
      - "8025:8025"   # Web UI
    healthcheck:
      test: ["CMD", "nc", "-z", "localhost", "1025"]
      interval: 5s
      timeout: 3s
      retries: 3
```

Con `NOTIFICATION_BACKEND=smtp`, `SMTP_HOST=mailhog`, `SMTP_PORT=1025`.

---

## Reglas de implementación

1. **Graceful degradation**: si `NOTIFICATION_BACKEND` está vacío o es `noop`, el sistema funciona sin enviar emails. Logs un mensaje info.
2. **No bloquear requests**: todas las notificaciones se envían en goroutines. Errores de envío se logean (zerolog) pero no fallan el request HTTP.
3. **Preferencias por defecto**: todos los tipos habilitados por defecto. El usuario puede desactivar individualmente.
4. **Idempotencia**: el `notification_log` previene envíos duplicados si se usa un dedup key (notification_type + user_id + referenceID + hour).
5. **Templates embebidos**: usar `embed` de Go para embeber templates HTML. No cargar de disco.
6. **Port pattern**: los módulos que envían notificaciones reciben `NotificationPort` (interface). Si es nil, no envían.
7. **Tests**: mockear `EmailSender` en tests unitarios. Verificar que los templates se renderizan correctamente.
8. **No tocar lógica existente** de los módulos que envían notificaciones, solo agregar el hook al final del flujo.

---

## Criterios de éxito

- [ ] `GET /v1/notifications/preferences` devuelve preferencias del usuario
- [ ] `PUT /v1/notifications/preferences` actualiza preferencias
- [ ] Welcome email se envía al crear usuario (Clerk webhook `user.created`)
- [ ] Plan upgraded email se envía al completar checkout (Stripe webhook)
- [ ] Payment failed email se envía al fallar pago (Stripe webhook)
- [ ] Subscription canceled email se envía al cancelar (Stripe webhook)
- [ ] Incident opened email se envía a miembros de la org
- [ ] Incident closed email se envía a miembros de la org
- [ ] Preferencias respetadas: si un tipo está deshabilitado, no se envía
- [ ] `notification_log` registra cada envío con status
- [ ] NoopSender funciona cuando no hay config de email
- [ ] SMTPSender funciona con MailHog en docker-compose dev
- [ ] SESSender compila y usa AWS SES SDK v2
- [ ] Templates HTML se renderizan correctamente (link a preferencias en footer)
- [ ] Frontend: NotificationPreferencesPage con toggles por tipo
- [ ] Frontend: ruta `/settings/notifications` agregada
- [ ] Sin dependencias nuevas de npm (frontend)
- [ ] Tests unitarios para usecases y templates pasan
- [ ] `go test ./...` en nexus-saas pasa
- [ ] Tests e2e existentes (01-07) siguen pasando

---

## Dependencias Go a agregar

```
github.com/aws/aws-sdk-go-v2/config
github.com/aws/aws-sdk-go-v2/service/ses
```

Solo se importan si `NOTIFICATION_BACKEND=ses`.

---

## Orden de ejecución recomendado

**Aclaración importante**: este orden existe solo para respetar dependencias técnicas. Todo el contenido del prompt sigue siendo obligatorio.

1. Migración SQL (`0005_notification_preferences`)
2. Domain entities y tipos
3. `sender.go` (interface + NoopSender)
4. `sender_smtp.go` (SMTPSender)
5. `templates.go` (HTML templates embebidos)
6. `repository.go` (preferencias + notification_log)
7. `usecases.go` (Notify, NotifyUser, GetPreferences, UpdatePreference)
8. `handler.go` (GET/PUT preferences)
9. Wire: providers + routes
10. Config: env vars + docker-compose (MailHog)
11. Conectar hooks: clerkwebhook, billing, incidents
12. Frontend: tipos, API client, NotificationPreferencesPage
13. Tests unitarios
14. Verificar e2e existentes
