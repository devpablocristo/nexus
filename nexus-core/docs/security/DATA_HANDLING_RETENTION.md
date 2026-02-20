# DATA_HANDLING_RETENTION.md

## Clasificacion de datos

- Input/context/output de runs: almacenados redactados en `audit_events`.
- Secrets de tools: cifrados en repositorio de secrets (AES-GCM).
- Identidad admin: actor/role/scopes en contexto y auditoria.

## Retencion recomendada

- Audit online (Postgres): 30 dias minimo (starter), 90 (growth), 365 (enterprise).
- Export SIEM diario via `/v1/audit/export?format=jsonl`.
- Admin activity events: misma retencion que auditoria operativa.

## Minimización

- No almacenar plaintext de secretos.
- Redaccion de payloads antes de persistencia de auditoria.
- Solo scopes efectivos (interseccion) en contexto.

## Borrado/expiracion

- Idempotency keys: TTL por `NEXUS_IDEMPOTENCY_TTL_HOURS`.
- Definir job de limpieza para auditoria por policy de plan.

## Controles de acceso

- Endpoints admin solo con permisos `admin:console:*` o rol autorizado.
- Secrets API requiere role/scopes de administrador.

## Transferencia/Export

- Export JSONL/CSV autenticado por tenant.
- Consumir por canal seguro interno (SIEM collector).

