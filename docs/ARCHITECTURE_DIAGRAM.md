# Nexus — Diagrama de arquitectura

Diagrama actualizado con todos los detalles. Colores alineados con la paleta del Excalidraw original para poder replicar o importar.

**Paleta para Excalidraw (hex):**
- Fondo servicios data/control: `#e9ecef` (stroke `#1e1e1e`)
- Pipeline label: `#fff9db` (stroke `#868e96`)
- Control/IA operators: `#b2f2bb` (stroke `#2f9e44`)
- Usuarios/Tower/Consumers: `#d0bfff` (stroke `#7048e8`)
- Postgres: `#a5d8ff` (stroke `#1971c2`)
- Redis: `#ffc9c9` (stroke `#e03131`)
- Clerk/Stripe/SES: `#f3d9fa` (stroke `#9c36b5`)
- Prometheus/Grafana: `#ffe8cc` (stroke `#f08c00`)
- Flechas internas (entitlements, events): `#f08c00`
- Flechas públicas (run): `#2f9e44`

---

```mermaid
flowchart TB
    classDef fillCore fill:#e9ecef,stroke:#1e1e1e
    classDef fillOps fill:#b2f2bb,stroke:#2f9e44
    classDef fillUsers fill:#d0bfff,stroke:#7048e8
    classDef fillDb fill:#a5d8ff,stroke:#1971c2
    classDef fillRedis fill:#ffc9c9,stroke:#e03131
    classDef fillExt fill:#f3d9fa,stroke:#9c36b5
    classDef fillObs fill:#ffe8cc,stroke:#f08c00
    classDef fillPipeline fill:#fff9db,stroke:#868e96

    subgraph EXTERNAL[" "]
        direction TB
        subgraph USERS["👤 Users (humanos)"]
            U[Humanos]
        end
        subgraph CONSUMERS["🤖 Consumers"]
            C[Agents, services, jobs]
        end
    end

    subgraph TOWER_BOX["🖥️ Tower · SPA :4173"]
        T[Tower<br/>Supervisión UI<br/>tools, audit, admin, billing]
    end

    subgraph VPC["Internal Network · VPC"]
        subgraph CORE_BOX["nexus-core · Data Plane :8080"]
            direction TB
            CORE[data-plane/]
            PIPELINE["Auth → Tool → Policy → Rate<br/>→ Egress → Secrets → Audit<br/>/v1/run, /mcp, /a2a"]
            CORE --- PIPELINE
        end

        subgraph SAAS_BOX["nexus-saas · Control Plane :8082"]
            SAAS[control-plane/<br/>billing, users, incidents<br/>actions, events, assistant proxy]
        end

        subgraph OPS_BOX["⚙️ nexus-control-operators :8090"]
            OPS[control-workers/<br/>Sentry · Coordinator<br/>Mitigation · Recovery]
        end

        subgraph AI_BOX["🧠 nexus-ai-operators :8000"]
            AI[ai-runtime/<br/>Assistant, prompting<br/>guardrails, evals]
        end

        subgraph DATA[" "]
            PG_CORE[("🐘 Postgres Core<br/>nexus")]
            PG_SAAS[("🐘 Postgres SaaS<br/>nexus_saas")]
            REDIS["⚡ Redis<br/>rate limit / cache"]
        end

        subgraph EXTERNAL_SVC["Integraciones"]
            CLERK[🔐 Clerk IDP]
            STRIPE[💳 Stripe]
            SES[📧 AWS SES]
        end

        subgraph OBS["Observabilidad"]
            PROM[📊 Prometheus :9090]
            GRAF[📈 Grafana :3001]
        end
    end

    %% Usuarios y consumers → Tower / Core
    U -->|"Browser"| T
    C -->|"HTTPS POST /v1/run<br/>X-NEXUS-CORE-KEY / JWT"| CORE_BOX
    T -->|"tools, audit,<br/>approvals, openapi"| CORE_BOX
    T -->|"admin, billing,<br/>incidents, assistant"| SAAS_BOX

    %% Core ↔ SaaS (internos)
    CORE_BOX -->|"GET /internal/entitlements<br/>GET /internal/runtime-overrides<br/>POST /internal/usage/events"| SAAS_BOX

    %% Core → Operators (bridge)
    CORE_BOX -->|"GET /internal/operators/events<br/>events · results<br/>X-NEXUS-AI-KEY"| OPS_BOX
    CORE_BOX -->|"events (vía SaaS proxy)"| AI_BOX

    %% Operators → Core/SaaS
    OPS_BOX -.->|"POST /internal/operators/events/append<br/>POST /internal/operators/actions/apply<br/>POST /internal/operators/incidents"| CORE_BOX
    AI_BOX -.->|"GET /internal/assistant/context/:org_id"| SAAS_BOX
    SAAS_BOX -->|"POST /v1/assistant/query<br/>POST /v1/internal/tick"| AI_BOX

    %% Persistencia
    CORE_BOX --> PG_CORE
    CORE_BOX --> REDIS
    SAAS_BOX --> PG_SAAS

    %% SaaS → integraciones
    SAAS_BOX --> CLERK
    SAAS_BOX --> STRIPE
    SAAS_BOX --> SES

    %% Observabilidad (scrape)
    PROM -.->|"scrape /metrics"| CORE_BOX
    PROM -.->|"scrape"| SAAS_BOX
    PROM -.->|"scrape"| OPS_BOX
    PROM -.->|"scrape"| AI_BOX
    GRAF --> PROM

    class CORE_BOX,SAAS_BOX fillCore
    class OPS_BOX,AI_BOX fillOps
    class USERS,CONSUMERS,TOWER_BOX fillUsers
    class PG_CORE,PG_SAAS fillDb
    class REDIS fillRedis
    class CLERK,STRIPE,SES fillExt
    class PROM,GRAF fillObs
    class PIPELINE fillPipeline
```

---

## Resumen de flujos

| Origen | Destino | Contrato / etiqueta |
|--------|---------|----------------------|
| Consumers | nexus-core | `POST /v1/run`, `POST /mcp`, `POST /a2a/call` |
| Tower | nexus-core | tools, audit, approvals, openapi |
| Tower | nexus-saas | admin, billing, incidents, assistant |
| nexus-core | nexus-saas | entitlements, runtime-overrides, usage/events |
| nexus-core | Control Operators | `/internal/operators/events` (poll), events/results |
| nexus-core | AI Operators | events vía proxy SaaS |
| Control Operators | nexus-core | events/append, actions/apply, incidents, policy-proposals |
| nexus-saas | AI Operators | assistant/query, internal/tick |
| AI Operators | nexus-saas | internal/assistant/context |
| nexus-core | Postgres Core, Redis | persistencia, rate limit |
| nexus-saas | Postgres SaaS, Clerk, Stripe, SES | persistencia e integraciones |
| Prometheus | todos los servicios | scrape /metrics (dashed) |
| Grafana | Prometheus | datasource |
