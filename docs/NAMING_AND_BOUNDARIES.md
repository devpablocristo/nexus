# Naming and Boundaries

## Names

| Name | What | Path |
|------|------|------|
| `nexus-core` | Gateway / data plane (Go) | `nexus-core/` |
| `nexus-saas` | Business plane (Go) | `nexus-saas/` |
| `nexus-control-operators` | Deterministic control plane (Go) | `nexus-control-operators/` |
| `nexus-ai-operators` | AI operators (Python) | `nexus-ai-operators/` |
| `nexus-tower` | Supervision UI (React/TS) | `nexus-tower/` |
| `nexus-sdk` (Python) | Python SDK | `sdks/python-sdk/` |
| `nexus-sdk` (TypeScript) | TypeScript SDK | `sdks/typescript-sdk/` |
| `pkgs/go-pkg` | Shared Go packages | `pkgs/go-pkg/` |
| `pkgs/contracts` | Shared JSON schemas and error codes | `pkgs/contracts/` |

## Stable Compatibility

- Existing REST/MCP/A2A endpoint paths are unchanged.
- Existing auth headers remain: `X-NEXUS-CORE-KEY`, `Authorization: Bearer <jwt>`.
- Internal operator header: `X-NEXUS-AI-KEY`.
- SDK clients target `/v1/*` endpoints exclusively.

## Metric Namespaces

| Namespace | Service |
|-----------|---------|
| `nexus_gateway_*` | nexus-core (Prometheus) |
| `nexus_operators_*` | nexus-control-operators (Prometheus) |
| `nexus_saas_*` | nexus-saas (Prometheus) |

## Shared Packages (go.work)

```
go 1.24.0

use (
    ./nexus-control-operators
    ./nexus-core
    ./nexus-saas
    ./pkgs/go-pkg
)
```

## Explicit Non-Renames

- External header names used by clients (`X-NEXUS-CORE-KEY`).
- Metric namespace `nexus_gateway_*` used by dashboards/alerts.
- Error codes catalog (`pkgs/contracts/error-codes.json`).
- Event schemas (`pkgs/contracts/events.schema.json`).
