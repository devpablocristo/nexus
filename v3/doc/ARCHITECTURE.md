# Nexus v3 — Arquitectura

## Visión general

Nexus v3 es un producto SaaS modular. Cada módulo es un servicio Go independiente, con un frontend único (console) y una base de datos PostgreSQL compartida.

```
┌─────────────────────────────────────────────────────┐
│                    console/                          │
│              (React + Tailwind, :13001)              │
│  Inbox │ Requests │ Policies │ Sandbox │ Learning    │
│  │ Dashboard │ Config                                │
└────────────────────┬────────────────────────────────┘
                     │ /v1/*
                     ▼
┌─────────────────────────────────────────────────────┐
│                    review/                           │
│              (Go, net/http, :18084)                  │
│                                                     │
│  requests ─── policies ─── approvals                │
│      │                         │                    │
│   audit ──── learning ──── dashboard ─── config     │
│                                                     │
│  wire/setup.go (DI manual)                          │
└────────────────────┬────────────────────────────────┘
                     │ pgx
                     ▼
┌─────────────────────────────────────────────────────┐
│                  PostgreSQL                          │
│           (1 instancia, N databases)                │
│           nexus_review │ nexus_billing │ ...        │
└─────────────────────────────────────────────────────┘
```

## Principios

1. **Monolito modular** — cada servicio es un binario con módulos internos bien separados por ports & adapters. Microservicios cuando el dolor lo justifique.
2. **Hexagonal por módulo** — usecases (lógica + ports), handlers (HTTP), repositories (PostgreSQL). Cada capa con sus propios tipos.
3. **PostgreSQL siempre** — desarrollo, staging, producción. Sin repos in-memory.
4. **Frontend único** — `console/` sirve a todos los módulos. Un deploy, un login, una experiencia.
5. **Flat structure** — servicios al mismo nivel en v3/. Sin nesting innecesario.

## Estructura de v3

```
v3/
├── review/              # servicio Go (primer módulo)
│   ├── cmd/api/main.go
│   ├── internal/
│   │   ├── requests/    # módulo principal (CEL, risk, AI, audit, execution_stats)
│   │   ├── policies/    # CRUD 7 ops + shadow mode
│   │   ├── approvals/   # inbox + approve/reject + break-glass
│   │   ├── audit/       # trail append-only + replay
│   │   ├── learning/    # pattern detection + proposals
│   │   ├── dashboard/   # métricas
│   │   ├── config/      # configuración global (5 secciones)
│   │   └── shared/      # helpers transversales
│   ├── wire/setup.go    # DI manual
│   ├── migrations/
│   ├── Dockerfile
│   └── go.mod
├── console/             # frontend único (React + Tailwind)
│   ├── src/
│   │   ├── views/       # Inbox, Requests, Policies, Sandbox, Learning, Dashboard, Config
│   │   ├── components/  # RiskBadge, StatusBadge
│   │   ├── api.js       # API client
│   │   └── i18n.js      # EN/ES con persistencia
│   ├── nginx.conf       # proxy /v1/* → review
│   └── Dockerfile
├── pkgs/go-pkg/         # código Go compartido (agnóstico al proyecto)
│   ├── handlers/        # DecodeJSON, WriteJSON, health
│   ├── postgres/        # pgxpool, MigrateUp
│   ├── apikey/          # auth SHA256 middleware
│   ├── httpserver/      # security headers, CORS, graceful shutdown
│   └── observability/   # slog JSON, Prometheus, RED middleware
├── scripts/
│   ├── lib/common.sh
│   ├── quality/check-api.sh
│   ├── smoke/
│   └── e2e/
├── doc/
├── docker-compose.yml
├── .env.example
└── Makefile
```

## Módulo hexagonal (patrón)

Cada módulo sigue la misma estructura:

```
internal/{modulo}/
    usecases.go                  # lógica + ports (interfaces)
    usecases/domain/entities.go  # tipos de dominio

    handler.go                   # adapter HTTP + mapper (DTO ↔ dominio)
    handler/dto/dto.go           # tipos HTTP

    repository.go                # interface + sentinel errors + impl pgx
    repository/models/models.go  # tipos DB (si difieren del dominio)

    {adapter}.go                 # otros adapters
    {adapter}/                   # tipos del adapter
```

### Reglas clave

- **Accept interfaces, return structs** — constructores reciben interfaces, devuelven `*Struct`
- **Interfaces en el consumidor** — cada adapter define su port con solo los métodos que necesita
- **Tipos por capa** — usecases solo conocen dominio. Handler convierte DTO ↔ dominio. Repository convierte dominio ↔ model.
- **Un solo `repository.go`** — interface + sentinel errors + impl pgx. Sin sufijos.

## Docker

```yaml
COMPOSE_PROJECT_NAME=nexus-v3

nexus-v3-postgres-1    # PostgreSQL 16 Alpine (1 instancia, N databases)
nexus-v3-review-1      # Go backend (:18084)
nexus-v3-console-1     # React/nginx (:13001)
```

Cuando se agregue un nuevo servicio:
1. Crear directorio `v3/{servicio}/` con Go code
2. Crear database `nexus_{servicio}` en la misma instancia postgres
3. Agregar servicio al `docker-compose.yml`
4. Agregar sección al `console/`

## Crecimiento esperado

```
v3/
├── review/         ← actual
├── billing/        ← futuro
├── gateway/        ← futuro (API unificada)
├── workers/        ← futuro (jobs async)
├── console/
├── pkgs/
└── docker-compose.yml
```
