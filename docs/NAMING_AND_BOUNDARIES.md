# Naming and Boundaries

## Names

| Name | What | Path |
|------|------|------|
| `nexus-core` | Gateway / data plane (Go) | `data-plane/` |
| `nexus-saas` | Business plane (Go) | `control-plane/` |
| `nexus-control-operators` | Deterministic control plane (Go) | `control-workers/` |
| `nexus-ai-operators` | AI operators (Python) | `ai-runtime/` |
| `nexus-tower` | Supervision UI (React/TS) | `tower/` |
| `nexus-sdk` (Python) | Python SDK | `sdks/python-sdk/` |
| `nexus-sdk` (TypeScript) | TypeScript SDK | `sdks/typescript-sdk/` |
| `pkgs/go-pkg` | Shared Go packages | `pkgs/go-pkg/` |
| `pkgs/contracts` | Shared JSON schemas and error codes | `pkgs/contracts/` |

## Stable Compatibility

- Existing REST/MCP/A2A endpoint paths are unchanged.
- Existing auth headers remain: `X-NEXUS-CORE-KEY`, `Authorization: Bearer <jwt>`.
- Internal operator header: `X-NEXUS-AI-KEY`.
- Public contracts stay under `/v1/*`, `/mcp` and `/a2a/*`.

## Metric Namespaces

| Namespace | Service |
|-----------|---------|
| `nexus_gateway_*` | nexus-core (Prometheus) |
| `nexus_operators_*` | nexus-control-operators (Prometheus) |
| `nexus_saas_*` | nexus-saas (Prometheus) |
| `nexus_ai_*` / `nexus_operator_*` | nexus-ai-operators (Prometheus) |

## Shared Packages (go.work)

```
go 1.24.0

use (
    ./control-workers
    ./data-plane
    ./control-plane
    ./pkgs/go-pkg
)
```

## Explicit Non-Renames

- External header names used by clients (`X-NEXUS-CORE-KEY`).
- Metric namespace `nexus_gateway_*` used by dashboards/alerts.
- Error codes catalog (`pkgs/contracts/error-codes.json`).
- Event schemas (`pkgs/contracts/events.schema.json`).
