# RFC: Nexus Review

Estado: Implementado (MVP Q2)
Autor: Pablo Cristo
Fecha: 19 de marzo de 2026
Version: 5.0

## 1. Resumen

Nexus Review recibe requests, decide (allow / deny / require_approval), guarda todo, y aprende del uso para mejorar las decisiones.

Una request puede venir de cualquier origen: un agente IA, un servicio, o un humano. Nexus no sabe ni le importa. Evalua la request contra politicas (CEL), clasifica riesgo, y produce una decision. Si requiere aprobacion humana, genera un resumen con IA para que el aprobador decida en segundos. Todo queda registrado. Con el tiempo, Nexus detecta patrones y sugiere nuevas reglas.

### Tres pilares

1. **Decidir** — Evaluar requests y producir allow / deny / require_approval.
2. **Registrar** — Guardar el ciclo completo de cada request (propuesta, evaluacion, decision, aprobacion, resultado).
3. **Aprender** — Detectar patrones, sugerir politicas, ajustar. El sistema mejora con el uso.

### Wedge inicial: Monitoring / incident response

El engine es generico. La primera superficie de venta cubre requests sobre alertas, incidentes y runbooks (PagerDuty, OpsGenie, Datadog). Pero el producto no esta atado a ese dominio.

### Ejemplos de requests

| Request | Origen | Riesgo tipico |
|---------|--------|---------------|
| Silenciar alerta critica por 4 horas | ops-bot (agente) | high |
| Escalar alerta a equipo on-call | ops-bot (agente) | low |
| Ejecutar runbook restart-api-gateway | deploy-service (servicio) | high |
| Resolver incidente INC-2847 | sre@company.dev (humano) | medium |
| Crear incidente por anomalia detectada | triage-bot (agente) | medium |

### Diferenciadores

1. **Cascade risk scoring.** 6 factores independientes con amplificacion multiplicativa. Determinista, explicable, sin ML. Inspirado en la cascada de coagulacion.
2. **Feedback loop.** Los resultados de ejecucion (exito/fallo) retroalimentan el factor F5 del cascade. El sistema se calibra solo con el uso.
3. **Contexto con IA.** No datos crudos — un resumen que explica que pide la request, por que se freno, y que recomienda Nexus. Decision en 10 segundos.
4. **Sandbox completo.** Simulate (dry-run) + shadow policies (evaluan sin actuar) + replay test (probar CEL contra historial).
5. **Break-glass.** Aprobacion multi-aprobador para operaciones criticas. Un rechazo cancela todo.
6. **Learning loop.** "Aprobaste 94% de alert.escalate la semana pasada. Queres auto-aprobar?" El sistema propone reglas.
7. **Ontologia tipada.** Action types como ciudadanos de primer nivel con schema, riesgo y metadata. Accion desconocida = 403 FORBIDDEN.
8. **Delegation graph.** Autoridad delegada explicita: owner → agente → action_types → recursos → TTL. Agente sin delegacion = 403 FORBIDDEN.

## 2. Scope: PoC

### Lo que hace el PoC

```
Request llega (de agente, servicio, o humano)
  → CEL evalua politicas → decision: allow / deny / require_approval
  → Cascade risk scoring (6 factores + amplificacion multiplicativa)
  → Si require_approval: Claude genera resumen contextualizado
  → Aprobador decide en el inbox (con confirmacion obligatoria)
  → Requester recibe resultado
  → Todo queda registrado (audit trail)
  → Nexus analiza patrones y sugiere nuevas politicas
  → Config module: todos los valores configurables via API + UI
```

| Componente | En el PoC | Por que |
|------------|-----------|---------|
| CEL policy engine | Si | Sin esto no hay evaluacion |
| Cascade risk scoring (6 factores) | Si | Evaluacion multi-factor con amplificacion multiplicativa |
| Feedback loop (execution → risk) | Si | Resultados de ejecucion retroalimentan F5 del cascade |
| Shadow policies (enforced/shadow) | Si | Evaluar policies sin actuar, con shadow_hits y promote |
| Break-glass (multi-aprobador) | Si | Operaciones criticas requieren N aprobadores |
| Audit trail append-only | Si | Sin esto no hay replay ni learning |
| Approval workflow | Si | Sin esto no hay producto |
| AI contextualizer | Si | Es el diferenciador |
| AI policy proposals (learning) | Si | Es el tercer pilar |
| Sandbox (simulate + shadow + replay test) | Si | Entorno de pruebas completo |
| Config module | Si | Todos los parametros configurables via API + UI |
| Idempotency-Key | Si | Requests duplicadas son criticas |
| API key auth | Si | Minimo para operar |
| Hexagonal (ports & adapters) | Si | Base de calidad |
| Ontologia tipada (action types) | Si | Acciones como tipos de primer nivel con schema, riesgo y metadata |
| Delegation graph | Si | Autoridad delegada: owner → agent → action_types → resources → TTL |
| Console UI (9 tabs) | Si | Sin UI no hay demo |
| i18n (EN + ES) | Si | Soporte bilingue con persistencia localStorage |
| Confirmacion segura (nota + APPROVE/REJECT) | Si | Previene clicks accidentales |

### Lo que NO hace el PoC

- No ejecuta la accion (el requester ejecuta y reporta resultado)
- No tiene leases
- No tiene multi-tenancy
- No tiene billing
- No tiene webhooks de notificacion (polling)
- No tiene background job de expiracion (expira en query time)

## 3. Arquitectura

### Principio: monolito modular, hexagonal

Un solo servicio Go. Modulos con ports & adapters. El wire conecta todo.

```
Requester (agente / servicio / humano)
        |
        v
  Nexus Review API (un solo servicio Go)
  ┌──────────────────────────────────────────────────────┐
  │                                                      │
  │  internal/requests/         (modulo principal)       │
  │  ├── usecases/domain/       entidades de dominio    │
  │  ├── usecases.go            logica + ports + opts   │
  │  ├── handler.go             adapter: HTTP           │
  │  ├── handler/dto/           DTOs                    │
  │  ├── repository.go          port + pgx impl         │
  │  ├── policy_evaluator.go    CEL evaluation          │
  │  ├── risk.go                cascade risk scoring    │
  │  ├── execution_stats.go     feedback loop adapter   │
  │  ├── ai_contextualizer.go   port+adapter: Claude    │
  │  └── audit_sink.go          emision de eventos      │
  │                                                      │
  │  internal/policies/         (CRUD + shadow mode)    │
  │  internal/approvals/        (inbox + break-glass)   │
  │  internal/audit/            (trail + replay)        │
  │  internal/learning/         (pattern detection +    │
  │                              policy proposals)      │
  │  internal/dashboard/        (metricas agregadas)    │
  │  internal/config/           (configuracion global)  │
  │  internal/actiontypes/      (ontologia tipada)      │
  │  internal/delegations/      (delegation graph)      │
  │                                                      │
  │  wire/setup.go              (DI manual)             │
  │  cmd/api/main.go            (entry point)           │
  │                                                      │
  └──────────────────────────────────────────────────────┘
        │              │
        v              v
   PostgreSQL     Claude API
```

### Estructura de directorios

```
v3/
├── review/                            # servicio Go (backend)
│   ├── cmd/api/main.go
│   ├── internal/
│   │   ├── requests/                  # modulo principal
│   │   │   ├── usecases/domain/entities.go
│   │   │   ├── usecases.go
│   │   │   ├── handler.go
│   │   │   ├── handler/dto/dto.go
│   │   │   ├── repository.go
│   │   │   ├── policy_evaluator.go
│   │   │   ├── risk.go               # cascade risk scoring (6 factores)
│   │   │   ├── execution_stats.go   # feedback loop (execution → risk F5)
│   │   │   ├── ai_contextualizer.go
│   │   │   ├── ai_contextualizer/types.go
│   │   │   └── audit_sink.go
│   │   ├── policies/                  # CRUD (7 ops)
│   │   ├── approvals/                 # inbox + decisiones
│   │   ├── audit/                     # trail + replay
│   │   ├── learning/                  # patrones + propuestas
│   │   ├── dashboard/                 # metricas
│   │   ├── config/                    # configuracion global via API
│   │   ├── actiontypes/              # ontologia tipada (CRUD 5 ops)
│   │   ├── delegations/              # delegation graph (CRUD 5 ops)
│   │   └── shared/                    # codigo transversal
│   ├── wire/setup.go
│   ├── migrations/
│   ├── Dockerfile
│   └── go.mod
├── console/                           # frontend (React + Tailwind)
│   ├── src/
│   ├── Dockerfile
│   └── nginx.conf
├── ../../../core/                     # capacidades compartidas externas a nexus
├── scripts/
│   ├── lib/common.sh
│   ├── quality/check-api.sh
│   ├── smoke/run-policies-crud.sh
│   ├── smoke/run-requests-flow.sh
│   └── e2e/run-full-lifecycle.sh
├── doc/
├── docker-compose.yml
├── .env.example
└── Makefile
```

### Patron hexagonal por modulo

```
usecases/domain/entities.go    # Entidades puras (sin deps HTTP ni DB)
usecases.go                    # Logica + ports (interfaces) + functional options
repository.go                  # Interface + sentinel errors + implementacion pgx
handler.go                     # Adapter: net/http (recibe interface, no *Usecases)
handler/dto/dto.go             # DTOs (separados del dominio)
```

PostgreSQL siempre — no hay repositorios in-memory. Un solo archivo `repository.go` por modulo.

### Reutilizacion de `core`

| Paquete | Uso |
|---------|-----|
| `core/backend/go/httpjson` | DecodeJSON, WriteJSON, health endpoints |
| `core/databases/postgres/go` | pgxpool, MigrateUp, ConfigFromEnv |
| `core/backend/go/apikey` | Auth SHA256 middleware |
| `core/backend/go/httpserver` | Security headers, CORS, graceful shutdown |
| `core/backend/go/observability` | slog JSON, Prometheus, RED middleware |

## 4. Data Model

### 4.1 requests (tabla principal)

```sql
CREATE TABLE requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key TEXT UNIQUE,
    requester_type  TEXT NOT NULL,
    requester_id    TEXT NOT NULL,
    requester_name  TEXT,
    action_type     TEXT NOT NULL,
    target_system   TEXT,
    target_resource TEXT,
    params          JSONB NOT NULL DEFAULT '{}',
    reason          TEXT,
    context         TEXT,
    risk_level      TEXT NOT NULL DEFAULT 'low',
    decision        TEXT NOT NULL,
    decision_reason TEXT,
    policy_id       UUID REFERENCES policies(id),
    status          TEXT NOT NULL DEFAULT 'pending',
    approval_id     UUID,
    execution_result JSONB,
    error_message    TEXT,
    ai_summary       TEXT,
    ai_degraded      BOOLEAN NOT NULL DEFAULT false,
    evaluated_at    TIMESTAMPTZ,
    decided_at      TIMESTAMPTZ,
    executed_at     TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.2 policies

```sql
CREATE TABLE policies (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,
    action_type TEXT,
    target_system TEXT,
    expression  TEXT NOT NULL,       -- CEL
    effect      TEXT NOT NULL,       -- allow, deny, require_approval
    risk_override TEXT,
    priority    INT NOT NULL DEFAULT 100,
    origin      TEXT NOT NULL DEFAULT 'manual',
    proposal_id UUID,
    mode        TEXT NOT NULL DEFAULT 'enforced', -- enforced / shadow
    shadow_hits INTEGER NOT NULL DEFAULT 0,
    enabled     BOOLEAN NOT NULL DEFAULT true,
    archived_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.3 approvals

```sql
CREATE TABLE approvals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      UUID NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    decided_by      TEXT,
    decision_note   TEXT,
    decided_at      TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ NOT NULL,
    break_glass     BOOLEAN NOT NULL DEFAULT false,
    required_approvals INTEGER NOT NULL DEFAULT 1,
    decisions       JSONB NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.4 request_events (audit trail / replay)

Append-only. No UPDATE, no DELETE.

```sql
CREATE TABLE request_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      UUID NOT NULL,
    event_type      TEXT NOT NULL,
    actor_type      TEXT NOT NULL,
    actor_id        TEXT NOT NULL,
    summary         TEXT NOT NULL,
    data            JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.5 policy_proposals (learning)

```sql
CREATE TABLE policy_proposals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proposed_name   TEXT NOT NULL,
    proposed_description TEXT,
    proposed_expression TEXT NOT NULL,
    proposed_effect TEXT NOT NULL,
    proposed_action_type TEXT,
    proposed_priority INT NOT NULL DEFAULT 100,
    pattern_summary TEXT NOT NULL,
    confidence      FLOAT NOT NULL,
    sample_size     INT NOT NULL,
    time_window     TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    decided_by      TEXT,
    decided_at      TIMESTAMPTZ,
    policy_id       UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.6 idempotency_keys

```sql
CREATE TABLE idempotency_keys (
    key             TEXT PRIMARY KEY,
    request_id      UUID NOT NULL,
    response        JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL
);
```

### 4.7 execution_stats (feedback loop)

```sql
CREATE TABLE execution_stats (
    action_type     TEXT PRIMARY KEY,
    success_count   INTEGER NOT NULL DEFAULT 0,
    failure_count   INTEGER NOT NULL DEFAULT 0,
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.8 action_types (ontologia tipada)

```sql
CREATE TABLE action_types (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                 TEXT NOT NULL UNIQUE,
    description          TEXT NOT NULL DEFAULT '',
    category             TEXT NOT NULL DEFAULT '',
    risk_class           TEXT NOT NULL DEFAULT 'low',
    schema               JSONB NOT NULL DEFAULT '{}',
    reversible           BOOLEAN NOT NULL DEFAULT true,
    requires_break_glass BOOLEAN NOT NULL DEFAULT false,
    enabled              BOOLEAN NOT NULL DEFAULT true,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

9 action types seeded: alert.silence, alert.escalate, runbook.execute, incident.resolve, config.update, deploy.trigger, delete, iam.grant_role, treasury.transfer.

### 4.9 delegations (delegation graph)

```sql
CREATE TABLE delegations (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id             TEXT NOT NULL,
    owner_type           TEXT NOT NULL DEFAULT 'user',
    agent_id             TEXT NOT NULL,
    agent_type           TEXT NOT NULL DEFAULT 'agent',
    allowed_action_types JSONB NOT NULL DEFAULT '[]',
    allowed_resources    JSONB NOT NULL DEFAULT '[]',
    purpose              TEXT NOT NULL DEFAULT '',
    max_risk_class       TEXT NOT NULL DEFAULT 'high',
    expires_at           TIMESTAMPTZ,
    enabled              BOOLEAN NOT NULL DEFAULT true,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Diagrama de relaciones

```
requests ──1:N──> request_events
    │
    ├──0:1──> approvals (con break_glass + decisions JSONB)
    ├──0:1──> policies (policy que matcheo)
    └──0:1──> idempotency_keys

policy_proposals ──0:1──> policies (si fue aceptada)

execution_stats ──por action_type── (alimentada por POST /v1/requests/{id}/result)

action_types ──por name── requests.action_type (verificacion en Submit)
delegations  ──por agent_id── requests.requester_id (verificacion en Submit)
```

## 5. API

### 5.1 Enviar request

```
POST /v1/requests
Header: X-API-Key: ...
Header: Idempotency-Key: ... (opcional)
```

### 5.2 Reportar resultado

```
POST /v1/requests/{id}/result
```

### 5.3 Consultar estado

```
GET /v1/requests/{id}
```

### 5.4 Listar requests

```
GET /v1/requests?status=pending_approval&action_type=alert.silence&limit=20
```

### 5.5 Approval inbox

```
GET /v1/approvals/pending
POST /v1/approvals/{id}/approve
POST /v1/approvals/{id}/reject
```

### 5.6 Replay

```
GET /v1/requests/{id}/replay
```

### 5.7 Policies CRUD (7 operaciones canonicas)

```
POST   /v1/policies                    -- 201
GET    /v1/policies                    -- 200
GET    /v1/policies/{id}               -- 200
PATCH  /v1/policies/{id}               -- 200
DELETE /v1/policies/{id}               -- 204
POST   /v1/policies/{id}/archive       -- 204
POST   /v1/policies/{id}/restore       -- 204
```

### 5.8 Learning: policy proposals

```
GET    /v1/learning/proposals
GET    /v1/learning/proposals/{id}
POST   /v1/learning/proposals/{id}/accept
POST   /v1/learning/proposals/{id}/dismiss
POST   /v1/learning/analyze
```

### 5.9 Simulation (dry-run + replay)

```
POST /v1/requests/simulate
POST /v1/requests/simulate/replay
```

`simulate` evalua una request sin persistirla. Retorna la decision, los factores de la cascada de riesgo, y la amplificacion aplicada. Util para probar politicas antes de enviar requests reales.

`simulate/replay` toma una expresion CEL propuesta y la evalua contra el historial de requests existentes. Retorna cuantas habrian matcheado y con que efecto. Util para validar una policy antes de activarla.

### 5.10 Dashboard

```
GET /v1/metrics/summary?period=7d
```

### 5.11 Config

```
GET    /v1/config                    -- 200 (toda la configuracion)
PATCH  /v1/config                    -- 200 (actualizar multiples secciones)
PATCH  /v1/config/{section}          -- 200 (actualizar una seccion)
POST   /v1/config/reset              -- 200 (restaurar defaults)
```

Secciones configurables: risk, approvals, learning, AI, general. Todos los valores son modificables via API y via la UI de la consola (tab Config).

### 5.12 Action Types (ontologia tipada)

```
POST   /v1/action-types                -- 201
GET    /v1/action-types                -- 200
GET    /v1/action-types/{id}           -- 200
PATCH  /v1/action-types/{id}           -- 200
DELETE /v1/action-types/{id}           -- 204
```

Campos: name (unico), description, category, risk_class (low/medium/high/critical), schema (JSON), reversible, requires_break_glass, enabled.

Integrado en Submit: si `action_type` no esta registrado → 403 FORBIDDEN.

### 5.13 Delegations (delegation graph)

```
POST   /v1/delegations                -- 201
GET    /v1/delegations                -- 200
GET    /v1/delegations/{id}           -- 200
PATCH  /v1/delegations/{id}           -- 200
DELETE /v1/delegations/{id}           -- 204
```

Campos: owner_id, owner_type, agent_id, agent_type, allowed_action_types (JSON array), allowed_resources (JSON array), purpose, max_risk_class, expires_at, enabled.

Integrado en Submit: si el agente no tiene delegacion vigente para la accion → 403 FORBIDDEN.

### 5.14 Health

```
GET /healthz
GET /readyz
```

## 6. Policy Engine

### CEL (Google Common Expression Language)

Variables disponibles:
```
request.action_type, request.target_system, request.target_resource
request.params, request.reason, request.context
request.requester_type, request.requester_id
time.hour (0-23 UTC), time.day_of_week (0-6)
```

### Evaluacion

1. Politicas activas (enabled=true, archived_at IS NULL)
2. Filtrar por scope (action_type, target_system)
3. Ordenar por priority (menor = mayor)
4. First-match-wins
5. Si ninguna matchea: decision por riesgo default

### Cascade Risk Scoring (inspirado en cascada de coagulacion)

El riesgo se evalua mediante 6 factores independientes con amplificacion multiplicativa por combinaciones sospechosas. Cada factor produce un score parcial; la suma se multiplica por un factor de amplificacion cuando ciertos factores coinciden.

**6 factores:**

| Factor | Que evalua | Score max |
|--------|-----------|-----------|
| `action_type` | Riesgo base de la accion (high/medium/low) | 0.4 |
| `off_hours` | Fuera de horario laboral (antes 9h, despues 18h) | 0.2 |
| `actor_unknown` | Actor sin historial o con pocas requests previas | 0.3 |
| `frequency_anomaly` | Demasiadas requests del mismo tipo en la ultima hora | 0.3 |
| `execution_history` | Tasa de exito/fallo historica (alimentada por `execution_stats` via feedback loop) | 0.3 (-0.15 si excelente) |
| `target_sensitivity` | Sistema destino es produccion o staging | 0.3 |

**Amplificaciones (combinaciones que se potencian):**

| Combinacion | Multiplicador | Razon |
|-------------|---------------|-------|
| off_hours + actor_unknown | 1.8x | Fuera de horario + desconocido = sospechoso |
| action_type + frequency_anomaly | 1.5x | Accion riesgosa + frecuencia anomala |
| actor_unknown + target_sensitivity | 1.6x | Desconocido atacando prod |
| off_hours + actor_unknown + frequency_anomaly | 2.5x | Cascada completa |
| action_type + off_hours + target_sensitivity | 2.0x | Accion peligrosa + off-hours + prod |
| 4+ factores activos | 2.5x | Amplificacion maxima |

Cap maximo de amplificacion: 3.0x.

**Umbrales de score final → decision:**

| Score | Nivel | Decision |
|-------|-------|----------|
| < 0.5 | low | allow |
| 0.5 — 1.0 | medium | allow |
| 1.0 — 1.5 | medium | allow |
| 1.5 — 2.0 | high | require_approval |
| >= 2.0 | critical | deny |

**Decision por policy + riesgo (compatibilidad):**

| Policy effect | Risk | Decision |
|---------------|------|----------|
| deny | * | deny |
| require_approval | * | require_approval |
| allow | high | require_approval |
| allow | medium/low | allow |
| (no match) | high | require_approval |
| (no match) | medium/low | allow |

## 7. AI

### Contextualizer (approval)

- **StubContextualizer**: fallback con datos crudos
- **ClaudeContextualizer**: HTTP a `api.anthropic.com/v1/messages`, timeout 5s
- Si falla: retorna resumen crudo + `ai_degraded=true`. Nunca falla la request.

### Proposer (learning)

- **StubProposer**: genera CEL simples por action_type
- Futuro: Claude genera expresiones CEL sofisticadas

### Analyzer (learning)

SQL puro sobre requests: agrupa por action_type, calcula approval rate, detecta patrones con >=50 muestras y >=90% consistencia.

## 8. Stack

| Componente | Tecnologia |
|------------|-----------|
| Backend | Go 1.26, net/http, pgx/v5 |
| Database | PostgreSQL 16 Alpine |
| Policy engine | CEL (google/cel-go) |
| AI | Claude API via HTTP directo |
| Frontend | React + Tailwind (console) |
| Auth | API keys (SHA256) |
| Deployment | Docker Compose |
| Observability | slog JSON + Prometheus |
| Tests | Table-driven, httptest, fakes inline |

## 9. Docker

```
COMPOSE_PROJECT_NAME=nexus-v3

Containers:
  nexus-v3-review-1      (Go backend, :18084)
  nexus-v3-console-1     (React/nginx, :13001)
  nexus-v3-postgres-1    (PostgreSQL, :15434)
```

## 10. Roadmap

### PoC → MVP Q2 (completado)

| Area | PoC | MVP Q2 |
|------|-------------|-----|
| Risk scoring | Cascade 6 factores + amplificacion + feedback loop | — (ya implementado en PoC) |
| Feedback loop | execution_stats alimenta F5 del cascade automaticamente | — (ya implementado en PoC) |
| Shadow policies | mode enforced/shadow, shadow_hits, promote | — (ya implementado en PoC) |
| Break-glass | Multi-aprobador, configurable por action_type + risk | — (ya implementado en PoC) |
| Sandbox | Simulate + shadow monitor + replay test | — (ya implementado en PoC) |
| Config module | API CRUD + UI (5 secciones) | — (ya implementado en PoC) |
| Ontologia tipada | action_type como string libre | ✅ Tabla action_types, 9 seeded, CRUD 5 ops, verificacion en Submit |
| Delegation graph | Sin delegaciones | ✅ Tabla delegations, CRUD 5 ops, verificacion en Submit |
| Console | 7 tabs | ✅ 9 tabs (+ Actions, Agents) |

### MVP Q2 → Siguiente

| Area | Estado actual | Siguiente |
|------|-------------|-----|
| Proposer | Stub (template) | Claude real |
| Evidence packs | No | Export JSON firmado |
| Outcome attestation | No | Attest endpoint + firma |
| Rate limiting | No | Por IP/key |
| Paginacion | Limit simple | Cursor |
| Indices DB | Basicos | Optimizados |
| Validacion CEL | No | Al crear policy |
| Approval TTL | Global | Por policy |
| Claude retry | No | Circuit breaker |
