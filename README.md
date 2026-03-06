# Nexus Monorepo

Nexus es un plano de control para ejecución gobernada de herramientas por agentes y operadores.

## Servicios

| Servicio | Rol |
|----------|-----|
| `nexus-core` | data plane determinista: `/v1/run`, `/mcp`, `/a2a`, policies, DLP, egress, approvals, audit |
| `nexus-saas` | business plane multi-tenant: billing, users, incidents, actions, events, notifications, assistant proxy |
| `nexus-control-operators` | workers deterministas del control plane |
| `nexus-ai-operators` | runtime AI asistido: assistant, prompting versionado, fallback, evals |
| `nexus-tower` | UI de supervisión |

## Invariantes

- No LLM en el pipeline de enforcement.
- Operators sin writes directos a DB.
- `nexus-core` y `nexus-saas` mantienen ownership separado.
- Contracts, docs, SDKs y observabilidad forman parte del producto.

## Quickstart

```bash
cp .env.example .env
make up
make migrate-up
make seed
```

## Documentación canónica

- `docs/DOC.md`
- `docs/SERVICE_BOUNDARIES.md`
- `docs/AGENT_OPERATED_MODEL.md`
- `docs/policy/POLICY_DSL_REFERENCE.md`
- `docs/protocols/MCP_GUIDE.md`
- `docs/protocols/A2A_GUIDE.md`
- `docs/data/DATA_MODEL_AND_OWNERSHIP.md`
- `docs/events/EVENT_CATALOG.md`
- `docs/testing/TEST_STRATEGY.md`
- `docs/runbooks/INCIDENT_RESPONSE.md`
- `docs/adr/README.md`

## Contratos compartidos

- `pkgs/contracts/openapi.nexus-core.snapshot.yaml`
- `pkgs/contracts/openapi.nexus-saas.snapshot.yaml`
- `pkgs/contracts/error-codes.json`
- `pkgs/contracts/events.schema.json`

## SDKs

- `sdks/python-sdk`
- `sdks/typescript-sdk`
- `sdks/go-sdk`
