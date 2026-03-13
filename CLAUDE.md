# Claude Instructions

## Preferred skills for v2

Use these local skills by default when working on `v2`.

Primary flow:

- `api-designer` first for endpoint design, contracts, resource modeling, and CRUD shape
- `golang-pro` for implementation in Go
- `golang-testing` for tests on each slice
- `code-review` before any major completion claim or after meaningful changes

Use these only when the task actually calls for them:

- `api-security-hardening` for auth, rate limits, CORS, validation hardening, and API security
- `ddd-hexagonal-architecture` when formalizing ports/adapters and architectural boundaries
- `monitoring-observability` when adding logs, metrics, tracing, or observability concerns
- `event-driven-architecture` only if `v2` starts using events or async workflows

Do not install external skills for these areas unless the local skills are clearly insufficient.

## CRUD conventions for v2

Apply these rules to every CRUD in `v2`.

- `POST /v1/<resource>`
- `GET /v1/<resource>`
- `GET /v1/<resource>/{id}`
- `PATCH /v1/<resource>/{id}`
- `DELETE /v1/<resource>/{id}` means hard delete
- `POST /v1/<resource>/{id}/archive` means soft delete
- `POST /v1/<resource>/{id}/restore` restores a soft-deleted resource

Rules:

- All CRUDs must follow the same pattern.
- Apply DRY where it makes sense.
- `DELETE` is always hard delete.
- Soft delete must use `archive`, never `DELETE`.
- Restore must use `restore`.
- Keep this convention identical across all resources in `v2`.
