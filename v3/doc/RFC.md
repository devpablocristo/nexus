# RFC: Nexus Review

Estado: Implementado (PoC)
Autor: Pablo Cristo
Fecha: 18 de marzo de 2026
Version: 4.0

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

### Diferenciador

1. **Contexto con IA.** No datos crudos — un resumen que explica que pide la request, por que se freno, y que recomienda Nexus. Decision en 10 segundos.
2. **Replay completo.** Cada request documentada de punta a punta. Reconstruible en postmortems.
3. **Aprende.** "Aprobaste 94% de alert.escalate la semana pasada. Queres auto-aprobar?" El sistema propone reglas.

## 2. Scope: PoC

### Lo que hace el PoC

```
Request llega (de agente, servicio, o humano)
  → CEL evalua politicas → decision: allow / deny / require_approval
  → Si require_approval: Claude genera resumen contextualizado
  → Aprobador decide en el inbox (con confirmacion obligatoria)
  → Requester recibe resultado
  → Todo queda registrado (audit trail)
  → Nexus analiza patrones y sugiere nuevas politicas
```

| Componente | En el PoC | Por que |
|------------|-----------|---------|
| CEL policy engine | Si | Sin esto no hay evaluacion |
| Audit trail append-only | Si | Sin esto no hay replay ni learning |
| Approval workflow | Si | Sin esto no hay producto |
| AI contextualizer | Si | Es el diferenciador |
| AI policy proposals (learning) | Si | Es el tercer pilar |
| Idempotency-Key | Si | Requests duplicadas son criticas |
| API key auth | Si | Minimo para operar |
| Hexagonal (ports & adapters) | Si | Base de calidad |
| Console UI (inbox, policies, replay, learning, dashboard) | Si | Sin UI no hay demo |
| Confirmacion segura (nota + APPROVE/REJECT) | Si | Previene clicks accidentales |

### Lo que NO hace el PoC

- No ejecuta la accion (el requester ejecuta y reporta resultado)
- No tiene leases
- No tiene risk cascade multi-factor (tiering simple)
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
  │  ├── risk.go                risk tiering            │
  │  ├── ai_contextualizer.go   port+adapter: Claude    │
  │  └── audit_sink.go          emision de eventos      │
  │                                                      │
  │  internal/policies/         (CRUD + CEL validation) │
  │  internal/approvals/        (inbox + decisions)     │
  │  internal/audit/            (trail + replay)        │
  │  internal/learning/         (pattern detection +    │
  │                              policy proposals)      │
  │  internal/dashboard/        (metricas agregadas)    │
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
│   │   │   ├── risk.go
│   │   │   ├── ai_contextualizer.go
│   │   │   ├── ai_contextualizer/types.go
│   │   │   └── audit_sink.go
│   │   ├── policies/                  # CRUD (7 ops)
│   │   ├── approvals/                 # inbox + decisiones
│   │   ├── audit/                     # trail + replay
│   │   ├── learning/                  # patrones + propuestas
│   │   ├── dashboard/                 # metricas
│   │   └── shared/                    # codigo transversal
│   ├── wire/setup.go
│   ├── migrations/
│   ├── Dockerfile
│   └── go.mod
├── console/                           # frontend (React + Tailwind)
│   ├── src/
│   ├── Dockerfile
│   └── nginx.conf
├── pkgs/go-pkg/                       # codigo compartido agnostico
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

### Reutilizacion de pkgs/go-pkg

| Paquete | Uso |
|---------|-----|
| `pkgs/go-pkg/handlers` | DecodeJSON, WriteJSON, health endpoints |
| `pkgs/go-pkg/postgres` | pgxpool, MigrateUp, ConfigFromEnv |
| `pkgs/go-pkg/apikey` | Auth SHA256 middleware |
| `pkgs/go-pkg/httpserver` | Security headers, CORS, graceful shutdown |
| `pkgs/go-pkg/observability` | slog JSON, Prometheus, RED middleware |

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

### Diagrama de relaciones

```
requests ──1:N──> request_events
    │
    ├──0:1──> approvals
    ├──0:1──> policies (policy que matcheo)
    └──0:1──> idempotency_keys

policy_proposals ──0:1──> policies (si fue aceptada)
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

### 5.9 Dashboard

```
GET /v1/metrics/summary?period=7d
```

### 5.10 Health

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

### Risk Tiering

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
  nexus-v3-review-1           (Go backend, :18084)
  nexus-v3-console-1          (React/nginx, :13001)
  nexus-v3-review-postgres-1  (PostgreSQL, :15434)
```

## 10. Roadmap

### PoC → MVP

| Area | PoC (actual) | MVP |
|------|-------------|-----|
| Risk config | Hardcodeado | DB o env vars |
| Proposer | Stub (template) | Claude real |
| Analyzer | In-memory | SQL GROUP BY |
| Rate limiting | No | Por IP/key |
| Paginacion | Limit simple | Cursor |
| Indices DB | Basicos | Optimizados |
| Dashboard | Cuenta en Go | SQL aggregation |
| Validacion CEL | No | Al crear policy |
| Approval TTL | Global | Por policy |
| Claude retry | No | Circuit breaker |
