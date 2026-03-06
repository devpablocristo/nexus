# Data Model And Ownership

Mapa canónico de ownership por bounded context.

## Principios

- `nexus-core` es source of truth del data plane determinista.
- `nexus-saas` es source of truth del business plane multi-tenant.
- `nexus-control-operators` y `nexus-ai-operators` nunca escriben directo a PostgreSQL.
- Si una vista cruza servicios, sigue habiendo un owner de escritura único.

## `nexus-core` (`nexus`)

| Entidad / tabla | Write owner | Readers | Notas |
|-----------------|-------------|---------|-------|
| `orgs`, `org_api_keys`, `org_api_key_scopes` | `nexus-core` | core, tower, sdks | onboarding y auth M2M |
| `tools` | `nexus-core` | core, tower, MCP | catálogo de tools |
| `policies` | `nexus-core` | core, tower | Policy DSL y límites |
| `egress_rules` | `nexus-core` | core, tower | allowlist SSRF/egress |
| `tool_secrets` | `nexus-core` | core | secretos cifrados |
| `audit_events` | `nexus-core` | core, tower, export | hash-chain, append-only |
| `pending_approvals` | `nexus-core` | core, tower | HITL |
| `idempotency_keys` | `nexus-core` | core | replay/conflict/in-progress |
| `agent_sessions` | `nexus-core` | core, saas/tower via API | estado operativo por sesión |

## `nexus-saas` (`nexus_saas`)

| Entidad / tabla | Write owner | Readers | Notas |
|-----------------|-------------|---------|-------|
| `orgs`, `org_api_keys`, `org_api_key_scopes` | `nexus-saas` | saas | auth/business plane propio |
| `tenant_settings` | `nexus-saas` | saas, core via internal entitlements | plan, billing_status, tenant status |
| `admin_activity_events` | `nexus-saas` | saas, tower | actividad admin |
| `org_usage_counters`, `saas_usage_event_dedup` | `nexus-saas` | saas, tower | metering |
| `users`, `org_members` | `nexus-saas` | saas, tower | synced desde Clerk |
| `notification_preferences`, `notification_log`, `in_app_notifications` | `nexus-saas` | saas, tower | email e in-app |
| `events` | `nexus-saas` | saas, operators vía API | event stream operativo del business plane |
| `incidents` | `nexus-saas` | saas, tower | incidentes visibles al tenant |
| `actions` | `nexus-saas` | saas, tower | overrides temporales |
| `policy_proposals`, `policy_versions` | `nexus-saas` | saas, tower | workflow humano |
| `alert_rules` | `nexus-saas` | saas, tower | alerting de negocio |
| `agent_sessions` | `nexus-saas` | saas, tower | vistas SaaS/assistant |

## Cross-service rules

- `nexus-core` consulta entitlements y runtime-overrides en `nexus-saas`; no escribe allí.
- `nexus-saas` usa core proxy o assistant proxy; no implementa enforcement.
- Los operators leen y actúan por HTTP:
  - `nexus-control-operators` vía `nexus-core /internal/operators/*`
  - `nexus-ai-operators` vía `nexus-core /internal/operators/*` y proxy assistant en `nexus-saas`

## Source of truth resumido

- Billing / tenant lifecycle: `nexus-saas`
- Auth gateway, approvals, audit, Policy DSL, egress y secrets: `nexus-core`
- Supervisión UI: `nexus-tower` consume APIs; no duplica ownership

## Retención y cleanup

- `audit_events`: controlado por el plano core y políticas de retención/export.
- `events`, `notifications`, `usage`: retención SaaS según plan y políticas operativas.
- Tenant `deleted`: soft-delete en `tenant_settings`; cleanup posterior controlado por runbook/lifecycle.
