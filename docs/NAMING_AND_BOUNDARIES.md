# Naming and Boundaries

## Names

- Product BE name: `nexus-core`.
- Internal runtime/data plane component: `gateway`.
- Deterministic control-plane service: `nexus-control-operators` (deployed from `nexus-control-operators/cmd/ops-workers`).
- AI operations service: `nexus-ai-operators`.
- Supervision UI: `nexus-tower`.
- Python SDK: `nexus-sdk` (`sdks/python-sdk`).
- TypeScript SDK: `nexus-sdk` (`sdks/typescript-sdk`).

## Stable Compatibility

- Existing REST/MCP/A2A endpoint paths are unchanged.
- Existing auth headers remain (`X-NEXUS-CORE-KEY`, `Authorization: Bearer <jwt>`).
- SDK clients target `/v1/*` endpoints exclusively.

## Explicit Non-Renames (for safety)

- Existing external header names used by clients.
- Existing metric namespace `nexus_gateway_*` used by dashboards/alerts.
- Error codes catalog (`shared/contracts/error-codes.json`).
