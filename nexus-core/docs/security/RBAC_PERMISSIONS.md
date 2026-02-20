# RBAC_PERMISSIONS.md

## Scope model (authoritative)

Nexus usa permisos por `scope` con acciones `read/write/admin` por recurso.

| Recurso | Read | Write/Admin | Endpoints |
|---|---|---|---|
| Tools | `tools:read` | `tools:write` | `GET /v1/tools*`, `POST/PUT /v1/tools*` |
| Policies | `policy:read` | `policy:write` | `GET /v1/tools/:name/policies`, `POST/PUT policies` |
| Egress | `egress:read` | `egress:write` | `GET/POST/DELETE /v1/tools/:name/egress-rules` |
| Audit | `audit:read` | n/a | `GET /v1/audit`, `GET /v1/audit/export` |
| Gateway | `gateway:simulate` | `gateway:run` | `POST /v1/run/simulate`, `POST /v1/run` |
| MCP | `mcp:read` | `mcp:call` | `tools/list|get`, `tools/call` |
| A2A | n/a | `a2a:call` | `POST /a2a/call` |
| Admin console | `admin:console:read` | `admin:console:write` | `/v1/admin/bootstrap`, `/v1/admin/tenant-settings`, `/v1/admin/activity` |
| Secrets | n/a | `admin:secrets` (+ role admin/secops) | `GET/POST/DELETE /v1/tools/:name/secrets` |

## Role shortcuts

- `role=admin` => acceso completo operativo.
- `role=secops` => acceso operativo amplio (lectura global + secretos), pero no equivalente a super-admin comercial.

## Claims -> scopes mapping

- API key: scopes salen de `org_api_key_scopes` y se intersectan con `X-NEXUS-SCOPES`.
- JWT: scopes salen de claim configurable (`NEXUS_JWT_SCOPES_CLAIM`) y se intersectan con `X-NEXUS-SCOPES`.

## Compat legacy y deprecacion

- Flag: `NEXUS_AUTH_LEGACY_SCOPE_FALLBACK`.
- `true` (default actual): si request no trae scopes efectivos, se permite por compatibilidad legacy (excepto restricciones adicionales de endpoint).
- `false` (modo estricto): scopes obligatorios; requests sin scopes reciben `403`.

## Plan de deprecacion (decidido)

- 2026-03-31: anunciar deprecacion de fallback legacy en release notes.
- 2026-06-30: entornos nuevos por defecto con `NEXUS_AUTH_LEGACY_SCOPE_FALLBACK=false`.
- 2026-09-30: remover fallback legacy (breaking change mayor).

## Implementacion en repo (referencias)

- Modelo de decisión y fallback: `internal/shared/authz/http_permissions.go`
- Scope constants: `internal/shared/authz/scopes.go`
- Tool RBAC: `internal/tool/handler.go`
- Policy RBAC: `internal/policy/handler.go`
- Egress RBAC: `internal/egress/handler.go`
- Audit RBAC: `internal/audit/handler.go`
- Gateway RBAC: `internal/gateway/handler.go`
- MCP RBAC: `internal/mcp/handler.go`
- A2A RBAC: `internal/a2a/handler.go`
- Secrets RBAC: `internal/secrets/handler.go`
