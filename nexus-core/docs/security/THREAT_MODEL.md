# THREAT_MODEL.md

## Scope

- Gateway API (`/v1/*`, `/mcp`)
- Admin API (`/v1/admin/*`)
- Admin Console (`/admin`)
- Storage: Postgres + Redis

## Activos criticos

- Secrets de herramientas
- Policies de autorizacion
- Audit trail y eventos admin
- Claves API/JWT claims

## Fronteras de confianza

1. Cliente/Agente -> Gateway
2. Gateway -> Tool upstream
3. Gateway -> DB/Redis
4. Admin Console -> Admin API

## Amenazas principales (STRIDE resumido)

1. Spoofing identidad
- Riesgo: API key robada/JWT mal validado.
- Mitigacion: hash API key, JWT issuer/audience claims, scopes intersectados.

2. Tampering politicas/config
- Riesgo: cambios admin no auditados.
- Mitigacion: `admin_activity_events` + RBAC por permisos.

3. Repudiation
- Riesgo: sin evidencia verificable de acciones.
- Mitigacion actual: audit + hash-chain.
- Roadmap: receipts firmados verificables externamente.

4. Info disclosure
- Riesgo: exfiltracion a hosts no autorizados.
- Mitigacion: egress default-deny + SSRF protection + DLP summary.

5. DoS
- Riesgo: flood requests o saturacion upstream.
- Mitigacion: rate limit + timeout budget + max body/response bytes.

6. Elevation of privilege
- Riesgo: scopes excesivos para funciones admin.
- Mitigacion: permisos granulares (`admin:console:read/write`) y role checks.

## Controles pendientes prioritarios

- OIDC/SSO para consola admin.
- RBAC completo por accion y recurso.
- Hard limits por tenant/plan en enforcement runtime.
- Red-team agentic gates en CI.

