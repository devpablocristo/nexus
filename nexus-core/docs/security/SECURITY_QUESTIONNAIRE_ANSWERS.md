# SECURITY_QUESTIONNAIRE_ANSWERS.md

## 1) Authentication

- API key auth (`X-NEXUS-CORE-KEY`) y JWT/JWKS opcional.
- Referencias:
  - `internal/shared/handlers/auth_middleware.go`
  - `internal/identity/usecases.go`

## 2) Authorization (least privilege)

- RBAC estricto por scopes por endpoint.
- Referencias:
  - `docs/security/RBAC_PERMISSIONS.md`
  - `internal/shared/authz/http_permissions.go`
  - handlers `internal/*/handler.go`

## 3) Tenant isolation

- Repos filtran por `org_id`; tool/policy/audit acotados por tenant.
- Referencias:
  - `internal/*/repository.go`
  - `migrations/0002_core_tables.up.sql`

## 4) Encryption at rest / secrets handling

- Secrets de tools cifrados AES-GCM.
- Referencias:
  - `internal/secrets/repository.go`
  - `pkg/utils/aesgcm.go`

## 5) Encryption in transit

- TLS termina en infraestructura/reverse proxy (deployment concern).
- GAP: falta guía TLS de referencia en deployment chart.
- Cierre: 2026-03-15 en `docs/runbooks/DEPLOY_PROD.md` con sección TLS ingress.

## 6) Audit logging / traceability

- `audit_events` append-only con hash-chain (`prev_event_hash`, `event_hash`).
- `admin_activity_events` para cambios administrativos.
- Referencias:
  - `internal/audit/repository.go`
  - `migrations/0005_audit_hash_chain.up.sql`
  - `migrations/0007_admin_console_foundation.up.sql`

## 7) Data retention / export

- Export JSONL/CSV autenticado por tenant.
- Policy de retención documentada por plan.
- Referencias:
  - `internal/audit/handler.go`
  - `docs/security/DATA_HANDLING_RETENTION.md`

## 8) Abuse prevention

- Rate limit por tool + hard limit por tenant `run_rpm`.
- Body/response/timeouts y idempotencia en writes.
- Referencias:
  - `internal/gateway/usecases.go`
  - `internal/gateway/idempotency_repository.go`

## 9) SSRF / egress control

- Validación SSRF + egress allowlist default-deny.
- Referencias:
  - `pkg/utils/ssrf.go`
  - `internal/egress/usecases.go`
  - `internal/gateway/usecases.go`

## 10) Incident response

- Runbook P1/P2 disponible.
- Referencias:
  - `docs/runbooks/INCIDENTS_P1_P2.md`

## 11) Backups / restore

- Procedimientos base documentados.
- Referencias:
  - `docs/runbooks/DEPLOY_PROD.md`
- GAP: falta job automatizado + restore drill periódico con evidencia.
- Cierre: 2026-03-22.

## 12) Release governance

- Gates definidos: unit + integration + e2e + smoke.
- Referencias:
  - `docs/runbooks/RELEASE_GATES.md`

## 13) SSO/OIDC

- JWT/JWKS presente para API.
- GAP: SSO web completo para consola admin aún no implementado.
- Cierre objetivo: 2026-04-05 (Semana 4 plan).

## 14) Administrative change control

- Se auditan acciones admin de bootstrap/settings/activity y se expone query.
- Referencias:
  - `internal/admin/usecases.go`
  - `internal/admin/handler.go`

## 15) Residual risks (actual)

- Receipts criptográficos verificables externamente (no solo hash-chain local): GAP.
- Delegación verificable por ejecución (zero-trust chain): GAP.
- A2A bridge baseline implementado (`POST /a2a/call`) sobre `gateway.Run`, pero faltan pruebas de paridad exhaustivas MCP vs A2A por escenario adversarial.
- Cierre previsto en vanguard roadmap (`docs/vanguard/VANGUARD_BETS.md`).
