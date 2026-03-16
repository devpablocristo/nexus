# Nexus v2 MVP Status

Relacionado:

- [README.md](README.md)
- [DEFINITION.md](DEFINITION.md)
- [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md)
- [PRE_PROD.md](PRE_PROD.md)
- [PROD_CHECKLIST.md](PROD_CHECKLIST.md)

Estado actual: cerrado.

## Alcance cerrado

El MVP de `v2` queda cerrado con estos componentes:

- `control-plane`
  - `resources`
  - `action policies`
  - `audit records`
- `data-plane`
  - `actions`
  - `approvals`
  - `leases`
  - `execute`
- `control-workers`
  - `incidents`
  - `alerts`

## Persistencia real

Todos los agregados principales ya tienen persistencia real con PostgreSQL:

- `control-plane/resources`
- `control-plane/policies`
- `control-plane/audit`
- `data-plane/actions`
- `control-workers/incidents`
- `control-workers/alerts`

Reglas vigentes:

- `v2/pkgs/go-pkg/postgres` centraliza el pool y el runner de migrations
- la estrategia de migrations es `up-only`
- cada modulo embebe su SQL numerado
- si la URL de base no esta configurada, cada servicio puede seguir cayendo a repo en memoria para tests y dev rapido

## Auth minima

La autenticacion minima del MVP ya esta activa.

- todos los endpoints de negocio requieren API key
- `/healthz` y `/readyz` quedan libres
- se acepta `X-API-Key`
- tambien se acepta `Authorization: Bearer <key>`

Configuracion:

- cada servicio usa `NEXUS_API_KEYS` para auth inbound
- `data-plane` usa:
  - `NEXUS_CONTROL_PLANE_API_KEY`
  - `NEXUS_CONTROL_WORKERS_API_KEY`
- `control-workers` usa:
  - `NEXUS_CONTROL_PLANE_API_KEY`

Semantica:

- auth inter-servicio ya no es anonima
- `control-plane` acepta key admin y keys de servicio
- `control-workers` acepta key admin y key de `data-plane`
- `data-plane` acepta key admin

## Audit

La regla de `audit` queda fijada asi:

- el write inter-servicio es `best effort`
- si falla, no revierte la operacion principal
- el fallo se loguea de forma estructurada
- nunca se descarta en silencio

Tambien queda cubierto:

- cambios admin de `resources`
- cambios admin de `policies`
- lifecycle de `actions`
- `incidents`
- `alerts`

## Verificacion de cierre

El MVP se considera cerrado porque ya se verifico:

- `make milestone`
- `go test ./...`
- `go test -race ./...`
- `golangci-lint run ./...`
- smoke y e2e con auth activa
- `docker compose up -d --build --wait`
- persistencia real tras restart para:
  - `resources`
  - `policies`
  - `actions`
  - `incidents`
  - `alerts`
  - `audit`

## Lo que no entra en el MVP

Se mantiene explicitamente fuera de este cierre:

- `ai-runtime`
- tenants / orgs
- auth avanzada
- RBAC
- playbooks
- persistencia del legacy ya retirado
- nuevas features de dominio

## Siguiente etapa

Con el MVP cerrado, la siguiente etapa inmediata no es dominio nuevo.

Primero sigue:

- [PRE_PROD.md](PRE_PROD.md)

Y una vez completado eso, sigue el gate final de:

- [PROD_CHECKLIST.md](PROD_CHECKLIST.md)

Recién despues de esos dos gates tiene sentido abrir:

- `playbooks`
- `ai-runtime`
- identidad y ownership mas ricos
- observabilidad y superficie operativa mas completa
