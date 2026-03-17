# Nexus v2 Docs

Documentacion principal de `v2`. Entrada unica recomendada para entender producto, estado y tecnica.

**Que es Nexus:** El nexo unico entre agentes financieros y lo protegido (engine determinista que gobierna autoridad delegada) y entre los equipos humanos que supervisan (un solo agente IA que notifica, contextualiza y ofrece acciones en un chat unico).

**Estado actual:** MVP cerrado, Fase 0 cerrada en local (4 deploy blockers), Fase 1A en runtime (cascada, canaries, baselines), saas-core integrado (billing, auth, tenancy). Ver [ONE_PAGER.md](ONE_PAGER.md) para producto y GTM.

**Orden recomendado de lectura:**

1. [DEFINITION.md](DEFINITION.md)
   - definicion de producto, nicho, negocio y separacion por servicio
2. [MVP.md](MVP.md)
   - alcance cerrado del MVP y criterios de cierre
3. [PRE_PROD.md](PRE_PROD.md)
   - checklist de endurecimiento despues del MVP
4. [PROD_CHECKLIST.md](PROD_CHECKLIST.md)
   - gate final para decidir salida a produccion
5. [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md)
   - reglas tecnicas, convenciones, auth, persistencia y superficies actuales
6. [ENDPOINT_FLOWS.md](ENDPOINT_FLOWS.md)
   - flujo interno endpoint por endpoint
7. [ROADMAP.md](ROADMAP.md)
   - fases post-MVP con disenos cerrados (1A, 1B, 1C) y pendientes (2-6)
8. [OPS.md](OPS.md)
   - guia operativa: topologia, rollout, rollback, config, runbooks
9. [REVIEW.md](REVIEW.md)
   - historial de decisiones de diseno (colaboracion Claude + GPT)
10. [ONE_PAGER.md](ONE_PAGER.md)
    - una pagina: problema, solucion, diferenciacion, estado, proximo paso (ventas/partners)
11. [POLISH_PLAN.md](POLISH_PLAN.md)
    - plan de pulido de doc, producto y GTM
12. [CURSOR_ONBOARDING.md](CURSOR_ONBOARDING.md)
    - onboarding para agentes (Cursor): que falta, prioridad, convenciones
13. [V1_STUDY.md](V1_STUDY.md)
    - estudio completo de Nexus v1 (repo `v1/`): arquitectura, flujos, conceptos reutilizables para v2

Assets operativos relevantes:

- `v2/infra`
  - baseline AWS en Terraform para `data-plane`, `control-plane` y `control-workers`
- `v2/ops/observability`
  - Prometheus, Grafana, rules y dashboards de pre-prod
- `make -C v2 smoke-observability`
  - valida el stack de observabilidad local
- `v2/scripts/ops`
  - backup y restore manual de PostgreSQL para pre-prod
- `make -C v2 smoke-persistence`
  - valida restart con estado persistido en compose
- `make -C v2 smoke-db-restore`
  - valida backup + restore manual del `control-plane`
- `cd v2/infra && terraform validate`
  - valida el scaffold AWS del stack `v2`

Reglas de estructura relevantes:

- artefactos agnosticos y multilenguaje viven bajo `v2/pkgs/contracts`
- codigo agnostico compartido vive bajo `v2/pkgs/go-pkg`
- codigo especifico de servicio vive dentro de cada modulo:
  - `data-plane`
  - `control-plane`
  - `control-workers`
  - `ai-runtime`

## Proximos pasos (resumen)

1. Cerrar 4 deploy blockers de [PRE_PROD.md](PRE_PROD.md) y pasar [PROD_CHECKLIST.md](PROD_CHECKLIST.md).
2. Migrations de saas-core en control-plane y tests de saas-core.
3. Integrar saas-core en Pymes y Ponti; features diferenciales (CapabilityLease, AI Analyst).
4. GTM: demo, doc de integracion, prospects (meta: segunda semana junio 2026). Detalle en [CURSOR_ONBOARDING.md](CURSOR_ONBOARDING.md).
