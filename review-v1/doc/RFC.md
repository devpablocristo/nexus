# RFC: Nexus Review v1

Estado: Draft
Autor: Pablo Cristo
Fecha: 17 de marzo de 2026
Version: 3.0

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
  → Aprobador decide en el inbox
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
| Approval inbox UI | Si | Sin UI no hay demo |
| Replay UI | Si | Cierra el ciclo |

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

Un solo servicio Go. Modulos con ports & adapters, identicos a Nexus v2. El wire conecta todo.

```
Requester (agente / servicio / humano)
        |
        v
  Nexus Review API (un solo servicio Go)
  ┌──────────────────────────────────────────────────────┐
  │                                                      │
  │  internal/requests/         (modulo principal)       │
  │  ├── usecases/domain/       entidades de dominio    │
  │  ├── usecases.go            logica + ports          │
  │  ├── handler.go             adapter: HTTP           │
  │  ├── handler/dto/           DTOs                    │
  │  ├── repository.go          port: persistencia      │
  │  ├── repository_postgres.go adapter: PostgreSQL     │
  │  ├── policy_evaluator.go    CEL evaluation          │
  │  ├── risk.go                risk tiering            │
  │  ├── ai_contextualizer.go   port+adapter: Claude    │
  │  ├── audit.go               emision de eventos      │
  │  └── metrics.go             Prometheus              │
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
review-v1/
├── cmd/api/
│   └── main.go
├── internal/
│   ├── requests/                       # Modulo principal: requests
│   │   ├── usecases/domain/
│   │   │   └── entities.go             # Request, RequestStatus, Decision, RiskLevel, RequesterType
│   │   ├── usecases.go                 # Ports: PolicyEvaluator, AIContextualizer, LearningEngine
│   │   ├── handler.go                  # POST /v1/requests, GET, etc.
│   │   ├── handler/dto/
│   │   │   └── dto.go
│   │   ├── repository.go
│   │   ├── repository_postgres.go
│   │   ├── policy_evaluator.go
│   │   ├── risk.go
│   │   ├── ai_contextualizer.go
│   │   ├── audit.go
│   │   ├── metrics.go
│   │   └── *_test.go
│   │
│   ├── policies/                       # CRUD de politicas CEL
│   │   ├── usecases/domain/entities.go
│   │   ├── usecases.go
│   │   ├── handler.go
│   │   ├── repository.go
│   │   └── repository_postgres.go
│   │
│   ├── approvals/                      # Inbox + decisiones humanas
│   │   ├── usecases/domain/entities.go
│   │   ├── usecases.go
│   │   ├── handler.go
│   │   ├── repository.go
│   │   └── repository_postgres.go
│   │
│   ├── audit/                          # Trail inmutable + replay
│   │   ├── usecases/domain/entities.go
│   │   ├── usecases.go
│   │   ├── handler.go
│   │   ├── repository.go
│   │   └── repository_postgres.go
│   │
│   ├── learning/                       # Pattern detection + policy proposals
│   │   ├── usecases/domain/entities.go # Proposal, Pattern, Insight
│   │   ├── usecases.go                 # AnalyzePatterns(), GenerateProposals()
│   │   ├── handler.go                  # GET /v1/learning/proposals, POST accept/dismiss
│   │   ├── analyzer.go                 # Port+Adapter: pattern detection
│   │   ├── proposer.go                 # Port+Adapter: Claude genera propuestas
│   │   ├── repository.go
│   │   └── repository_postgres.go
│   │
│   └── dashboard/
│       ├── usecases.go
│       ├── handler.go
│       └── repository_postgres.go
│
├── wire/setup.go
├── migrations/0001_initial.up.sql
├── web/                                # React (approval inbox, replay, proposals, dashboard)
├── doc/
├── go.mod
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

### Patron hexagonal por modulo

```
usecases/domain/entities.go    # Entidades puras
usecases.go                    # Logica + ports (interfaces)
repository.go                  # Port: persistencia
repository_postgres.go         # Adapter: pgx
handler.go                     # Adapter: net/http
handler/dto/dto.go             # DTOs (separados del dominio)
```

### Reutilizacion de pkgs/go-pkg de v2

| Paquete | Uso |
|---------|-----|
| `pkgs/go-pkg/handlers` | CRUD patterns, pagination |
| `pkgs/go-pkg/postgres` | pgxpool, migrations |
| `pkgs/go-pkg/apikey` | Auth SHA256 |
| `pkgs/go-pkg/httpserver` | Security headers, graceful shutdown, health |
| `pkgs/go-pkg/observability` | slog JSON, Prometheus, RED middleware |

## 4. Data Model

### 4.1 requests (tabla principal)

```sql
CREATE TABLE requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key TEXT UNIQUE,

    -- Quien pide
    requester_type  TEXT NOT NULL,               -- agent, service, human
    requester_id    TEXT NOT NULL,               -- "ops-bot", "deploy-service", "sre@company.dev"
    requester_name  TEXT,                        -- nombre legible

    -- Que pide
    action_type     TEXT NOT NULL,               -- "alert.silence", "runbook.execute", o cualquier string
    target_system   TEXT,                        -- "pagerduty", "datadog", "internal"
    target_resource TEXT,                        -- "CPU-CRITICAL-PROD-DB-01", "INC-2847"
    params          JSONB NOT NULL DEFAULT '{}',
    reason          TEXT,
    context         TEXT,                        -- contexto libre del requester

    -- Evaluacion
    risk_level      TEXT NOT NULL DEFAULT 'low', -- low, medium, high
    decision        TEXT NOT NULL,               -- allow, require_approval, deny
    decision_reason TEXT,
    policy_id       UUID REFERENCES policies(id),

    -- Estado
    status          TEXT NOT NULL DEFAULT 'pending',
    -- pending → evaluated → allowed|denied|pending_approval → approved|rejected|expired → executed|failed

    -- Aprobacion
    approval_id     UUID REFERENCES approvals(id),

    -- Resultado
    execution_result JSONB,
    error_message    TEXT,

    -- AI
    ai_summary       TEXT,
    ai_degraded      BOOLEAN NOT NULL DEFAULT false,

    -- Timestamps
    evaluated_at    TIMESTAMPTZ,
    decided_at      TIMESTAMPTZ,
    executed_at     TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_requests_requester ON requests(requester_type, requester_id, created_at DESC);
CREATE INDEX idx_requests_status ON requests(status, created_at DESC);
CREATE INDEX idx_requests_action ON requests(action_type, created_at DESC);
CREATE INDEX idx_requests_decision ON requests(decision, created_at DESC);
```

### 4.2 policies

```sql
CREATE TABLE policies (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,

    -- Scope (null = aplica a todos)
    action_type TEXT,
    target_system TEXT,

    -- Regla
    expression  TEXT NOT NULL,                  -- CEL
    effect      TEXT NOT NULL,                  -- allow, deny, require_approval
    risk_override TEXT,

    -- Orden
    priority    INT NOT NULL DEFAULT 100,       -- menor = mayor prioridad

    -- Origen
    origin      TEXT NOT NULL DEFAULT 'manual', -- manual, learned (propuesta por IA y aceptada)
    proposal_id UUID REFERENCES policy_proposals(id),

    -- Estado
    enabled     BOOLEAN NOT NULL DEFAULT true,
    archived_at TIMESTAMPTZ,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_policies_active ON policies(enabled, priority) WHERE archived_at IS NULL;
```

### 4.3 approvals

```sql
CREATE TABLE approvals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      UUID NOT NULL REFERENCES requests(id),

    status          TEXT NOT NULL DEFAULT 'pending', -- pending, approved, rejected, expired
    decided_by      TEXT,
    decision_note   TEXT,
    decided_at      TIMESTAMPTZ,

    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_approvals_pending ON approvals(status, expires_at) WHERE status = 'pending';
```

### 4.4 request_events (audit trail / replay)

Append-only. No UPDATE, no DELETE.

```sql
CREATE TABLE request_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      UUID NOT NULL REFERENCES requests(id),

    event_type      TEXT NOT NULL,
    -- received, evaluated, allowed, denied, sent_to_approval, approved, rejected,
    -- expired, executed, execution_failed, cancelled

    actor_type      TEXT NOT NULL,               -- requester, system, human
    actor_id        TEXT NOT NULL,

    summary         TEXT NOT NULL,
    data            JSONB NOT NULL DEFAULT '{}',

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_request_events_request ON request_events(request_id, created_at);
```

### 4.5 policy_proposals (learning)

Propuestas de politicas generadas por IA a partir de patrones.

```sql
CREATE TABLE policy_proposals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Que propone
    proposed_name   TEXT NOT NULL,
    proposed_description TEXT,
    proposed_expression TEXT NOT NULL,           -- CEL
    proposed_effect TEXT NOT NULL,               -- allow, deny, require_approval
    proposed_action_type TEXT,
    proposed_priority INT NOT NULL DEFAULT 100,

    -- Por que
    pattern_summary TEXT NOT NULL,               -- "94% de alert.escalate fueron aprobadas en los ultimos 14 dias"
    confidence      FLOAT NOT NULL,              -- 0.0 a 1.0
    sample_size     INT NOT NULL,                -- cantidad de requests analizadas
    time_window     TEXT NOT NULL,               -- "14d", "7d", "30d"

    -- Estado
    status          TEXT NOT NULL DEFAULT 'pending', -- pending, accepted, dismissed, expired
    decided_by      TEXT,
    decided_at      TIMESTAMPTZ,
    policy_id       UUID REFERENCES policies(id),-- si fue aceptada, la policy creada

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_proposals_pending ON policy_proposals(status) WHERE status = 'pending';
```

### 4.6 idempotency_keys

```sql
CREATE TABLE idempotency_keys (
    key             TEXT PRIMARY KEY,
    request_id      UUID NOT NULL REFERENCES requests(id),
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

### 5.1 Enviar request (requester → Nexus)

```
POST /v1/requests
Header: X-API-Key: nxr_...
Header: Idempotency-Key: ... (opcional)
```

Request:
```json
{
  "requester_type": "agent",
  "requester_id": "ops-bot",
  "action_type": "alert.silence",
  "target_system": "pagerduty",
  "target_resource": "CPU-CRITICAL-PROD-DB-01",
  "params": {
    "duration_minutes": 240,
    "silence_type": "maintenance"
  },
  "reason": "Database migration in progress, expected CPU spike",
  "context": "Prod DB-01 CPU at 94%. Planned migration started at 19:00. Alert fired 12 times in the last hour."
}
```

Response:
```json
{
  "request_id": "uuid",
  "decision": "require_approval",
  "risk_level": "high",
  "decision_reason": "Policy 'no-silence-critical-without-approval' requires approval",
  "status": "pending_approval",
  "approval": {
    "id": "uuid",
    "expires_at": "2026-03-17T21:00:00Z"
  },
  "ai_summary": "ops-bot quiere silenciar CPU-CRITICAL-PROD-DB-01 por 4 horas. Hay una migracion de DB en curso que explica el spike (94%). La alerta sono 12 veces en 1 hora. Recomendacion: aprobar con duracion reducida a 3h."
}
```

### 5.2 Reportar resultado

```
POST /v1/requests/{id}/result
```

```json
{
  "success": true,
  "result": {"silence_id": "sil_abc123"},
  "duration_ms": 180
}
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

Response:
```json
{
  "request_id": "uuid",
  "requester": {"type": "agent", "id": "ops-bot"},
  "action_type": "alert.silence",
  "target": "pagerduty / CPU-CRITICAL-PROD-DB-01",
  "final_status": "executed",
  "duration_total": "3m12s",
  "timeline": [
    {
      "event": "received",
      "actor": "ops-bot",
      "at": "2026-03-17T20:00:00Z",
      "summary": "Request received: silence alert CPU-CRITICAL-PROD-DB-01 for 4 hours"
    },
    {
      "event": "evaluated",
      "actor": "nexus",
      "at": "2026-03-17T20:00:01Z",
      "summary": "Risk: high. Policy 'no-silence-critical-without-approval'. Decision: require_approval"
    },
    {
      "event": "sent_to_approval",
      "actor": "nexus",
      "at": "2026-03-17T20:00:02Z",
      "summary": "Sent to approval inbox. Expires at 21:00"
    },
    {
      "event": "approved",
      "actor": "sre-lead@company.dev",
      "at": "2026-03-17T20:03:10Z",
      "summary": "Approved: 'Migration window confirmed, silence OK'"
    },
    {
      "event": "executed",
      "actor": "ops-bot",
      "at": "2026-03-17T20:03:12Z",
      "summary": "Executed successfully. Silence ID: sil_abc123. 180ms"
    }
  ]
}
```

### 5.7 Policies CRUD

```
POST   /v1/policies
GET    /v1/policies
GET    /v1/policies/{id}
PATCH  /v1/policies/{id}
DELETE /v1/policies/{id}
```

### 5.8 Learning: policy proposals

```
GET    /v1/learning/proposals              -- listar propuestas pendientes
GET    /v1/learning/proposals/{id}         -- detalle de una propuesta
POST   /v1/learning/proposals/{id}/accept  -- aceptar (crea la policy)
POST   /v1/learning/proposals/{id}/dismiss -- descartar
```

Ejemplo de propuesta:
```json
{
  "id": "uuid",
  "proposed_name": "auto-approve-alert-escalate",
  "proposed_description": "Auto-approve alert escalations — historically approved 96% of the time",
  "proposed_expression": "request.action_type == 'alert.escalate'",
  "proposed_effect": "allow",
  "pattern_summary": "En los ultimos 14 dias, 96% de las requests 'alert.escalate' fueron aprobadas (274 de 285). Tiempo promedio de aprobacion: 45 segundos. Cero rechazos por riesgo.",
  "confidence": 0.96,
  "sample_size": 285,
  "time_window": "14d",
  "status": "pending"
}
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

## 6. Flows

### 6.1 Auto-allow

```
Requester ──POST /v1/requests──> Nexus
                                    │
                               Validate + idempotency
                                    │
                               Evaluate policies (CEL)
                                    │
                               No deny/approval policy matches
                                    │
                               Risk tiering: low → decision: allow
                                    │
                               Emit events: [received, evaluated, allowed]
                                    │
                               Return {decision: "allow"}
                                    │
Requester <─────────────────────────┘
   │
   └──> Ejecuta la accion, reporta resultado
```

### 6.2 Require approval

```
Requester ──POST /v1/requests──> Nexus
                                    │
                               Policy matches → require_approval
                                    │
                               Create approval (TTL 1h)
                                    │
                               Claude genera resumen (async, best-effort)
                                    │
                               Emit events: [received, evaluated, sent_to_approval]
                                    │
                               Return {decision: "require_approval"}
                                    │
Requester <─────────────────────────┘
   │
   └──> Poll GET /v1/requests/{id}

                    Aprobador abre inbox, ve resumen IA, aprueba
                                    │
                               Emit event: [approved]
                                    │
Requester detecta status=approved
   │
   └──> Ejecuta, reporta resultado
```

### 6.3 Deny

```
Requester ──POST /v1/requests──> Nexus
                                    │
                               Policy matches → deny
                                    │
                               Emit events: [received, evaluated, denied]
                                    │
                               Return {decision: "deny", reason: "..."}
```

### 6.4 Learning loop

```
Background (periodico o on-demand):
   │
   └── Analizar request_events de los ultimos N dias
   │
   └── Detectar patrones:
   │     - action_type con >90% approval rate y >50 samples
   │     - action_type con >80% deny rate
   │     - combinaciones (action_type + target_system) con patron claro
   │
   └── Para cada patron con confidence >0.85:
   │     └── Llamar a Claude: "Dado este patron, genera una propuesta de policy CEL"
   │     └── Guardar en policy_proposals
   │
   └── El usuario ve las propuestas en el inbox de learning
   │     └── Accept → crea policy con origin='learned'
   │     └── Dismiss → marca como descartada
```

## 7. Policy Engine

### CEL (Google Common Expression Language)

```
request.action_type          -- "alert.silence"
request.target_system        -- "pagerduty"
request.target_resource      -- "CPU-CRITICAL-PROD-DB-01"
request.params               -- map
request.reason               -- "Database migration..."
request.context              -- "Prod DB-01 CPU at 94%..."
request.requester_type       -- "agent"
request.requester_id         -- "ops-bot"

time.hour                    -- 14 (UTC)
time.day_of_week             -- 1 (lunes=1)
```

### Evaluacion

1. Politicas activas (enabled=true, archived_at IS NULL)
2. Filtrar por scope (action_type, target_system)
3. Ordenar por priority (menor = mayor)
4. First-match-wins
5. Si ninguna matchea: allow (default open)

### Risk Tiering

| Condicion | Risk |
|-----------|------|
| Policy con risk_override | el override |
| action_type definido como high por config | high |
| action_type definido como medium por config | medium |
| default | low |

| Policy effect | Risk | Decision |
|---------------|------|----------|
| deny | * | deny |
| require_approval | * | require_approval |
| allow | high | require_approval |
| allow | medium/low | allow |
| (no match) | high | require_approval |
| (no match) | medium/low | allow |

## 8. AI

### 8.1 Contextualizer (approval)

Port:
```go
type AIContextualizer interface {
    Summarize(ctx context.Context, input SummarizeInput) (string, error)
}
```

Se invoca solo cuando decision = require_approval. Timeout 5s. Fallback: datos crudos, ai_degraded=true.

### 8.2 Proposer (learning)

Port:
```go
type PolicyProposer interface {
    GenerateProposal(ctx context.Context, pattern Pattern) (*PolicyProposal, error)
}
```

Se invoca cuando el analyzer detecta un patron con confidence >0.85. Claude recibe el patron y genera una expresion CEL + nombre + descripcion.

### 8.3 Analyzer (learning)

No usa IA. Queries SQL puras sobre request_events:

```sql
-- Patron: action_type con alto approval rate
SELECT
    r.action_type,
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE r.status = 'approved') as approved,
    ROUND(COUNT(*) FILTER (WHERE r.status = 'approved')::numeric / COUNT(*)::numeric, 2) as approval_rate
FROM requests r
WHERE r.decision = 'require_approval'
  AND r.created_at > now() - interval '14 days'
GROUP BY r.action_type
HAVING COUNT(*) > 50
   AND COUNT(*) FILTER (WHERE r.status = 'approved')::numeric / COUNT(*)::numeric > 0.90;
```

El analyzer detecta el patron. El proposer (Claude) genera la policy. El humano acepta o descarta.

## 9. Approval Inbox (UI)

```
┌─────────────────────────────────────────────────────────────────┐
│  Nexus Review — Inbox                             [3 pending]   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ● HIGH    alert.silence   CPU-CRITICAL-PROD-DB-01  pagerduty  │
│    "Silenciar alerta critica por 4h — migracion en curso..."   │
│    ops-bot (agent) | Policy: no-silence-critical | 52min left  │
│    [Approve]  [Reject]  [Details]                              │
│                                                                 │
│  ● HIGH    runbook.execute  restart-api-gateway     ops-bot    │
│    "Reiniciar API gateway por memory leak detectado..."        │
│    ops-bot (agent) | Policy: approve-prod-runbooks | 28min     │
│    [Approve]  [Reject]  [Details]                              │
│                                                                 │
│  ● MEDIUM  incident.resolve  INC-2847              triage-bot  │
│    "Cerrar incidente — metricas volvieron a la normalidad..."  │
│    triage-bot (agent) | Policy: review-close | 45min           │
│    [Approve]  [Reject]  [Details]                              │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  Learning Proposals                                [2 new]      │
│                                                                 │
│  💡 "Auto-approve alert.escalate"                              │
│     96% approved in last 14d (274/285). Confidence: 0.96       │
│     [Accept]  [Dismiss]  [Details]                             │
│                                                                 │
│  💡 "Deny runbook.execute on weekends"                         │
│     89% rejected on Sat/Sun in last 30d. Confidence: 0.89     │
│     [Accept]  [Dismiss]  [Details]                             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 10. Stack tecnologico

| Componente | Tecnologia |
|------------|-----------|
| Backend | Go 1.26, net/http |
| Database | PostgreSQL 16, pgx |
| Policy engine | CEL (google/cel-go) |
| AI | Claude API (anthropic-sdk-go) |
| Frontend | React + Tailwind |
| Auth | API keys (SHA256) + JWT (UI) |
| Deployment | Docker Compose (dev), fly.io (prod MVP) |
| Observability | slog + Prometheus |

## 11. Deployment

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: nexus_review

  nexus-review:
    build: .
    ports: ["8080:8080"]
    environment:
      DATABASE_URL: postgres://...
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY}
      PORT: 8080
      APPROVAL_DEFAULT_TTL: 3600
      LEARNING_ANALYSIS_INTERVAL: 86400
```

## 12. Roadmap

### PoC (semanas 1-4)

| Semana | Entregable |
|--------|-----------|
| 1 | Scaffold. Data model. Migrations. Health. Wire. |
| 2 | Requests: POST /v1/requests, policy engine CEL, risk tiering, audit events, idempotency. |
| 3 | Approvals: inbox API, approve/reject. AI contextualizer. Replay API. |
| 4 | Learning: analyzer + proposer + proposals API. UI: inbox + replay + proposals. Demo. |

### v1 completo (semanas 5-10)

| Semana | Entregable |
|--------|-----------|
| 5 | Hash-chained audit. Background job expiracion. DegradationCollector. |
| 6 | Break-glass approval. Rate limiting. Canary requesters. |
| 7 | Dashboard UI completo. Metricas API. |
| 8 | SDK Python. Webhooks de notificacion. |
| 9 | Multi-tenancy basico (org_id). |
| 10 | Polish, smoke tests, landing page, deploy a prod. |

## 13. Decision buscada

1. El core de 3 pilares (decidir, registrar, aprender) es correcto?
2. El modelo generico de "requests" (no atado a agentes) es el enfoque correcto?
3. Learning en el PoC (no post-PoC) es la prioridad correcta?
4. Arrancamos?
