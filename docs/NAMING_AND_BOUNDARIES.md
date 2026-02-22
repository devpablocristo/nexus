# Naming and Boundaries

## Names

- Product BE name: `nexus-core`.
- Internal runtime/data plane component: `gateway`.
- AI operations service: `nexus-operator`.
- Supervision UI: `nexus-tower`.

## Stable Compatibility

- Existing REST/MCP endpoint paths are unchanged.
- Existing auth headers remain (`X-NEXUS-GATEWAY-KEY`, etc.).

## Explicit Non-Renames (for safety)

- Existing external header names used by clients.
- Existing metric namespace `nexus_gateway_*` used by dashboards/alerts.
