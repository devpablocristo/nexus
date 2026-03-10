# Prompt 03 — UI de Admin Console en nexus-tower

## Contexto del proyecto

Nexus es una plataforma SaaS (Go + React/TypeScript) compuesta por:

| Servicio | Stack | Puerto | Descripción |
|----------|-------|--------|-------------|
| `nexus-core` | Go/Gin | 8080 | Gateway pipeline, auth, tools, policies, egress, audit |
| `nexus-saas` | Go/Gin | 8082 | Eventos, incidentes, acciones, admin, billing, users |
| `nexus-tower` | React/Vite | 5173 | SPA frontend (Clerk auth, TanStack Query) |

**Stack frontend**: React 18, Vite, TypeScript, React Router 6, TanStack Query, Clerk, CSS custom (sin Tailwind).

## Alcance obligatorio

Este prompt hereda los estándares de `docs/prompts/00_base_transversal.md`.

Todo lo definido acá es obligatorio para la Admin Console:
- superficies de administración reales
- permisos correctos
- consumo de APIs existentes sin duplicación
- consistencia visual y de UX con `nexus-tower`
- testing, errores y estados de carga acordes al resto del producto

La secuencia sugerida existe solo por dependencias técnicas.

## Prerequisito

Leer y respetar `docs/prompts/00_base_transversal.md` antes de ejecutar este prompt.

---

## Lo que YA existe en el backend (NO tocar)

### 4 endpoints de Admin API (nexus-saas, bajo `/v1`, requieren auth JWT o API key)

#### 1. `GET /v1/admin/bootstrap`

Devuelve datos del usuario autenticado + tenant settings de su org.

```json
{
  "org_id": "uuid",
  "actor": "user_ext_id",
  "role": "admin",
  "scopes": ["admin:console:read", "admin:console:write", ...],
  "auth_method": "jwt",
  "can_read_admin": true,
  "can_write_admin": true,
  "tenant_settings": {
    "plan_code": "growth",
    "hard_limits": {
      "tools_max": 75,
      "run_rpm": 1200,
      "audit_retention_days": 90
    },
    "updated_by": "user_ext_id",
    "updated_at": "2026-03-05T03:00:00Z",
    "created_at": "2026-02-01T10:00:00Z"
  }
}
```

Autorización: `admin:console:read` scope, o rol `admin`/`secops`.

#### 2. `GET /v1/admin/tenant-settings`

```json
{
  "plan_code": "growth",
  "hard_limits": {
    "tools_max": 75,
    "run_rpm": 1200,
    "audit_retention_days": 90
  },
  "updated_by": "user_ext_id",
  "updated_at": "2026-03-05T03:00:00Z",
  "created_at": "2026-02-01T10:00:00Z"
}
```

#### 3. `PUT /v1/admin/tenant-settings`

Request body:

```json
{
  "plan_code": "enterprise",
  "hard_limits": {
    "tools_max": 250,
    "run_rpm": 5000,
    "audit_retention_days": 365
  }
}
```

Autorización: rol `admin` o scope `admin:console:write`.

#### 4. `GET /v1/admin/activity?limit=50`

```json
{
  "items": [
    {
      "id": "uuid",
      "actor": "user_ext_id",
      "action": "upsert_tenant_settings",
      "resource_type": "tenant_settings",
      "resource_id": "org_uuid",
      "payload": { "plan_code": "growth", "hard_limits": { ... } },
      "created_at": "2026-03-05T03:00:00Z"
    }
  ]
}
```

### APIs existentes que la Admin Console también puede consumir

| API | Ruta | Servicio | Datos útiles |
|-----|------|----------|--------------|
| Billing status | `GET /v1/billing/status` | saas | plan_code, billing_status, current_period_end, hard_limits, usage |
| Usage | `GET /v1/billing/usage` | saas | 4 contadores del periodo (api_calls, events, incidents, actions) |
| Org members | `GET /v1/orgs/:org_id/members` | saas | lista de miembros con roles |
| Tools | `GET /v1/tools` | core | lista de tools registradas |
| API keys | `GET /v1/orgs/:org_id/api-keys` | saas | lista de API keys activas |

**Todas estas funciones ya existen en `tower/src/lib/api.ts`**. No crear duplicados.

### Hard limits por plan (referencia, no hardcodear en frontend)

| Plan | tools_max | run_rpm | audit_retention_days |
|------|-----------|---------|---------------------|
| starter | 20 | 300 | 30 |
| growth | 75 | 1,200 | 90 |
| enterprise | 250 | 5,000 | 365 |

---

## Lo que YA existe en el frontend (no duplicar)

### Páginas actuales (13)

Tools, Audit Log, Monitoring, Secrets, Policies, Incidents, Events, Assistant, API Keys, Billing, Organizations, Profile, BillingSuccess.

### Patrón establecido

- Cada página usa `useQuery` / `useMutation` de TanStack Query
- API client en `src/lib/api.ts` con `requestJSON('core' | 'saas', path)`
- Tipos en `src/lib/types.ts`
- Estilos en CSS global (`src/index.css`), no Tailwind
- Rutas en `src/app/App.tsx`, nav items en `src/components/Shell.tsx`
- Páginas en `src/pages/XxxPage.tsx`

### BillingPage como referencia de diseño

La BillingPage es el mejor modelo a seguir para la Admin Console:
- Summary cards (plan, status, next billing)
- Usage bars con labels y porcentajes
- Grids de límites
- Secciones con `<h3>` y `className="billing-section"`
- Botones de acción contextuales

---

## Qué implementar

### Fase 1 — API client y tipos

#### 1.1 Nuevos tipos (`src/lib/types.ts`)

```typescript
export type AdminBootstrap = {
  org_id: string;
  actor?: string;
  role?: string;
  scopes: string[];
  auth_method: string;
  can_read_admin: boolean;
  can_write_admin: boolean;
  tenant_settings: AdminTenantSettings;
};

export type AdminTenantSettings = {
  plan_code: string;
  hard_limits: {
    tools_max: number;
    run_rpm: number;
    audit_retention_days: number;
  };
  updated_by?: string;
  updated_at?: string;
  created_at?: string;
};

export type AdminActivityItem = {
  id: string;
  actor?: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  payload: Record<string, unknown>;
  created_at: string;
};
```

#### 1.2 Nuevas funciones API (`src/lib/api.ts`)

```typescript
export async function getAdminBootstrap(): Promise<AdminBootstrap>
export async function getAdminTenantSettings(): Promise<AdminTenantSettings>
export async function updateAdminTenantSettings(req: { plan_code: string; hard_limits: Record<string, number> }): Promise<AdminTenantSettings>
export async function getAdminActivity(limit?: number): Promise<{ items: AdminActivityItem[] }>
```

---

### Fase 2 — Página Admin Dashboard

#### Ruta: `/admin`

#### Layout de la página

```
┌─────────────────────────────────────────────────────────────┐
│ Admin Console                                                │
│ Manage your organization's plan, limits, and activity.       │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Overview                                                     │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌───────────┐ │
│  │ Plan       │ │ Status     │ │ Members    │ │ Tools     │ │
│  │ Growth     │ │ Active     │ │ 5          │ │ 12 / 75   │ │
│  └────────────┘ └────────────┘ └────────────┘ └───────────┘ │
│                                                               │
│  Plan & Limits                                [Edit Limits]   │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Plan Code        Growth                                │  │
│  │ Tools            12 / 75                               │  │
│  │ Rate Limit       1,200 rpm                             │  │
│  │ Audit Retention  90 days                               │  │
│  │ Last Updated     Mar 5, 2026 by user@acme.com          │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
│  Usage This Period (Mar 2026)                                 │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ API Calls       12,450                                 │  │
│  │ ████████████░░░░░░░░                                   │  │
│  │ Events           3,200                                 │  │
│  │ ██████░░░░░░░░░░░░░░                                   │  │
│  │ Incidents           18                                 │  │
│  │ █░░░░░░░░░░░░░░░░░░░                                   │  │
│  │ Actions             45                                 │  │
│  │ ██░░░░░░░░░░░░░░░░░░                                   │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
│  Recent Activity                              [View all →]    │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ When             Who            Action     Resource     │  │
│  │ Mar 5, 03:00     admin@acme     upsert..   tenant_set  │  │
│  │ Mar 4, 22:15     admin@acme     create..   api_key     │  │
│  │ Mar 4, 18:30     —              rotate..   api_key     │  │
│  │ Mar 3, 14:00     admin@acme     upsert..   tenant_set  │  │
│  │ ...                                                     │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

#### Datos consumidos

| Sección | API | Query key |
|---------|-----|-----------|
| Overview cards | `getAdminBootstrap()` + `getTools()` + `getOrgMembers(orgId)` | `admin-bootstrap`, `tools`, `org-members` |
| Plan & Limits | `getAdminBootstrap()` (reusa query) | `admin-bootstrap` |
| Usage | `getUsageSummary()` (ya existe en api.ts) | `billing-usage` |
| Recent Activity | `getAdminActivity(10)` | `admin-activity` |

#### Edit Limits modal

Al hacer clic en "Edit Limits", abrir un modal con:

```
┌───────────────────────────────────────┐
│ Edit Plan Limits                       │
│                                         │
│ Plan Code                               │
│ ┌─────────────────────────────────┐    │
│ │ growth                     ▼    │    │
│ └─────────────────────────────────┘    │
│                                         │
│ Max Tools                               │
│ ┌─────────────────────────────────┐    │
│ │ 75                              │    │
│ └─────────────────────────────────┘    │
│                                         │
│ Rate Limit (rpm)                        │
│ ┌─────────────────────────────────┐    │
│ │ 1200                            │    │
│ └─────────────────────────────────┘    │
│                                         │
│ Audit Retention (days)                  │
│ ┌─────────────────────────────────┐    │
│ │ 90                              │    │
│ └─────────────────────────────────┘    │
│                                         │
│ When you select a plan, default limits  │
│ are auto-filled. You can override them. │
│                                         │
│        [Cancel]     [Save Changes]      │
└───────────────────────────────────────┘
```

Comportamiento:
- Al cambiar plan_code en el dropdown, auto-rellenar los hard_limits con los defaults (starter/growth/enterprise)
- Permitir override manual de cada límite
- `PUT /v1/admin/tenant-settings` al guardar
- Invalidar query `admin-bootstrap` al éxito
- Solo visible si `can_write_admin === true`

---

### Fase 3 — Página Activity Log completa

#### Ruta: `/admin/activity`

Tabla paginada con las últimas 200 actividades administrativas.

```
┌─────────────────────────────────────────────────────────────┐
│ Admin Activity Log                                           │
│ Track all administrative changes in your organization.       │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  When               Actor           Action          Resource  │
│  ──────────────────────────────────────────────────────────── │
│  Mar 5, 03:00:12    admin@acme.com  upsert_tenant   tenant.. │
│  Mar 4, 22:15:33    admin@acme.com  create_api_key  api_key  │
│  Mar 4, 18:30:45    system          rotate_api_key  api_key  │
│  Mar 3, 14:00:00    admin@acme.com  upsert_tenant   tenant.. │
│  ...                                                          │
│                                                               │
│  Showing 50 of 134                        [Load more]         │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

Al hacer clic en una fila, expandir inline para mostrar el `payload` JSON formateado.

---

### Fase 4 — Navegación y routing

#### Shell.tsx — agregar nav item

Insertar **antes** de "Billing":

```typescript
{ to: '/admin', label: 'Admin' },
```

El orden de nav debería quedar:
Tools → Audit Log → Monitoring → Secrets → Policies → Incidents → Events → Assistant → API Keys → **Admin** → Billing → Organizations → Profile

#### App.tsx — agregar rutas

```tsx
<Route path="/admin" element={<AdminPage />} />
<Route path="/admin/activity" element={<AdminActivityPage />} />
```

---

### Fase 5 — Permisos en la UI

#### Visibilidad del nav item "Admin"

El link "Admin" en la navegación debe **siempre mostrarse** — la página maneja la lógica de permisos.

#### Dentro de AdminPage

Usar `getAdminBootstrap()` para obtener `can_read_admin` y `can_write_admin`:
- Si `can_read_admin === false`: mostrar mensaje "You don't have permission to view admin settings."
- Si `can_read_admin === true && can_write_admin === false`: mostrar todo en modo lectura (ocultar "Edit Limits")
- Si `can_write_admin === true`: mostrar todo + botón "Edit Limits"

---

## Reglas de implementación

1. **Seguir el patrón existente** de las otras páginas (BillingPage es la referencia).
2. **Usar TanStack Query** para todas las queries y mutations (`useQuery`, `useMutation`, `queryClient.invalidateQueries`).
3. **No instalar dependencias nuevas** — usar lo que ya existe.
4. **CSS**: agregar estilos en `src/index.css` siguiendo los patrones existentes (`.billing-page`, `.summary-card`, `.usage-bar`, etc.). Reusar clases existentes cuando sea posible.
5. **No duplicar funciones de API** que ya existen (`getTools`, `getOrgMembers`, `getBillingStatus`, `getUsageSummary`). Importarlas.
6. **No tocar el backend** — solo frontend.
7. **Modal**: usar el mismo patrón de modal que usa ToolsPage para crear/editar tools.
8. **Formateo**: usar `Intl.NumberFormat` para números, `toLocaleDateString` para fechas.
9. **Error handling**: mostrar errores inline con `query.error`, no alerts.
10. **Plan auto-fill en modal**: cuando el usuario selecciona un plan diferente, auto-rellenar los 3 campos con los defaults de ese plan. Los defaults se obtienen del mapa hardcodeado (starter/growth/enterprise).

---

## Criterios de éxito

- [ ] `GET /v1/admin/bootstrap` se consume correctamente en AdminPage
- [ ] Overview cards muestran: plan, billing status, miembros, tools count
- [ ] Plan & Limits section muestra hard limits actuales con "last updated by"
- [ ] Usage bars reutilizan datos de `/v1/billing/usage`
- [ ] Recent Activity muestra las últimas 10 actividades con actor, action, resource
- [ ] Botón "Edit Limits" abre modal para editar plan_code + hard_limits
- [ ] Modal auto-rellena limits al cambiar plan
- [ ] `PUT /v1/admin/tenant-settings` se ejecuta al guardar
- [ ] Activity log page (`/admin/activity`) muestra 200 registros con expand para payload
- [ ] Permisos respetados: read-only sin `can_write_admin`, forbidden sin `can_read_admin`
- [ ] Nav item "Admin" agregado en Shell.tsx
- [ ] Rutas `/admin` y `/admin/activity` agregadas en App.tsx
- [ ] No se crearon funciones API duplicadas
- [ ] No se instalaron dependencias nuevas
- [ ] TypeScript compila sin errores (`npm run build`)
- [ ] Estilo visual consistente con BillingPage y el resto de Tower

---

## Archivos a crear/modificar

| Archivo | Acción |
|---------|--------|
| `src/lib/types.ts` | Agregar tipos AdminBootstrap, AdminTenantSettings, AdminActivityItem |
| `src/lib/api.ts` | Agregar 4 funciones: getAdminBootstrap, getAdminTenantSettings, updateAdminTenantSettings, getAdminActivity |
| `src/pages/AdminPage.tsx` | **Crear** — Admin Dashboard |
| `src/pages/AdminActivityPage.tsx` | **Crear** — Activity Log completo |
| `src/components/EditLimitsModal.tsx` | **Crear** — Modal para editar plan + limits |
| `src/app/App.tsx` | Agregar rutas /admin y /admin/activity |
| `src/components/Shell.tsx` | Agregar nav item "Admin" |
| `src/index.css` | Agregar estilos para admin pages |

---

## Orden de ejecución recomendado

**Aclaración importante**: este orden existe solo para respetar dependencias técnicas. Todo el contenido del prompt sigue siendo obligatorio.

1. Tipos en `types.ts`
2. Funciones API en `api.ts`
3. `EditLimitsModal.tsx`
4. `AdminPage.tsx` (consume bootstrap + tools + members + usage + activity)
5. `AdminActivityPage.tsx`
6. Rutas en `App.tsx`
7. Nav item en `Shell.tsx`
8. Estilos en `index.css`
9. Verificar `npm run build`
