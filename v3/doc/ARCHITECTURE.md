# Nexus v3 — Arquitectura

## Visión general

Nexus v3 es un monorepo de productos separados mientras se estabilizan contratos:

- **Nexus**: governance/control plane. La implementación técnica temporal vive en `review/`.
- **Companion**: empleado digital generalista. Consume Nexus por API.
- **Console**: UI operativa.
- **Connectors**: capacidades operativas internas de Companion.

La regla de frontera es: **Nexus decide, Companion trabaja, Connectors conectan**.

La frontera de confianza se materializa con un principal efectivo (`actor_id`, `org_id`, scopes, método de auth y marca de service principal). Nexus es autoridad de decisión/auditoría; Companion puede operar, pero side effects de connectors solo se ejecutan con request de Nexus `allowed` o `approved`, tenant compatible y resultado reportado.

```
┌─────────────────────────────────────────────────────┐
│                    console/                          │
│              (React + Tailwind, :13001)              │
│  Inbox │ Requests │ Policies │ Actions │ Agents      │
│  │ Sandbox │ Learning │ Dashboard │ Config            │
└────────────────────┬────────────────────────────────┘
                     │ /v1/*
                     ▼
┌─────────────────────────────────────────────────────┐
│              review/ = Nexus                          │
│              (Go, net/http, :18084)                   │
│                                                     │
│  requests ─── policies ─── approvals                │
│      │                         │                    │
│   audit ──── learning ──── dashboard ─── config     │
│      │                                              │
│   actiontypes ─── delegations                       │
│                                                     │
│  wire/setup.go (DI manual)                           │
└────────────────────┬────────────────────────────────┘
                     │ HTTP client
                     ▼
┌─────────────────────────────────────────────────────┐
│                  companion/                          │
│              (Go, net/http, :18085)                  │
│                                                     │
│  tasks ─ memory ─ runtime/chat                       │
│    │        │          │                             │
│  watchers ─┴──── connectors ─── adapters             │
│                         │                           │
│                  mock │ pymes │ futuros             │
└────────────────────┬────────────────────────────────┘
                     │ pgx
                     ▼
┌─────────────────────────────────────────────────────┐
│                  PostgreSQL                          │
│           (1 instancia, N databases)                │
│           nexus_review │ nexus_companion │ ...      │
└─────────────────────────────────────────────────────┘
```

## Principios

1. **Monolito modular** — cada servicio es un binario con módulos internos bien separados por ports & adapters. Microservicios cuando el dolor lo justifique.
2. **Hexagonal por módulo** — usecases (lógica + ports), handlers (HTTP), repositories (PostgreSQL). Cada capa con sus propios tipos.
3. **PostgreSQL siempre** — desarrollo, staging, producción. Sin repos in-memory.
4. **Frontend único** — `console/` sirve a todos los módulos. Un deploy, un login, una experiencia.
5. **Flat structure** — servicios al mismo nivel en v3/. Sin nesting innecesario.
6. **Productos separados, repo único temporal** — no se parte Git hasta que Nexus API, Companion API y el contrato de connectors estén estables.
7. **core/modules son librerías** — no se mueve código allí si no es agnóstico, independiente y versionable para cualquier proyecto del ecosistema Pablo.

## Estructura de v3

```
v3/
├── review/              # implementación técnica temporal de Nexus Governance
│   ├── cmd/api/main.go
│   ├── internal/
│   │   ├── requests/    # módulo principal (CEL, risk, AI, audit, execution_stats)
│   │   ├── policies/    # CRUD 7 ops + shadow mode
│   │   ├── approvals/   # inbox + approve/reject + break-glass
│   │   ├── audit/       # trail append-only + replay
│   │   ├── learning/    # pattern detection + proposals
│   │   ├── dashboard/   # métricas
│   │   ├── config/      # configuración global (5 secciones)
│   │   ├── actiontypes/ # ontología tipada (CRUD 5 ops, 9 seeded)
│   │   ├── delegations/ # delegation graph (CRUD 5 ops)
│   │   └── shared/      # helpers transversales
│   ├── wire/setup.go    # DI manual
│   ├── migrations/
│   ├── Dockerfile
│   └── go.mod
├── companion/           # empleado digital generalista
│   ├── internal/
│   │   ├── tasks/       # trabajo, lifecycle y sync con Nexus
│   │   ├── memory/      # memoria operativa
│   │   ├── runtime/     # chat/orchestration/tools
│   │   ├── watchers/    # observación proactiva
│   │   └── connectors/  # contrato v1 + adapters internos
│   └── migrations/
├── console/             # frontend único (React + Tailwind)
│   ├── src/
│   │   ├── views/       # Inbox, Requests, Policies, Actions, Agents, Sandbox, Learning, Dashboard, Config
│   │   ├── components/  # RiskBadge, StatusBadge
│   │   ├── api.js       # API client
│   │   └── i18n.js      # EN/ES con persistencia
│   ├── nginx.conf       # proxy /v1/* → review
│   └── Dockerfile
├── ../../../core/       # librerías agnósticas externas a nexus
├── ../../../modules/    # librerías/capacidades agnósticas sobre core
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

## Connectors

Connectors pertenecen a Companion en esta etapa. Nexus no contiene adapters ni ejecución externa.

Cada capability declara `operation`, `mode`, `side_effect`, `risk_class`, `requires_review`, `input_schema` y `evidence_fields`. Las operaciones `read` pueden ejecutarse sin approval; las operaciones `write` o con side effects requieren Nexus `allowed` o `approved`.

El contrato solo debe moverse a `core` o `modules` si se vuelve agnóstico, estable y reutilizado fuera de Companion.

## Crecimiento esperado

```
nexus/
├── review/         ← implementación actual de Nexus Governance hasta separar repo
├── companion/      ← producto separado dentro del monorepo
├── console/
└── docker-compose.yml

Futuro, cuando los contratos estén estables:

nexus/              ← repo de governance
companion/          ← repo de empleado digital + connectors
console/            ← repo o app operativa, según necesidad
```
