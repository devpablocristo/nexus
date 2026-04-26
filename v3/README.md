# Nexus v3

Stack activo: **Nexus Governance** (categoría canónica: `GovernanceService`),
**console** (UI) y Postgres local.

> El servicio **Companion** (`ProductAgent` transversal) y sus capabilities
> (`connectors`) ahora viven en un proyecto independiente
> (`/home/pablocristo/Proyectos/pablo/companion/`). Se levantan en stacks
> separados; en runtime se integran vía HTTP (Companion consume Nexus).

## Taxonomía IA

- `Nexus Governance` es el nombre comercial del `GovernanceService` soberano.
- `Companion` (en su propio repo) es el nombre comercial del `ProductAgent`
  transversal y consume `Nexus Governance` para gating/audit.

## Arranque rápido

Desde este directorio (`v3/`):

```bash
test -f .env || cp .env.example .env
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build
```

## URLs por defecto (host)

| Servicio         | URL                       |
|------------------|---------------------------|
| Nexus Governance | `http://localhost:18084`  |
| Console          | `http://localhost:13001`  |

Variables: ver `.env.example`.

## Tests

```bash
make test          # Go unit (nexus)
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

En local (`localhost`), `console` usa proxy same-origin para hablar con `nexus`
sin exponer API keys en el bundle del browser. Para acceso remoto/browser con
sesión humana, configurá `NEXUS_AUTH_ISSUER_URL` en backend y
`VITE_CLERK_PUBLISHABLE_KEY` en `console`.

## Documentación

- [nexus/README.md](nexus/README.md)
- [doc/NEXUS_COWORKER_VISION.md](doc/NEXUS_COWORKER_VISION.md) — visión: de capa de control a **compañero de trabajo completo**
- [doc/NEXUS_COMPLETION_ROADMAP.md](doc/NEXUS_COMPLETION_ROADMAP.md)
- [doc/NEXUS_ECOSYSTEM_DESIGN.md](doc/NEXUS_ECOSYSTEM_DESIGN.md)
