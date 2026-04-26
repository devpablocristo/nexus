# Companion Connectors

Connectors son las capacidades operativas de Companion. Son sus ojos y manos hacia sistemas reales.

Por ahora viven dentro de Companion. No se extraen a `core` ni `modules` hasta que el contrato sea estable y exista reutilizacion real fuera de Companion.

## Contrato v1

Cada connector registra capabilities:

| Campo | Descripcion |
|-------|-------------|
| `operation` | Nombre estable de la operacion, por ejemplo `pymes.send_whatsapp_text`. |
| `mode` | `read` o `write`. |
| `side_effect` | `true` si modifica o dispara algo externo. |
| `read_only` | Compatibilidad explicita para operaciones de lectura. |
| `risk_class` | Riesgo base: `low`, `medium`, `high` o `critical`. |
| `requires_review` | Si Nexus debe permitir/aprobar antes de ejecutar. |
| `input_schema` | Schema minimo para validar payload de entrada. |
| `evidence_fields` | Campos esperados para evidencia/replay. |

## Regla De Ejecucion

- Operaciones `read` pueden ejecutarse sin approval.
- Operaciones `write` o con `side_effect` requieren Nexus `allowed` o `approved`.
- Si una operacion requiere review y no trae `review_request_id`, Companion responde `UNGATED`.
- Si el connector, la request de Nexus o la ejecucion pertenecen a otra `org_id`, Companion responde `FORBIDDEN`.
- Si el payload no cumple `input_schema.required`, Companion responde `VALIDATION`.
- Ejecuciones con `task_id + operation + review_request_id + idempotency_key` reutilizan la ejecucion persistida cuando existe.

## Relacion Con Nexus

Companion propone acciones a Nexus. Nexus decide. Companion ejecuta connectors solo cuando corresponde. Luego Companion reporta resultado a Nexus con `/result`.

Nexus no conoce adapters concretos. Nexus gobierna action types, riesgo, approvals, audit y evidence.

## Evidencia Minima

Cada ejecucion persistida registra un bloque `evidence_json` con:

- `actor_id`, `org_id`, `connector_id`, `connector_kind`;
- `operation`, `mode`, `side_effect`, `risk_class`;
- payload y resultado sanitizados;
- `external_ref`, `status`, `duration_ms`;
- `task_id`, `review_request_id`, `idempotency_key`;
- estado `verification: unsigned` para preparar attestation verificable posterior.

Campos sensibles como `api_key`, `token`, `secret`, `password`, `authorization`, `private_key` y `client_secret` se guardan enmascarados.

## Scopes De Companion

| Scope | Uso |
|-------|-----|
| `companion:connectors:execute` | ejecutar operaciones de connectors |
| `companion:connectors:admin` | crear/borrar/admin connectors |
| `companion:tasks:read` | leer/listar tasks |
| `companion:tasks:write` | crear/modificar/proponer/ejecutar tasks |

## Adapters Iniciales

- `mock`: adapter local para smoke/e2e.
- `pymes`: adapter concreto de Companion para Pymes. No es libreria reusable por ahora.
