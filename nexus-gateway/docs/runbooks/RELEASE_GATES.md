# RELEASE_GATES.md

## Policy

Ningún release productivo pasa sin todos los gates en verde.

## Gates obligatorios

1. Unit / module tests
```bash
go test ./...
```

2. API e2e (REST + MCP + controls)
```bash
make e2e
```

3. JWT e2e
```bash
make jwt-e2e
```

4. Smoke (stack limpio)
```bash
make down
docker compose up --build -d
make migrate-up
make seed
test "$(curl -sS -o /dev/null -w "%{http_code}" http://localhost:${NEXUS_HTTP_PORT:-8080}/readyz)" = "200"
test "$(curl -sS -o /dev/null -w "%{http_code}" http://localhost:${NEXUS_HTTP_PORT:-8080}/admin)" = "200"
curl -fsS http://localhost:${NEXUS_HTTP_PORT:-8080}/metrics | grep -E "nexus_run_total_prom|nexus_gateway_req_count"
```

5. Quickstart reproducible (onboarding gate)
```bash
make quickstart-admin
```

## Optional but recomendado

- `make qa`
- contract tests audit export:
```bash
go test ./internal/audit -run TestAuditExport
```

## Rollback policy

- Si cualquier gate falla => release bloqueado.
- Si falla post-deploy => rollback a imagen previa + evaluación DB compatibility.
- Ejecutar runbook de incidentes si impacto cliente (`docs/runbooks/INCIDENTS_P1_P2.md`).
