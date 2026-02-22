# Nexus Monorepo

Nexus is an **agent-operated execution control plane** split into three root projects:

- `nexus-core`: deterministic backend (control plane + data plane + audit/state).
- `nexus-operator`: AI-operated service (signals, risk scoring, reversible actions, proposals).
- `nexus-tower`: supervision UI (overview, timeline, policies, ask-agent, exports).

## Naming Rules

- Product backend name: **Nexus Core**.
- `gateway` remains an internal runtime component inside `nexus-core`.
- Existing REST/MCP endpoints and auth headers remain stable.

## Monorepo Layout

```text
/nexus-core
/nexus-operator
/nexus-tower
/shared
/docs
/scripts
/Makefile
/docker-compose.yml
```

## Quickstart (all services)

```bash
cp .env.example .env
make up
make migrate-up
make migrate-worldsim
make seed
make qa-worldsim
```

`make seed` prints:
- `NEXUS_DEMO_API_KEY` (human/demo key)
- `NEXUS_OPERATOR_API_KEY` (service key for operator)

Set `VITE_NEXUS_API_KEY` in `.env` if you want Tower to query authenticated endpoints from browser.

## URLs

- Nexus Core API: `http://localhost:8080`
- Admin Console: `http://localhost:8080/admin`
- Operator API: `http://localhost:8000`
- Tower UI: `http://localhost:5173`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000`

## Root Commands

- `make up` / `make down`
- `make migrate-up`
- `make migrate-worldsim`
- `make seed`
- `bash scripts/seed_worldsim_demo.sh`
- `make demo-doorjam`
- `make replay RUN_ID=<run-id>`
- `make qa`
- `make e2e`
- `make jwt-e2e`

## Contracts

Shared contracts/clients are under `shared/`:
- `shared/contracts/events.schema.json`
- `shared/contracts/openapi.nexus-core.snapshot.yaml`
- `shared/contracts/error-codes.json`
- `shared/clients/python/nexus_core_client.py`
- `shared/clients/ts/nexus_core_client.ts`

## Agent-Operated Model

- Operator **never** writes to DB directly.
- Operator consumes `GET /v1/events` and acts through:
  - `POST /v1/actions/apply`
  - `POST /v1/incidents`
  - `POST /v1/policy-proposals`
- Tower does not call LLM directly.
- Ask-agent flow: `nexus-tower` -> `nexus-core /v1/assistant/query` -> `nexus-operator /v1/assistant/query`.
