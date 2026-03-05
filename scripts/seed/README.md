# Seed

Población de datos de demo en las bases de datos.

## Scripts

| Script | Descripción |
|--------|-------------|
| `seed_demo.sh` | Crea org "demo", API keys, tools (echo, transfer), policies en core y saas |

## Uso

```bash
bash scripts/seed/seed_demo.sh
# o
./scripts/seed/seed_demo.sh --help
```

## Qué crea

| Recurso | Detalle |
|---------|---------|
| Org | `demo` (en core y saas) |
| API key `demo-key` | Scopes completos (tools, policy, audit, gateway, MCP, A2A, admin) |
| API key `operator-key` | Scopes limitados (audit:read, admin console) |
| Tool `echo` | read, internal, risk 1 → `mock-tools:8081/echo` |
| Tool `transfer` | write, external, risk 3 → `mock-tools:8081/transfer` |
| Policies (transfer) | deny DLP credit card, deny amount > 1000, allow con idempotency |
| Alert rule | `high-deny-rate` en saas |

## Output

```
NEXUS_DEMO_API_KEY=nexus-core-local-key
NEXUS_OPERATOR_API_KEY=nexus-ai-operators-local-key
```

Estas keys son determinísticas para entornos locales.
