# Nexus v3

Stack activo: **Nexus Governance** (categoría canónica: `GovernanceService`), **Companion** (categoría canónica: `ProductAgent` transversal), **console** (UI) y **Postgres**.

## Taxonomía IA

- `Nexus Governance` es el nombre comercial del `GovernanceService` soberano.
- `Companion` es el nombre comercial del `ProductAgent` transversal de Nexus.
- `Companion` consume y propone acciones, pero no reemplaza a los agentes embebidos de producto como `pymes`.

## Arranque rápido

Desde este directorio (`v3/`):

```bash
test -f .env || cp .env.example .env
docker compose up -d --build
```

Si Postgres ya tenía volumen **sin** la base `nexus_companion`:

```bash
bash scripts/dev/ensure-companion-db.sh
```

Luego reiniciá **companion** si hacía falta la base (`docker compose restart companion`).

## URLs por defecto (host)

| Servicio  | URL |
|-----------|-----|
| Nexus Governance | `http://localhost:18084` |
| Companion | `http://localhost:18085` |
| Console   | `http://localhost:13001` |

Variables: ver `.env.example`.

## Tests

```bash
make test          # Go unit (review + companion)
make smoke         # Requiere APIs levantadas (compose)
```

Smoke incluye flujo **Companion → Nexus Governance** (`scripts/smoke/run-companion-review-flow.sh`): comprueba el vínculo `review_request_id`, que el **estado de la tarea** coincida con el resultado de governance (`allowed` → `done`, `pending_approval` → `waiting_for_approval`, etc.) y que responda `POST /v1/tasks/{id}/sync`.

## Console local

`console/` requiere **Node 22**. Para alinear local, Docker y CI:

```bash
cd console
nvm use
npm ci
npm run typecheck
npm run build
```

En local (`localhost`), `console` usa proxy same-origin para hablar con `review` y `companion` sin exponer API keys en el bundle del browser. Para acceso remoto/browser con sesión humana, configurá `NEXUS_AUTH_ISSUER_URL` en backend y `VITE_CLERK_PUBLISHABLE_KEY` en `console`.

## Documentación

- [doc/NEXUS_COWORKER_VISION.md](doc/NEXUS_COWORKER_VISION.md) — visión: de capa de control a **compañero de trabajo completo**
- [companion/README.md](companion/README.md)
- [review/README.md](review/README.md)
- [doc/NEXUS_COMPLETION_ROADMAP.md](doc/NEXUS_COMPLETION_ROADMAP.md)
- [doc/NEXUS_ECOSYSTEM_DESIGN.md](doc/NEXUS_ECOSYSTEM_DESIGN.md)
