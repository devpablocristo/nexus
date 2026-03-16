# Nexus v2 Docs

Documentacion principal de `v2`.

Orden recomendado de lectura:

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
