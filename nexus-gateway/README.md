# Nexus Backend (MVP)

Nexus is a B2B **Agent Tool Gateway**: a single, controlled entrypoint that AI agents use to execute tools safely (authn via API key, tool registry, JSON Schema validation, policy + rate-limit enforcement, HTTP tool execution, and append-only audit).

## Architecture (hexagonal)

```
        HTTP (Gin)
           |
        Handlers  (internal/*/handler)
           |
        Usecases  (internal/*/usecases)  <-- ports (interfaces) live here
        /     \
   Repos      Executors
 (GORM)   (HTTP, RateLimit)
```

Key rules:
- Handlers depend only on usecase interfaces.
- Usecases import no Gin/GORM.
- Multi-tenancy enforced via `org_id` in every repository query.

## Quickstart

```bash
make dev
make migrate-up
make seed
```

API:
- Swagger UI: `http://localhost:8080/docs`
- OpenAPI: `http://localhost:8080/openapi.yaml`

## Verify (curl)

After `make seed`, the script prints `NEXUS_DEMO_API_KEY=...` once. Export it:

```bash
export NEXUS_API_KEY="paste_key_here"
```

Health:
```bash
curl -sS http://localhost:8080/healthz | jq
curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8080/readyz
```

List tools:
```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" http://localhost:8080/v1/tools | jq
```

Run echo:
```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{\"tool_name\":\"echo\",\"input\":{\"hello\":\"world\"},\"context\":{\"user_id\":\"u_123\"}}' \\
  http://localhost:8080/v1/run | jq
```

Transfer denied (amount > 1000):
```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{\"tool_name\":\"transfer\",\"input\":{\"amount\":5000,\"token\":\"secret\"},\"context\":{\"user_id\":\"u_123\"}}' \\
  http://localhost:8080/v1/run | jq
```
Expected: `status=blocked`, `decision=deny`, `error.code=POLICY_DENIED`.

Transfer denied (missing `context.user_id`):
```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{\"tool_name\":\"transfer\",\"input\":{\"amount\":500},\"context\":{\"token\":\"secret\"}}' \\
  http://localhost:8080/v1/run | jq
```

Transfer allowed:
```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{\"tool_name\":\"transfer\",\"input\":{\"amount\":500,\"card_number\":\"4111111111111111\"},\"context\":{\"user_id\":\"u_123\"}}' \\
  http://localhost:8080/v1/run | jq
```
Expected: `status=success`, `decision=allow`, and audit redaction applied.

Audit query:
```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \\
  "http://localhost:8080/v1/audit?tool_name=transfer&limit=10" | jq
```

## Auth (API key hashing)

- Clients send plaintext in `X-NEXUS-GATEWAY-KEY`.
- Server computes SHA-256 hex and looks up `org_api_keys.api_key_hash`.
- Server never logs plaintext keys (only truncated hash if needed).

## Policy DSL examples

Leaf:
```json
{ "path": "input.amount", "op": "lte", "value": 1000 }
```

Boolean:
```json
{
  "all": [
    { "path": "input.amount", "op": "lte", "value": 1000 },
    { "path": "context.user_id", "op": "exists" }
  ]
}
```

## Audit query

`GET /v1/audit?tool_name=...&decision=allow|deny&status=success|error|blocked&from=...&to=...&limit=...`

## MVP limitations

- In-memory rate limiting (single instance).
- HTTP tools only.
- No UI and no user auth beyond API key.

