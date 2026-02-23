# Action Catalog V1

## Objetivo
Definir acciones operativas aplicables por el Action Engine en forma determinista, reversible y auditable.

## Contrato común
Toda propuesta de acción (`ActionProposal`) debe incluir:

- `action_type`: string canónico.
- `scope`: objeto canónico.
  - `level`: `global|org|tool|actor`.
  - `org_id?`, `tool_id?`, `actor_id?` según `scope.level`.
- `ttl_seconds`: obligatorio para acciones automáticas.
- `params`: objeto validado por schema específico.
- `evidence_refs[]`: obligatorio si el origen es LLM y `root_cause != "unknown"`.

## Guardrails obligatorios
- TTL máximo por tier/severidad.
- acciones de alto blast radius marcan `approval_required=true`.
- `apply` debe rechazar con `APPROVAL_REQUIRED` si falta aprobación válida.
- ninguna acción se aplica por escritura directa: todo pasa por Action Engine.

## Idempotencia canónica
Llave lógica:
`incident_id + action_type + scope_hash + params_hash_normalized_without_ttl`

Reglas de normalización:
- JSON canonical con claves ordenadas recursivamente.
- sin `null`; campos no aplicables se omiten.
- `ttl_seconds` se excluye del hash de params.
- `scope_hash = hash(scope_canonical)`.

## Catálogo mínimo (Fase 1)

### `set_safe_mode`
- scope permitido: `org`, `global`.
- params:
  - `enabled` boolean required.
  - `reason` string opcional.
- approval:
  - requerido si `scope.level=global`.

### `pause_tool`
- scope permitido: `tool`.
- params:
  - `tool_id` string required.
  - `reason` string opcional.
- approval:
  - requerido por policy cuando el tool sea sensible.

### `quarantine_tenant`
- scope permitido: `org`, `actor`.
- params:
  - `org_id` required si `scope.level=org`.
  - `actor_id` required si `scope.level=actor`.
  - `mode` enum `soft|hard`, default `soft`.
- approval: requerido.

### `set_rate_limit`
- scope permitido: `org`, `tool`, `actor`.
- params:
  - `rpm` int >= 1 required.
  - `tool_id` requerido si `scope.level=tool`.
- approval:
  - requerido cuando `rpm < tenant_safe_floor`.

### `rollback_last_mitigation`
- scope permitido: `org`.
- params:
  - `incident_id` uuid required.
- approval: no requerido por default.

### `export_audit`
- scope permitido: `org`.
- params:
  - `filters` objeto opcional.
  - `format` enum `jsonl|csv`.
- approval: no requerido por default.
