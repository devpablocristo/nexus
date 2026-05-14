# Nexus

Nexus es el producto: incluye **governance** (servicio Go que decide
`allow`/`deny`/`require_approval`, administra approvals, audit y evidence) +
**console** (UI React) + Postgres local. El layout vive en el root del repo.

> El servicio **Companion** (`ProductAgent` transversal) y sus capabilities
> (`connectors`) viven en un proyecto independiente
> (`/home/pablocristo/Proyectos/pablo/companion/`). Se levantan en stacks
> separados; en runtime se integran vía HTTP (Companion consume Governance).

## Taxonomía

- `governance/` — servicio Go que enforce policies + audit + evidence packs
  (categoría canónica: `GovernanceService`). Es el plano de decisión.
- `console/` — UI React que consume `governance` vía proxy.
- `Nexus` (sin path) — el producto entero (governance + console).
- `Companion` (otro repo) es el `ProductAgent` que consume Nexus para gating.

## Contrato gobernado

Las acciones sensibles entran como `ToolIntent v1` dentro de
`action_binding`. Governance calcula `binding_hash = sha256(canonical_json)`
y lo devuelve en la decisión. El ejecutor externo, por ejemplo Companion, solo
puede ejecutar si el hash aprobado coincide con la acción real.

Nexus no incluye runtime LLM, prompts, agentes ni memoria IA. CI ejecuta un
guardrail para bloquear imports de SDKs/provider IA dentro de `governance/`.

Attestations: en producción `GOVERNANCE_ATTESTATION_VERIFIER` debe ser
`hmac-sha256` y requiere `GOVERNANCE_ATTESTATION_HMAC_SECRET`. El modo `none`
queda permitido solo para desarrollo/local y persiste evidence como
`verified=false`.

## Estructura

```
nexus/
├── governance/      # servicio Go (BE)
├── console/         # UI React (FE)
├── doc/
├── scripts/
├── docker-compose.yml
├── Makefile
├── .env.example
└── ...
```

## Arranque rápido

```bash
test -f .env || cp .env.example .env
make up
```

## URLs por defecto (host)

| Servicio         | URL                       |
|------------------|---------------------------|
| Nexus Governance | `http://localhost:18084`  |
| Console          | `http://localhost:13001`  |

Variables: ver `.env.example`.

## Tests

```bash
make test          # Go unit (governance)
make qa            # migraciones + Go build/vet/test -race + console si node_modules existe
make smoke         # Requiere API levantada (compose): policies + requests
make e2e           # Flujo de ciclo completo
```

## Console local

`console/` requiere **Node 22**. Para alinear local, Docker y CI:

```bash
cd console
nvm use
npm ci
npm run typecheck
npm run build
```

En local (`localhost`), `console` usa proxy same-origin para hablar con `governance`
sin exponer API keys en el bundle del browser. Para acceso remoto/browser con
sesión humana, configurá `GOVERNANCE_AUTH_ISSUER_URL` en backend y
`VITE_CLERK_PUBLISHABLE_KEY` en `console`.

## Documentación

- [governance/README.md](governance/README.md)
- [doc/NEXUS_COWORKER_VISION.md](doc/NEXUS_COWORKER_VISION.md) — visión: de capa de control a **compañero de trabajo completo**
- [doc/NEXUS_COMPLETION_ROADMAP.md](doc/NEXUS_COMPLETION_ROADMAP.md)
- [doc/NEXUS_ECOSYSTEM_DESIGN.md](doc/NEXUS_ECOSYSTEM_DESIGN.md)

## Deploy GCP (dev)

Workflow: [.github/workflows/deploy-governance-dev.yml](.github/workflows/deploy-governance-dev.yml) — push a `develop`.

Project: `pymes-dev-352318` · Region: `us-central1` · Instancia SQL
compartida: `pymes-dev-db` · DB: `nexus` · DB user: `governance_app`.

Infra ya aprovisionada (vía `gcloud`):

- DB `nexus` + user `governance_app` en `pymes-dev-db`.
- Runtime SA `governance-runtime-dev@pymes-dev-352318.iam.gserviceaccount.com` con `cloudsql.client` + `secretAccessor`.
- Secrets en Secret Manager: `governance-db-password`, `governance-api-keys`, `governance-signing-key`, `governance-callback-token`.
- WIF: repo `devpablocristo/nexus` habilitado en branch `develop`.

Variables a settear en GitHub (`devpablocristo/nexus` → Settings → Variables → Actions):

```text
GCP_PROJECT_ID_DEV = pymes-dev-352318
GCP_REGION = us-central1
WIF_PROVIDER_DEV = projects/884236221349/locations/global/workloadIdentityPools/github-actions-pool/providers/github-actions-provider
WIF_SERVICE_ACCOUNT_DEV = github-actions@pymes-dev-352318.iam.gserviceaccount.com
ARTIFACT_REGISTRY = pymes
CLOUDSQL_INSTANCE_DEV = pymes-dev-352318:us-central1:pymes-dev-db
CLOUD_RUN_SERVICE_ACCOUNT_DEV = governance-runtime-dev@pymes-dev-352318.iam.gserviceaccount.com
```

La instancia previa `nexus-db-dev` (proyecto `new-nexus-dev`) está
**apagada** para no generar costo; pendiente borrarla.
