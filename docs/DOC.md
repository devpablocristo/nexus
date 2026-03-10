# Nexus Technical Docs

Documento índice canónico del repo.

## Arquitectura

Nexus es un plano de control para ejecución gobernada de herramientas por agentes y operadores.

**Convención del monorepo:** nombres internos de directorio sin prefijo `nexus`; nombres públicos y de deploy conservan el branding `nexus-*`.

| Directorio interno | Servicio desplegado | Rol |
|--------------------|--------------------|-----|
| `data-plane` | `nexus-core` | Data plane determinista: `/v1/run`, `/mcp`, `/a2a`, policies, DLP, egress, approvals, audit, execution intents, preflights, leases |
| `control-plane` | `nexus-saas` | Control plane multi-tenant: billing, users, incidents, actions, events, notifications, assistant proxy |
| `control-workers` | `nexus-control-operators` | Workers deterministas del control plane (sentry, coordinator, mitigation, recovery) |
| `ai-runtime` | `nexus-ai-operators` | Runtime AI asistido: assistant, prompting versionado, fallback, evals |
| `tower` | `nexus-tower` | UI de supervisión (Tower) |

**Invariantes:**

- No LLM en el pipeline de enforcement.
- Operators sin writes directos a DB.
- `data-plane` y `control-plane` mantienen ownership separado.
- Contratos, docs, SDKs y observabilidad forman parte del producto.

## Dónde leer cada tema

- **Límites y nombres:** `docs/NAMING_AND_BOUNDARIES.md`
- **Boundaries y ownership por servicio:** `docs/SERVICE_BOUNDARIES.md`
- **Modelo agent-operated:** `docs/AGENT_OPERATED_MODEL.md`
- **Policy DSL:** `docs/policy/POLICY_DSL_REFERENCE.md`, `docs/policy/POLICY_DSL_COOKBOOK.md`
- **MCP:** `docs/protocols/MCP_GUIDE.md`
- **A2A:** `docs/protocols/A2A_GUIDE.md`
- **Data ownership y modelo:** `docs/data/DATA_MODEL_AND_OWNERSHIP.md`
- **Event catalog y ejemplos:** `docs/events/EVENT_CATALOG.md`, `docs/events/examples/*.json`
- **API versioning:** `docs/api/VERSIONING_POLICY.md`
- **Billing / grace period:** `docs/billing/GRACE_PERIOD_POLICY.md`
- **Testing y release gates:** `docs/testing/TEST_STRATEGY.md`, `docs/testing/RELEASE_GATES.md`
- **Engineering (onboarding, estándares):** `docs/engineering/ONBOARDING.md`, `docs/engineering/CODING_STANDARDS.md`
- **Incident response y runbooks:** `docs/runbooks/*`
- **ADRs:** `docs/adr/README.md`, `docs/adr/0001-*.md` … `docs/adr/0008-*.md`

## Contratos

- APIs públicas bajo `/v1/*`
- JSON-RPC en `/mcp`
- REST A2A en `/a2a/call`
- **Contratos compartidos en `pkgs/contracts/`:**
  - `pkgs/contracts/openapi.nexus-core.snapshot.yaml` (canónico: `data-plane/docs/openapi.yaml`)
  - `pkgs/contracts/openapi.nexus-saas.snapshot.yaml` (canónico: `control-plane/docs/openapi.yaml`)
  - `pkgs/contracts/error-codes.json`
  - `pkgs/contracts/events.schema.json`
- **Developer portal (Tower):** OpenAPI y Postman se publican en `tower/public/downloads/` (nexus-core.openapi.yaml, nexus-saas.openapi.yaml, colecciones Postman).
- **Postman:** `docs/postman/nexus-core.postman_collection.json`, `docs/postman/nexus-saas.postman_collection.json`

Antes de cerrar cambios que tocan contratos u OpenAPI: `make contracts-check`.

## Operación

- Launch checklist: `docs/runbooks/LAUNCH_CHECKLIST.md`
- SLO/SLI: `docs/runbooks/SLO_SLI.md`
- Rollback: `docs/runbooks/DEPLOY_ROLLBACK.md`
- Backup/DR: `docs/runbooks/DB_BACKUP_DR.md`
- Secret rotation: `docs/runbooks/SECRET_ROTATION.md`
- Incident response: `docs/runbooks/INCIDENT_RESPONSE.md`
- Postmortem: `docs/runbooks/POSTMORTEM_TEMPLATE.md`

## Infraestructura

- Terraform en `infra/`: módulos para networking, database, ECS, CDN, DNS, cache, monitoring, secrets, ECR, load balancer.
- Validación: `make infra-validate` (scripts en `scripts/infra/`).

## SDKs

- `sdks/python-sdk`
- `sdks/typescript-sdk`
- `sdks/go-sdk`

Validación: `make sdk-test`.

## Suite de prompts

La especificación histórica y de alcance vive en `docs/prompts/`: desde `00_base_transversal.md` hasta `17_architecture_decision_records.md` (identidad, billing, notificaciones, admin UI, DX/CICD, infra, seguridad, monitoring, polish, launch, AI runtime, policy/MCP/A2A, datos/eventos, incident response, onboarding, test strategy, ADRs).
