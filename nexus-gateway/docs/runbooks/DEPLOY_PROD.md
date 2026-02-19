# DEPLOY_PROD.md

## Objetivo

Desplegar Nexus Gateway en entorno productivo con observabilidad y controles minimos.

## Prerequisitos

- Docker + Docker Compose
- Dominio/TLS gestionado por reverse proxy externo
- Postgres y Redis accesibles
- Secret `NEXUS_MASTER_KEY` seguro (base64 de 32 bytes)

## Variables criticas

- `NEXUS_DATABASE_URL`
- `NEXUS_MASTER_KEY`
- `NEXUS_AUTH_ENABLE_JWT`
- `NEXUS_JWKS_URL` (si JWT habilitado)
- `NEXUS_RATE_LIMIT_BACKEND=redis`
- `NEXUS_REDIS_URL`
- `NEXUS_DISABLE_SSRF_PROTECTION=false` (obligatorio en prod)

## Procedimiento

1. Preparar `.env` productivo:
```bash
cp .env.example .env
# editar valores productivos
```

2. Levantar stack:
```bash
docker compose up --build -d
```

3. Aplicar migraciones:
```bash
make migrate-up
```

4. Verificar readiness:
```bash
curl -fsS http://localhost:${NEXUS_HTTP_PORT}/readyz
```

5. Seed inicial (solo primer arranque/no prod real):
```bash
make seed
```

6. Validar observabilidad:
```bash
curl -fsS http://localhost:${NEXUS_HTTP_PORT}/metrics | head
```

## SLO operativo inicial

- Availability mensual: **99.9%**
- Error budget mensual: **43m 49s**
- p95 `POST /v1/run` objetivo: **< 800ms** en carga nominal

## Backup/restore (minimo)

### Backup
```bash
docker compose exec -T postgres pg_dump -U postgres nexus > backup_$(date +%F_%H%M).sql
```

### Restore (entorno recovery)
```bash
cat backup_2026-02-18_1200.sql | docker compose exec -T postgres psql -U postgres -d nexus
```

## Release gates obligatorios

```bash
go test ./...
make e2e
make jwt-e2e
```

## Rollback

1. Volver imagen previa de `nexus-gateway`.
2. Si migration incompatible, aplicar `make migrate-down` segun plan.
3. Restaurar backup DB si corresponde.

