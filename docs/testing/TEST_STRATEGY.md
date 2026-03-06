# Test Strategy

## Matriz por servicio

| Servicio | Unit | Integration | Contract | E2E | Smoke | Load | Security |
|----------|------|-------------|----------|-----|-------|------|----------|
| `nexus-core` | policy, gateway, auth | repo/adapters | OpenAPI, MCP, A2A, error codes | scripts/e2e | `scripts/smoke/smoke_prod.sh` | `scripts/loadtest/k6_gateway.js` | CI scans |
| `nexus-saas` | billing, alerts, notifications, admin | repo/webhooks | OpenAPI, internal contracts | e2e SaaS flows | smoke post-deploy | selectivo | CI scans |
| `nexus-control-operators` | workers/engines | schema/eventstore | event schemas | operators e2e | smoke indirecto | no default | CI scans |
| `nexus-ai-operators` | observer, risk, prompt runtime | FastAPI routes | prompt/eval contracts | assistant/operator e2e | smoke indirecto | no default | CI scans |
| `nexus-tower` | components/pages | API integration mocks | UI contract around APIs | browser/e2e when relevant | UI smoke | no default | dependency audit |
| SDKs | client methods | example flows | contract compatibility | release samples | n/a | n/a | dependency audit |

## Reglas

- Enforcement, auth, billing y contracts comparten gates más estrictos.
- Cambios que tocan runtime crítico deben incluir evidencia de tests y, si aplica, smoke/load.
- Evals del runtime AI cuentan como contract coverage del subsistema AI.
