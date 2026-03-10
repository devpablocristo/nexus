# Engineering Onboarding

## Quickstart local

```bash
cp .env.example .env
make up
make migrate-up
make seed
```

## Orden recomendado para entender el sistema

1. `README.md`
2. `docs/DOC.md`
3. `docs/SERVICE_BOUNDARIES.md`
4. `docs/policy/*`
5. `docs/protocols/*`
6. `docs/data/*` y `docs/events/*`

## Cómo correr por servicio

- `nexus-core`: revisar `Makefile` y `docker-compose.yml`
- `nexus-saas`: mismo patrón que core
- `nexus-ai-operators`: `.venv/bin/pytest` / `uvicorn app.main:app`
- `nexus-tower`: Node 20.19+ , `npm install`, `npm run dev`
- `sdks/typescript-sdk`: Node 20+ para tests/build del SDK

## Dónde vive cada cosa

- contratos: `pkgs/contracts/`
- prompts: `docs/prompts/`
- runbooks: `docs/runbooks/`
- ADRs: `docs/adr/`
- SDKs: `sdks/`
- downloads públicos del portal: `tower/public/downloads/`

## Variables de entorno

No crear forks de lógica por ambiente. Toda diferencia debe entrar por config/env válida al startup.

## Gates rápidos antes de cerrar un cambio

- Si tocás OpenAPI, Postman, docs servidas o assets de developer portal: `make contracts-check`
- Si tocás lógica cross-service: actualizar `docs/*`, `pkgs/contracts/*` y revalidar el gate contractual
