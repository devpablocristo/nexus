# data-plane

`data-plane` is the internal monorepo directory for the deterministic Nexus runtime. The deployed service name remains `nexus-core`.

`data-plane` contains:
- Data plane (`gateway` runtime): `/v1/run`, `/v1/run/simulate`, `/mcp`, `/a2a/call`.
- Control plane: tools, policies, secrets, egress, audit/export, admin.
- Agent-operated APIs: actions, incidents, policy proposals, events feed.

## Deterministic Enforcement Pipeline

`/v1/run` executes this order (no LLM in enforcement):
1. authn/authz
2. timeout budget
3. tool lookup
4. idempotency
5. context + deterministic DLP summary
6. input schema validation
7. policy evaluation
8. tenant/tool limits + rate limit
9. SSRF + egress allowlist
10. secrets injection
11. upstream HTTP execution
12. output schema validation
13. append-only audit hash-chain

## Quickstart

```bash
cp .env.example .env
# local Docker demo only
# echo "NEXUS_DISABLE_SSRF_PROTECTION=true" >> .env
make up
make migrate-up
make seed
```

Docs and UI:
- OpenAPI: `http://localhost:8080/openapi.yaml`
- Swagger UI: `http://localhost:8080/docs`
- Supervision UI (`tower` / deployed as `nexus-tower`): `http://localhost:5173` in dev, `http://localhost:4173` in preview/docker
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000`

## Key Endpoints

- Runtime:
  - `POST /v1/run`
  - `POST /v1/run/simulate`
  - `POST /mcp`
  - `POST /a2a/call`
- Agent-operated control:
  - `GET /v1/events`
  - `POST /v1/actions/apply`
  - `POST /v1/actions/rollback`
  - `GET /v1/actions`
  - `POST /v1/incidents`
  - `GET /v1/incidents`
  - `GET /v1/incidents/:id`
  - `POST /v1/incidents/:id/close`
  - `POST /v1/policy-proposals`
  - `GET /v1/policy-proposals`
  - `POST /v1/policy-proposals/:id/approve`
  - `POST /v1/policy-proposals/:id/reject`
  - `POST /v1/policy-proposals/:id/shadow`

## API Stability

- HTTP API headers remain stable (`X-NEXUS-CORE-KEY`, etc.).
- Existing REST/MCP endpoints are unchanged.
