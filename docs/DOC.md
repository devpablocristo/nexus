# Nexus Technical Docs

Documento índice canónico del repo.

## Arquitectura

Nexus está dividido en bounded contexts explícitos:

- `nexus-core`: enforcement determinista, audit, approvals, MCP y A2A
- `nexus-saas`: business plane multi-tenant
- `nexus-control-operators`: automatización determinista
- `nexus-ai-operators`: prompting runtime, assistant y asistencia operativa
- `nexus-tower`: supervisión UI

## Dónde leer cada tema

- Boundaries y ownership: `docs/SERVICE_BOUNDARIES.md`
- Modelo agent-operated: `docs/AGENT_OPERATED_MODEL.md`
- Policy DSL: `docs/policy/POLICY_DSL_REFERENCE.md`
- MCP: `docs/protocols/MCP_GUIDE.md`
- A2A: `docs/protocols/A2A_GUIDE.md`
- Data ownership: `docs/data/DATA_MODEL_AND_OWNERSHIP.md`
- Event catalog: `docs/events/EVENT_CATALOG.md`
- Testing y release gates: `docs/testing/*`
- Incident response: `docs/runbooks/*`
- ADRs: `docs/adr/*`

## Contratos

- APIs públicas bajo `/v1/*`
- JSON-RPC en `/mcp`
- REST A2A en `/a2a/call`
- error codes: `pkgs/contracts/error-codes.json`
- event schema: `pkgs/contracts/events.schema.json`

## Operación

- Launch checklist: `docs/runbooks/LAUNCH_CHECKLIST.md`
- SLO/SLI: `docs/runbooks/SLO_SLI.md`
- Rollback: `docs/runbooks/DEPLOY_ROLLBACK.md`
- Backup/DR: `docs/runbooks/DB_BACKUP_DR.md`
- Secret rotation: `docs/runbooks/SECRET_ROTATION.md`
- Incident response: `docs/runbooks/INCIDENT_RESPONSE.md`

## Suite de prompts

La especificación histórica y de alcance vive en `docs/prompts/00_base_transversal.md` a `docs/prompts/17_architecture_decision_records.md`.
