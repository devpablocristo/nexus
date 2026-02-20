# PLAN_90_DAYS.md

## Assumptions (Sprint 1)

- Consola Admin: stack simple y estandar (`HTML/CSS/JS` estatico servido por Gin en `/admin`) para minimizar costo operativo y acelerar time-to-first-run.
- API sigue siendo el source of truth; consola consume endpoints existentes y endpoints `v1/admin/*`.
- No se crea ningun camino de ejecucion alterno: todo run de tools sigue pasando por `gateway.Run`.

## Guardrails

- No iniciar iniciativas de Fase 4+ (A2A bridge/zero-trust delegation/red-team gates avanzados) sin `>=1 cliente pago` o `>=1-2 POCs avanzadas con señales claras`.
- Priorizar solo trabajo que reduzca onboarding o desbloquee cierre enterprise.

## Week-by-week (13 semanas)

| Semana | Entregables tecnicos | Backend/API | Consola | AuthN/Z | Storage | Observabilidad | Docs/CI-CD | DoD |
|---|---|---|---|---|---|---|---|---|
| 1 | Admin MVP skeleton | `GET /v1/admin/bootstrap`, `GET/PUT /v1/admin/tenant-settings` | Shell UI `/admin` + session headers | Permisos por scope (`admin:console:read/write`) | `tenant_settings`, `admin_activity_events` | `/metrics` + contador run prometheus | Plan inicial + quickstart | Compila, migra, rutas responden 200/403 correctamente |
| 2 | Demo reproducible | scripts `quickstart_admin.sh` | REST+MCP demo panel | seed con scopes admin | defaults por plan (`starter/growth/enterprise`) | Prometheus+Grafana starter dashboard | runbook deploy+incidentes v1 | Clean install < 60 min con demo end-to-end |
| 3 | RBAC hardening | matriz permisos por endpoint | UI de permisos visibles | modelo permisos por accion | columnas para control admin opcional | alertas base p95/5xx/blocked | pruebas authz+smoke | cuestionario security sin huecos criticos de authz |
| 4 | OIDC/SSO integration | soporte claims mapeados a roles/scopes | login OIDC (si aplica) | JWKS/OIDC config hardening | audit de cambios auth config | dashboard auth failures | runbook OIDC | login enterprise usable sin headers manuales |
| 5 | Admin audit trail completo | auditar cambios tools/policies/secrets/egress | timeline de cambios admin | enforcement de least-privilege | esquema eventos admin extendido | panel "admin changes" | test regresion auditoria | who/what/when en toda accion administrativa |
| 6 | Onboarding 1h final | endpoints y ejemplos curados | wizard de onboarding (MVP) | plantillas scopes por rol | fixtures demo | panel onboarding KPIs | guia comercial tecnica | 1er flujo integrado por usuario nuevo < 60 min |
| 7 | Tenant hard limits v1 | enforcement de cuotas por plan en run/control plane | vista limites + consumo | permisos para override controlado | persistencia cuotas/consumo | alertas near-limit | pruebas de carga de cuota | limite bloquea correctamente sin bypass |
| 8 | Operacion enterprise | health/readiness mejorado + maintenance mode opcional | estado de sistema en consola | break-glass procedure | backups/restore automatizables | alertas SLO + error budget | runbooks P1/P2 final | restore probado y documentado |
| 9 | Release gates duros | make targets unit+integration+e2e+smoke release | changelog UI | policy para cambios de permisos | migraciones verificadas en CI | métricas de release | pipeline CI con gates | release falla si smoke/e2e fallan |
| 10 | Sales packaging | APIs de limites por plan estables | plan badges en consola | roles por plan | seeds por plan | dashboard por tenant/plan | pricing + demo playbooks v2 | material de venta listo para discovery+POC |
| 11 | SLO/SLA formal | telemetria para SLO | estatus SLO en consola | auditoria de excepciones | retention policy aplicada | SLI dashboard oficial | documento SLA | SLO 99.9% definido y medible |
| 12 | POC acceleration | plantillas de integracion vertical | templates de demos verticales | scopes por vertical | datasets demo | dashboard por vertical | one-pager + ROI | 1-2 POCs avanzadas con metricas |
| 13 | Gate vendible | freeze + hardening + bugbash | polish UX de demo | revisión permisos final | backup/restore drill final | dashboard final cliente | checklist GTM final | listo para cerrar contrato piloto |

## Dependencias (orden correcto)

1. Admin storage + rutas + permisos (W1) -> base para consola.
2. Quickstart + demo + observabilidad (W2) -> base para venta.
3. RBAC/OIDC + admin audit trail (W3-W5) -> base para enterprise.
4. Hard limits tenant/plan (W7) -> control comercial/operativo.
5. Release gates + SLO/SLA + runbooks (W8-W11) -> confianza operativa.

## Riesgos y mitigaciones

- Riesgo: dispersarse en features futuristas temprano.
  - Mitigacion: regla de negocio, no abrir Fase 4 sin señal comercial.
- Riesgo: consola sin seguridad enterprise.
  - Mitigacion: RBAC/OIDC obligatorio antes de go-live enterprise.
- Riesgo: deuda operativa (sin restore probado).
  - Mitigacion: backup/restore drill agendado en W8 y W13.

## Sprint 1-2 acceptance (checklist)

- [x] Consola admin accesible en `/admin`.
- [x] Endpoints admin con permisos por scopes/rol.
- [x] Storage para plan+hard limits y admin activity.
- [x] Quickstart reproducible REST + MCP + bootstrap admin.
- [x] Métricas `/metrics` + dashboard starter (Grafana).

