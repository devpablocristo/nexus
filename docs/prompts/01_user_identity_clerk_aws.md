# Prompt 01 — Identidad de usuario con Clerk + AWS para Nexus SaaS

## Contexto del proyecto

Nexus es un API gateway inteligente para herramientas de agentes IA.
Monorepo con go.work. Docker Compose para dev local.

Servicios:
- **nexus-core** (Go/Gin, :8080) — Gateway: auth, DLP, policies, rate limits, audit
- **nexus-saas** (Go/Gin, :8082) — SaaS: orgs, events, incidents, usage metering
- **nexus-control-operators** (Go, :8090) — Operadores deterministas
- **nexus-ai-operators** (Python/FastAPI, :8000) — Operadores IA
- **nexus-tower** (React/TypeScript/Vite, :5173) — Frontend SPA
- PostgreSQL x2, Redis, Prometheus, Grafana

## Alcance obligatorio

Este prompt hereda los estándares de `docs/prompts/00_base_transversal.md`.

Todo lo definido acá es parte del alcance requerido para identidad y acceso en Nexus:
- Clerk en frontend
- JWT/JWKS/OIDC
- sync de usuarios y membresías
- boundaries correctos entre `nexus-core`, `nexus-saas` y `nexus-tower`
- validación, seguridad, observabilidad y testing del flujo

El orden propuesto de implementación es solo técnico. No reduce el alcance final.

## Prerequisito

Leer y respetar `docs/prompts/00_base_transversal.md` antes de ejecutar este prompt.

## Stack decidido

```
Identity         → Clerk (free tier, 10k MAU)
Compute          → AWS ECS Fargate (futuro, hoy Docker Compose)
Database         → AWS RDS PostgreSQL (futuro, hoy Docker local)
Cache            → AWS ElastiCache Redis (futuro, hoy Docker local)
Email            → AWS SES (futuro, hoy Clerk maneja emails de auth)
Storage          → AWS S3
CDN              → AWS CloudFront + Route53
Secrets          → AWS Secrets Manager
Logs             → AWS CloudWatch
Registry         → AWS ECR
CI/CD            → GitHub Actions
```

## Lo que YA existe en auth (NO tocar, NO reinventar)

1. **JWT verification** en nexus-core (`internal/identity/executor/jwks/verifier.go`):
   - Valida RS256/384/512 via JWKS
   - Extrae claims: org_id, actor, role, scopes
   - Config: NEXUS_JWKS_URL, NEXUS_JWT_ISSUER, NEXUS_JWT_AUDIENCE
   - Clerk provee JWKS en: `https://{tu-dominio}.clerk.accounts.dev/.well-known/jwks.json`

2. **OIDC flow** en nexus-core: authorization_code + PKCE
   - /v1/auth/oidc/config, /v1/auth/oidc/authorize, /v1/auth/oidc/callback

3. **API key auth**: X-NEXUS-CORE-KEY header → sigue funcionando para M2M

4. **Auth middleware** en nexus-core (`internal/shared/handlers/auth_middleware.go`):
   - Extrae AuthContext (org_id, actor, role, scopes)
   - Soporta JWT Y API key simultáneamente

5. **Roles**: "admin" y "secops" con scopes granulares

6. **Auth/Auth bridge** en nexus-tower: existe base de autenticación y debe mantenerse alineada con Clerk, `ProtectedRoute` y el bridge de tokens hacia APIs

7. **Org creation** en nexus-core: POST /v1/orgs (crea org + API key)

## FASE 1: Clerk + nexus-tower (Frontend auth)

### 1.1 Configurar Clerk

Crear cuenta en clerk.com. Configurar:
- Application name: "Nexus"
- Sign-in methods: Email + Password, Google OAuth
- Organizations: Enable
- Custom JWT claims template (en Clerk Dashboard → JWT Templates):

```json
{
  "org_id": "{{org.id}}",
  "org_slug": "{{org.slug}}",
  "org_role": "{{org.role}}",
  "scopes": "tools:read,tools:write,policy:read,policy:write,egress:read,egress:write,audit:read,gateway:run,gateway:simulate,admin:secrets,admin:console:read,admin:console:write"
}
```

Guardar las keys:
- NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY=pk_test_...
- CLERK_SECRET_KEY=sk_test_...
- CLERK_WEBHOOK_SECRET=whsec_...

### 1.2 Integrar Clerk en nexus-tower

Instalar:
```bash
cd tower
npm install @clerk/clerk-react
```

Archivos a crear/modificar:

**src/main.tsx** — Wrappear con ClerkProvider:
```tsx
import { ClerkProvider } from '@clerk/clerk-react'

const CLERK_KEY = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY

<ClerkProvider publishableKey={CLERK_KEY}>
  <App />
</ClerkProvider>
```

**src/components/ProtectedRoute.tsx** — Proteger rutas:
```tsx
import { useAuth, RedirectToSignIn } from '@clerk/clerk-react'

export function ProtectedRoute({ children }) {
  const { isSignedIn, isLoaded } = useAuth()
  if (!isLoaded) return <LoadingSpinner />
  if (!isSignedIn) return <RedirectToSignIn />
  return children
}
```

**src/pages/LoginPage.tsx**:
```tsx
import { SignIn } from '@clerk/clerk-react'
export default function LoginPage() {
  return <SignIn routing="path" path="/login" />
}
```

**src/pages/SignupPage.tsx**:
```tsx
import { SignUp } from '@clerk/clerk-react'
export default function SignupPage() {
  return <SignUp routing="path" path="/signup" />
}
```

**src/api/client.ts** — Adjuntar JWT a todas las requests:
```tsx
import { useAuth } from '@clerk/clerk-react'

// El token JWT de Clerk se envía como Authorization: Bearer
// nexus-core ya sabe validar JWT via JWKS
const { getToken } = useAuth()
const token = await getToken()

fetch(url, {
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  }
})
```

Páginas nuevas:
```
/login              → <SignIn />
/signup             → <SignUp />
/settings           → <UserProfile /> (Clerk componente)
/settings/org       → <OrganizationProfile /> (Clerk componente)
/org-selector       → <OrganizationSwitcher /> (Clerk componente)
```

Todas las rutas existentes (/, /tools, /audit, /monitoring) → wrappear con ProtectedRoute.

### 1.3 Conectar nexus-core con Clerk JWKS

En .env agregar:
```
NEXUS_AUTH_ENABLE_JWT=true
NEXUS_JWKS_URL=https://{tu-clerk-domain}.clerk.accounts.dev/.well-known/jwks.json
NEXUS_JWT_ISSUER=https://{tu-clerk-domain}.clerk.accounts.dev
NEXUS_JWT_AUDIENCE=
NEXUS_AUTH_ALLOW_API_KEY=true
```

nexus-core ya valida JWT via JWKS. Solo hay que apuntar la URL al JWKS de Clerk.
API key auth sigue funcionando para M2M (scripts, operators, etc.).

## FASE 2: Webhook + nexus-saas (Backend sync)

### 2.1 Migración en nexus-saas

Crear `control-plane/migrations/0003_users_and_members.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id TEXT NOT NULL UNIQUE,
    email       TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL DEFAULT '',
    avatar_url  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS org_members (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id    UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      TEXT NOT NULL DEFAULT 'secops' CHECK (role IN ('admin', 'secops')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(org_id, user_id)
);

CREATE INDEX idx_org_members_org_id ON org_members(org_id);
CREATE INDEX idx_org_members_user_id ON org_members(user_id);
CREATE INDEX idx_users_external_id ON users(external_id);
```

Down migration:
```sql
DROP TABLE IF EXISTS org_members;
DROP TABLE IF EXISTS users;
```

### 2.2 Webhook endpoint en nexus-saas

Crear `control-plane/internal/clerkwebhook/handler.go`:

Clerk envía webhooks cuando:
- `user.created` → crear user en DB
- `organization.created` → crear org en DB
- `organizationMembership.created` → crear org_member en DB

Endpoint: `POST /v1/webhooks/clerk`
- Verificar firma del webhook (Clerk usa Svix, header `svix-signature`)
- Idempotente (si user ya existe, ignorar)

Dependencia Go: `github.com/svix/svix-webhooks/go` para verificar firmas.

### 2.3 Endpoints de usuario en nexus-saas

```
GET  /v1/users/me                         → perfil del usuario (desde JWT claims)
GET  /v1/orgs/:org_id/members             → listar miembros (auth: admin o secops)
GET  /v1/orgs/:org_id/api-keys            → listar API keys de la org
POST /v1/orgs/:org_id/api-keys            → crear nueva API key
DELETE /v1/orgs/:org_id/api-keys/:id      → revocar API key
POST /v1/orgs/:org_id/api-keys/:id/rotate → rotar API key
```

Las invitaciones y roles se gestionan 100% via Clerk (su dashboard/API/components).
No duplicar esa lógica en nexus-saas.

## FASE 3: Frontend completo

### 3.1 Páginas que exponen backend existente

El backend ya soporta estas features. Solo falta UI:

```
/settings/keys      → API keys: listar, crear, rotar, revocar
/secrets            → Secrets management (CRUD ya existe en nexus-core)
/policies           → Policies management (CRUD ya existe en nexus-core)
/incidents          → Incidents viewer (GET /v1/incidents en nexus-saas)
/events             → Events viewer (GET /v1/events en nexus-saas)
/assistant          → AI assistant chat (POST /v1/assistant/query en ai-operators)
```

### 3.2 Eliminar hardcoded API key

Actualmente nexus-tower usa un API key hardcoded. Reemplazar:
- Eliminar localStorage "nexus-api-key" hardcoded
- Todo request va con Authorization: Bearer {clerk-jwt}
- Mantener fallback de API key solo para desarrollo sin Clerk

## FASE 4: Variables de entorno y Docker

### 4.1 .env nuevas variables
```
# Clerk
VITE_CLERK_PUBLISHABLE_KEY=pk_test_...
CLERK_SECRET_KEY=sk_test_...
CLERK_WEBHOOK_SECRET=whsec_...

# JWT (apunta a Clerk)
NEXUS_AUTH_ENABLE_JWT=true
NEXUS_JWKS_URL=https://xxx.clerk.accounts.dev/.well-known/jwks.json
NEXUS_JWT_ISSUER=https://xxx.clerk.accounts.dev
```

### 4.2 docker-compose.yml
- nexus-tower: agregar VITE_CLERK_PUBLISHABLE_KEY
- nexus-saas: agregar CLERK_SECRET_KEY, CLERK_WEBHOOK_SECRET
- nexus-core: actualizar NEXUS_JWKS_URL

## Restricciones

- NO modificar el pipeline de gateway en nexus-core
- NO romper API key auth (debe seguir funcionando para M2M, scripts, operators)
- Toda la lógica de usuarios en nexus-saas, NO en nexus-core
- nexus-core solo valida JWT (ya lo hace)
- Backwards compatible con scripts e2e existentes
- Tests unit + integration para cada endpoint nuevo
- Migración con down migration funcional

## Estructura de archivos esperada

```
control-plane/
  migrations/0003_users_and_members.up.sql
  migrations/0003_users_and_members.down.sql
  internal/clerkwebhook/
    handler.go
  internal/user/
    handler.go
    handler/dto/dto.go
    repository.go
    repository/models/models.go
    usecases.go
    usecases/domain/entities.go
  internal/apikey/
    handler.go
    handler/dto/dto.go
    repository.go
    repository/models/models.go
    usecases.go

tower/
  src/pages/LoginPage.tsx
  src/pages/SignupPage.tsx
  src/pages/SettingsPage.tsx
  src/pages/OrgSettingsPage.tsx
  src/pages/ApiKeysPage.tsx
  src/pages/SecretsPage.tsx
  src/pages/PoliciesPage.tsx
  src/pages/IncidentsPage.tsx
  src/pages/EventsPage.tsx
  src/pages/AssistantPage.tsx
  src/components/ProtectedRoute.tsx
  src/contexts/AuthContext.tsx  (ya existe, conectar con Clerk)
  src/api/client.ts  (ya existe, agregar Bearer token)
```

## Orden de ejecución recomendado

**Aclaración importante**: este orden existe solo para respetar dependencias técnicas. Todo el contenido del prompt sigue siendo obligatorio.

```
Fase 1 (día 1-2):  Clerk + nexus-tower auth (login, signup, protected routes)
Fase 2 (día 2-3):  Webhook + migración + endpoints en nexus-saas
Fase 3 (día 3-5):  Frontend páginas nuevas (keys, secrets, policies, incidents, assistant)
Fase 4 (día 5):    Config Docker, e2e tests
```

## Criterios de éxito

- [ ] Un usuario puede registrarse desde el browser via Clerk
- [ ] Un usuario puede loguearse y ver sus tools
- [ ] Todas las rutas están protegidas (sin login → redirect a /login)
- [ ] El JWT de Clerk es validado por nexus-core via JWKS
- [ ] Webhook sincroniza usuarios/orgs de Clerk a nexus-saas DB
- [ ] Un admin puede crear/rotar/revocar API keys desde la UI
- [ ] API keys M2M siguen funcionando (backwards compatible)
- [ ] Scripts e2e existentes siguen pasando
- [ ] Tests para todos los endpoints nuevos
- [ ] Migración up y down funcional
