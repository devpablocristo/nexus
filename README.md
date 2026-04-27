# Nexus

Nexus es el producto: incluye **governance** (servicio Go que decide approve/reject,
audit, evidence) + **console** (UI React) + Postgres local. El layout vive en el
root del repo.

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
