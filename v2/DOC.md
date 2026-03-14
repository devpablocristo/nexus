# V2

## Reglas generales

Estas reglas aplican a todos los CRUD de `v2`.

- todos los CRUD deben seguir el mismo patron
- aplicar DRY donde convenga
- `DELETE` siempre significa hard delete
- soft delete no usa `DELETE`
- para soft delete se usa `archive`
- para restaurar soft deletes se usa `restore`
- esta convencion debe mantenerse siempre igual en todos los recursos

Patron esperado:

- `POST /v1/<resource>`
- `GET /v1/<resource>`
- `GET /v1/<resource>/{id}`
- `PATCH /v1/<resource>/{id}`
- `DELETE /v1/<resource>/{id}` para hard delete
- `POST /v1/<resource>/{id}/archive` para soft delete
- `POST /v1/<resource>/{id}/restore` para restaurar soft delete

## `/run`

`POST /v1/run` existe hoy en `v2/data-plane` como el primer happy path funcional.

Esta seccion documenta solo el comportamiento actual de `v2`, no el contrato completo de `v1`.

### Endpoint

- Metodo: `POST`
- Path: `/v1/run`
- Handler: `v2/data-plane/internal/gateway/handler.go`

### Request

Body JSON:

```json
{
  "request_id": "optional",
  "tool_name": "echo",
  "tool_id": "tool_echo",
  "timeout_ms": 2500,
  "input": {
    "hello": "world"
  },
  "context": {}
}
```

Header opcional:

- `Idempotency-Key: <string>`

Reglas actuales:

- el body debe ser JSON valido
- campos desconocidos fallan porque el decoder usa `DisallowUnknownFields`
- `tool_name` o `tool_id` son obligatorios
- `input` es obligatorio
- `request_id` es opcional
- `timeout_ms` es opcional
- `Idempotency-Key` es opcional
- si `request_id` no viene, se genera uno
- `context` es opcional
- `timeout_ms` se clamp-ea a `1000..30000`
- si `timeout_ms` no viene o es `<= 0`, usa default `10000`
- si `Idempotency-Key` viene, hoy solo aplica a tools write

Regla actual de resolucion:

- si viene `tool_id`, se usa `tool_id`
- si no viene `tool_id`, se usa `tool_name`

Si el body trae `request_id`, ese valor prevalece sobre el generado por el handler.

### Success response

Status: `200 OK`

```json
{
  "request_id": "generated-or-passed-through",
  "decision": "allow",
  "tool_name": "echo",
  "status": "success",
  "result": {},
  "latency_ms": 3,
  "intent_id": "",
  "approval_id": "",
  "idempotency": {
    "present": true,
    "outcome": "NEW"
  }
}
```

Campos actuales:

- `decision`: en exito hoy es `allow`
- `status`: hoy siempre `success` si no hubo error
- `result`: payload devuelto por el upstream, ya parseado
- `latency_ms`: duracion total medida dentro de `Usecases.Run`

### Blocked response

Status: `403 Forbidden`

```json
{
  "request_id": "generated-or-passed-through",
  "decision": "deny",
  "tool_name": "echo",
  "status": "blocked",
  "reason": "blocked by policy",
  "latency_ms": 1,
  "intent_id": "",
  "approval_id": ""
}
```

Cuando una policy requiere approval, el contrato actual de `/run` responde:

- status HTTP `202 Accepted`
- `decision=deny`
- `status=blocked`
- `reason=pending human approval (id: <approval_id>)`
- `intent_id`
- `approval_id`

### Error responses actuales

Formato:

```json
{
  "request_id": "req-id",
  "error": {
    "code": "SOME_CODE",
    "message": "human readable message"
  },
  "idempotency": {
    "present": true,
    "outcome": "CONFLICT"
  }
}
```

Mapa actual:

- `400 INVALID_JSON`
- `400 VALIDATION`
- `400 INVALID_TOOL_URL`
- `400 UNSUPPORTED_TOOL_KIND`
- `400 INPUT_SCHEMA_INVALID`
- `403 TOOL_DISABLED`
- `404 NOT_FOUND`
- `408 TIMEOUT`
- `409 IDEMPOTENCY_CONFLICT`
- `409 IDEMPOTENCY_IN_PROGRESS`
- `500 EGRESS_STORE_ERROR`
- `500 POLICY_DECISION_ERROR`
- `500 APPROVAL_NOT_CONFIGURED`
- `500 SECRETS_STORE_ERROR`
- `500 IDEMPOTENCY_STORE_ERROR`
- `502 OUTPUT_SCHEMA_INVALID`
- `502 UPSTREAM_ERROR`

### Flujo interno actual

1. `handler.runTool`
2. parseo del body
3. validacion basica del request
4. `Usecases.Run`
5. `clampTimeoutMS`
6. `resolveTool`
7. `resolveIdempotency`
8. `buildRequestFingerprint`
9. `mapRunError`
10. `toRunHTTPError`
11. `validateAndPrepare`
12. `decide`
13. `IntentRepository.Create`
14. `ApprovalPort.RequestApproval`
15. `IntentRepository.LinkApproval`
16. `prepareExecution`
17. `executeAndFinish`
18. `executor/http.Executor.Execute`
19. `markCompletedIdempotency` o `markFailedIdempotency`
20. armado de `RunResponse`

### Funciones de `Usecases.Run`

- `Usecases.Run`
  Es la orquestacion principal del caso de uso. Inicializa el estado del run, asegura `request_id`, normaliza `input` y `context`, llama las etapas del flujo y construye la respuesta final.

- `resolveTool`
  Resuelve la tool pedida a partir de `tool_id` o `tool_name`. Sirve para traer la definicion que se va a ejecutar y cortar temprano si la tool no existe.

- `resolveIdempotency`
  Resuelve el comportamiento de la request idempotente antes del resto del pipeline. Sirve para crear el registro nuevo, detectar conflicto, devolver replay o cortar por in-progress.

- `buildRequestFingerprint`
  Genera una huella estable del request a partir de tool, input y context. Sirve para saber si una misma idempotency key se esta reusando con otro payload.

- `clampTimeoutMS`
  Normaliza el timeout pedido por el caller. Sirve para aplicar default y limites antes de crear el deadline del request.

- `mapRunError`
  Traduce errores del flujo a errores del dominio de `run`. Hoy sirve especialmente para convertir `context deadline exceeded` en el error de timeout del endpoint.

- `toRunHTTPError`
  Normaliza errores del dominio a un error HTTP consistente del endpoint. Sirve para que handler e idempotency hablen el mismo contrato de status y codes.

- `validateAndPrepare`
  Valida que la tool este en condiciones de ejecutarse. Sirve para frenar antes del upstream si la tool esta deshabilitada, si el tipo no es soportado o si el `input` no cumple el `input_schema`.

- `decide`
  Evalua policies de la tool antes de ejecutar. Sirve para permitir, bloquear por deny o cortar en pending approval creando `intent` y `approval` sin llegar al upstream.

- `prepareExecution`
  Prepara la llamada real al upstream. Sirve para aplicar rate limit por tool, validar la URL de la tool, chequear egress por host y resolver headers desde secrets antes de ejecutar.

- `executeAndFinish`
  Ejecuta la llamada real al upstream y valida la salida. Sirve para delegar la ejecucion HTTP al adapter y asegurar que el resultado cumpla el `output_schema` antes de devolver exito.

### Validaciones actuales

Antes del upstream:

- existencia de la tool
- idempotency
- `timeout_ms` normalizado y `context.WithTimeout`
- `tool.Enabled == true`
- `tool.Kind == http`
- `input_schema`
- policies
- intents y approvals si la policy los requiere
- rate limit por tool
- URL valida de la tool
- egress host permitido
- resolucion de secrets y headers

Despues del upstream:

- `output_schema`
- mark completed de idempotency si corresponde

### Idempotency actual

Hoy `/run` soporta idempotency minima asi:

- header: `Idempotency-Key`
- aplica a tools write
- repo en memoria
- fingerprint estable de `tool + input + context`
- replay de respuesta success
- replay de respuesta blocked
- replay de error usando el mismo error HTTP
- conflicto si la misma key se usa con otro payload
- conflicto si otra request con la misma key sigue en progreso

Outcomes actuales:

- `NEW`
- `REPLAY`
- `IN_PROGRESS`
- `CONFLICT`
- `SKIPPED_NOT_WRITE`

La respuesta tambien expone:

- body `idempotency.present`
- body `idempotency.outcome`
- header `X-Idempotency-Outcome`

### Rate limit actual

Hoy `/run` aplica un rate limit minimo por tool dentro de `prepareExecution`.

Comportamiento actual:

- solo existe rate limit por tool
- la llave actual es `tool:<tool_id>`
- la ventana es de 1 minuto
- el backend actual puede ser `memory` o `redis`
- si no se configura nada, usa `memory`
- si `NEXUS_RATE_LIMIT_BACKEND=redis`, usa `NEXUS_REDIS_URL`
- si `tool.rate_limit_per_minute <= 0`, no limita
- si excede el limite, `/run` devuelve `403` blocked con reason `rate limit exceeded`

No existe todavia:

- rate limit por tenant
- rate limit por principal
- headers de quota o retry

### Egress actual

Hoy `/run` valida el host de salida antes del upstream.

Comportamiento actual:

- se parsea `tool.url`
- si la URL es invalida, devuelve `400 INVALID_TOOL_URL`
- se toma `hostname()` de la URL
- si el checker de egress esta activo, la decision es por `tool_id + host`
- si no hay reglas habilitadas para esa tool, hoy se niega
- si el host no esta permitido, `/run` devuelve `403` blocked con reason `egress host denied`
- si falla el repo de egress, devuelve `500 EGRESS_STORE_ERROR`

No hay endpoint publico de egress en `v2` todavia.

### Secrets actuales

Hoy `/run` resuelve secrets internos antes del upstream y los convierte en headers.

Comportamiento actual:

- siempre agrega `X-Nexus-Request-Id`
- lee secrets por `tool_id`
- ignora secrets deshabilitados
- `secret_type=header` agrega `key_name: plaintext_value`
- `secret_type=bearer` agrega `Authorization: Bearer <plaintext_value>`
- si falla el repo de secrets, devuelve `500 SECRETS_STORE_ERROR`

No hay CRUD publico de secrets en `v2` todavia.

### Policies actuales

Hoy `v2` tiene una capa minima de policy:

- repo en memoria
- evaluacion por `tool_name`
- expresiones CEL
- soporte de `allow` y `deny`
- soporte de `require_approval`
- soporte de `approval_ttl_seconds`
- `deny` bloquea antes del upstream
- `allow + require_approval` crea `intent` y `approval`, y `/run` responde `202 Accepted`
- default actual: si no matchea ninguna policy, se permite ejecutar

Las expresiones pueden mirar:

- `input.*`
- `context.*`
- `tool.name`
- `tool.kind`
- `tool.method`
- `tool.url`

## `/policies`

`v2` ya expone el CRUD completo de `policy` siguiendo la convencion global.

### Endpoints

- `POST /v1/policies`
- `GET /v1/policies`
- `GET /v1/policies/{id}`
- `PATCH /v1/policies/{id}`
- `DELETE /v1/policies/{id}`
- `POST /v1/policies/{id}/archive`
- `POST /v1/policies/{id}/restore`

### Modelo expuesto

```json
{
  "id": "uuid",
  "tool_name": "echo",
  "effect": "deny",
  "priority": 100,
  "expression": "input.hello == \"blocked\"",
  "reason": "blocked by policy",
  "enabled": true,
  "archived": false,
  "archived_at": null,
  "created_at": "2026-03-13T00:00:00Z",
  "updated_at": "2026-03-13T00:00:00Z"
}
```

### Reglas actuales

- `effect` debe ser `allow` o `deny`
- `tool_name` debe existir
- `expression` se valida con CEL en create y patch
- `expression` debe compilar y devolver `bool`
- si `priority` no viene en create, hoy defaultea a `100`
- `DELETE` hace hard delete real
- `archive` hace soft delete
- `restore` restaura un registro archivado
- una policy archivada no se puede modificar con `PATCH` hasta `restore`

### List

`GET /v1/policies` soporta:

- `tool_name`
- `include_archived=true|false`

Comportamiento actual:

- por default no devuelve archivados
- con `include_archived=true` devuelve tambien archivados

### Persistencia actual

El CRUD existe, pero por ahora sigue usando repo en memoria.

## `/approvals`

`v2` ya expone el lifecycle minimo de approvals para cerrar el branch abierto por `/run`.

### Endpoints

- `GET /v1/approvals`
- `GET /v1/approvals/{id}`
- `POST /v1/approvals/{id}/approve`
- `POST /v1/approvals/{id}/reject`

### Modelo expuesto

```json
{
  "id": "uuid",
  "intent_id": "uuid",
  "request_id": "req-123",
  "tool_name": "echo",
  "reason": "operator approval required",
  "status": "pending",
  "decided_by": null,
  "decided_at": null,
  "expires_at": "2026-03-13T00:00:00Z",
  "created_at": "2026-03-13T00:00:00Z",
  "updated_at": "2026-03-13T00:00:00Z"
}
```

### Comportamiento actual

- `GET /v1/approvals` devuelve approvals pendientes
- `GET /v1/approvals/{id}` devuelve un approval especifico, aunque ya este resuelto
- `approve` cambia el status a `approved`
- `reject` cambia el status a `rejected`
- un approval ya decidido no se puede volver a decidir
- aprobar o rechazar actualiza el status del intent vinculado

### Request de approve/reject

Body JSON:

```json
{
  "decided_by": "alice"
}
```

`decided_by` hoy es opcional. Si no viene, queda vacio.

## `/run/intents`

`v2` ya expone lectura minima de intents para inspeccionar lo que `/run` creo cuando una policy requiere approval.

### Endpoints

- `GET /v1/run/intents`
- `GET /v1/run/intents/{id}`
- `GET /v1/run/intents/{id}/preflight`
- `POST /v1/run/intents/{id}/lease`
- `POST /v1/run/intents/{id}/execute`

### Modelo expuesto

```json
{
  "id": "uuid",
  "request_id": "req-123",
  "tool_id": "tool_echo",
  "tool_name": "echo",
  "policy_id": "uuid",
  "risk_class": "mutate_prod",
  "reason": "operator approval required",
  "status": "pending_approval",
  "preflight_status": "passed",
  "preflight_summary": {
    "required": true
  },
  "preflight_completed_at": "2026-03-13T00:00:00Z",
  "approval_id": "uuid",
  "expires_at": "2026-03-13T00:00:00Z",
  "executed_at": "2026-03-13T00:05:00Z",
  "created_at": "2026-03-13T00:00:00Z",
  "updated_at": "2026-03-13T00:00:00Z"
}
```

### Comportamiento actual

- `GET /v1/run/intents` devuelve intents recientes
- soporta `?limit=<n>`
- si `limit` es invalido o `<= 0`, devuelve `400 VALIDATION`
- `GET /v1/run/intents/{id}` devuelve un intent especifico
- si el intent no existe, devuelve `404 NOT_FOUND`
- `GET /v1/run/intents/{id}/preflight` devuelve la vista resumida de preflight del intent
- incluye `risk_class`, `status`, `summary` y `intent_status`
- `POST /v1/run/intents/{id}/lease` emite una lease de ejecucion para un intent aprobado
- la lease expone `id`, `intent_id`, `risk_class`, `status`, `credential_mode`, `credential_hints`, `expires_at` y `used_at`
- la lease es `single-use`: una sola ejecucion la puede consumir, y el consumo es atomico
- si el intent no esta aprobado, devuelve `403 APPROVAL_REQUIRED`
- si el intent vencio, devuelve `403 LEASE_EXPIRED`
- si el preflight fallo, devuelve `403 PREFLIGHT_FAILED`
- `POST /v1/run/intents/{id}/execute` ejecuta el intent guardado
- ahora requiere body JSON:

```json
{
  "lease_id": "uuid"
}
```

- solo ejecuta si el intent esta en `approved`
- si no viene `lease_id`, devuelve `400 VALIDATION`
- si `lease_id` es invalido, devuelve `400 VALIDATION`
- si no existe lease activa para ese intent, devuelve `403` con `reason=execution lease not found` o `execution lease is not active for this intent`
- si la lease vencio, devuelve `403` con `reason=execution lease expired before execution`
- si el intent tiene `preflight_status=failed`, devuelve `403` con `reason=intent preflight failed and cannot be executed`
- si el intent esta `pending_approval`, `rejected` o `executed`, devuelve `403` con `reason=intent is not approved for execution`
- si el intent vencio, devuelve `403` con `reason=intent expired before execution`
- marca la lease como `used` antes de ejecutar
- reutiliza `tool_id`, `tool_name`, `input` y `context` guardados en el intent
- acepta `X-Timeout-Ms` igual que `/run`
- si ejecuta bien, marca el intent como `executed` y completa `executed_at`

## `policy`

El paquete `internal/policy` ahora cubre dos cosas:

- CRUD de policies
- decision previa a la ejecucion del upstream

### Rol en el flujo

La integracion actual ocurre en `Usecases.decide`.

El orden es:

1. `Usecases.Run`
2. `resolveTool`
3. `validateAndPrepare`
4. `decide`
5. `prepareExecution`
6. si la decision permite y la preparacion pasa, recien ahi `executeAndFinish`

O sea, `policy` corta el flujo antes del upstream.

### Modelo actual

La entidad minima es:

```json
{
  "tool_name": "echo",
  "effect": "allow",
  "priority": 1,
  "expression": "input.hello == \"blocked\"",
  "reason": "operator approval required",
  "require_approval": true,
  "approval_ttl_seconds": 3600,
  "enabled": true
}
```

Campos actuales:

- `id`: identificador de la policy
- `tool_name`: tool a la que aplica la policy
- `effect`: hoy puede ser `allow` o `deny`
- `priority`: orden de evaluacion, menor numero primero
- `expression`: expresion CEL
- `reason`: texto que se devuelve si la policy bloquea
- `require_approval`: si el match debe crear approval antes de ejecutar
- `approval_ttl_seconds`: vencimiento del approval e intent cuando aplica
- `enabled`: si esta en `false`, no participa
- `archived`: indica soft delete
- `archived_at`: timestamp del archive
- `created_at`: creacion
- `updated_at`: ultima actualizacion

### Repositorio actual

Hoy solo existe un repo en memoria.

Comportamiento:

- soporta create, list, get, save, delete, archive y restore
- en runtime filtra por `tool_name`
- en runtime ignora policies deshabilitadas
- en runtime ignora archivadas
- ordena por `priority` ascendente
- usa `created_at` como desempate estable

No hay persistencia real todavia.

### Evaluador actual

El evaluador interpreta `expression` con CEL contra tres fuentes:

- `input`
- `context`
- `tool`

Ejemplos de paths validos:

- `input.hello`
- `context.actor`
- `tool.method`
- `tool.url`

Si `expression` viene vacia, hoy matchea siempre.

Las expresiones validas deben devolver `bool`.

La implementacion actual compila la expresion en el primer uso y cachea el programa compilado por string de expresion.

### Forma de una expresion

Una expresion simple tiene esta forma:

```cel
input.hello == "blocked"
```

Ejemplos validos:

- `tool.method == "POST"`
- `context.actor == "pablo"`
- `input.hello.contains("block")`
- `input.hello.matches("^block")`
- `tool.method == "POST" && input.hello == "blocked"`
- `!(context.role == "admin")`

Ejemplo:

```cel
tool.method == "POST" && input.hello.contains("block")
```

### Capacidades que usa hoy

- comparaciones booleanas
- comparaciones numericas
- `&&`
- `||`
- `!`
- acceso a campos por path
- funciones/metodos estandar de CEL como `contains` y `matches`

Notas:

- una expresion invalida falla en compilacion
- una expresion que no devuelve `bool` falla
- create y patch validan la expresion antes de guardar
- `matches` de CEL reemplaza al regex custom que tenia el evaluator anterior

### Regla de decision actual

La decision actual es simple:

1. se listan las policies de la tool ya ordenadas por prioridad
2. se evalua una por una
3. la primera que matchea define el resultado inmediato
4. si matchea una `deny`, `/run` devuelve `403` con `decision=deny` y `status=blocked`
5. si matchea una `allow` con `require_approval=true`, `/run` crea `intent` + `approval` y devuelve `202`
6. si matchea una `allow` sin approval, la ejecucion sigue
7. si no matchea ninguna, hoy el default es `allow`

En otras palabras, se comporta como `first match wins`.

### Approval e intents actuales

Hoy `v2` ya tiene el camino minimo de approval dentro de `/run`:

- repo en memoria de intents
- repo en memoria de approvals
- `risk class` deterministica
- `preflight` deterministico simple
- un solo approval step
- sin quorum
- con approve/reject endpoint
- con preflight endpoint
- con lease endpoint
- con execute-intent endpoint

Comportamiento actual:

- la policy crea un `intent` con status `pending_approval`
- antes de crear el intent, calcula `risk_class` y `preflight`
- si el preflight falla, `/run` bloquea antes de crear intent/approval
- despues crea un `approval` vinculado a ese intent
- la respuesta expone `intent_id` y `approval_id`
- si la request era idempotente, un replay conserva esos mismos IDs
- `GET /v1/approvals` permite listar pendientes
- `POST /v1/approvals/{id}/approve|reject` permite cerrar el approval
- `GET /v1/run/intents` permite inspeccionar intents recientes
- `GET /v1/run/intents/{id}` permite inspeccionar un intent puntual
- `GET /v1/run/intents/{id}/preflight` permite inspeccionar el resumen de preflight
- `POST /v1/run/intents/{id}/lease` emite una lease antes de la ejecucion
- `POST /v1/run/intents/{id}/execute` ejecuta el intent aprobado usando `lease_id`
- cuando ejecuta bien, el intent pasa a `executed`

### Alcance actual

Esto todavia no existe en `v2/policy`:

- `tool_id`
- `org_id`
- approval
- limits
- auditoria
- persistencia en DB
- explicacion detallada de por que matcheo una expresion

### Ejecucion HTTP actual

El adapter `executor/http` hoy soporta:

- `GET`: convierte `input` en query params
- `POST`: convierte `input` en JSON body

La respuesta del upstream se interpreta asi:

- si `Content-Type` contiene `application/json`, se parsea como JSON
- si no, se devuelve como `{ "raw": "..." }`
- si el upstream responde fuera de `2xx`, `/run` falla con `UPSTREAM_ERROR`

### Wiring inicial

Hoy `wire/setup.go` registra en memoria una tool `echo`.

Definicion inicial:

- id: `tool_echo`
- nombre: `echo`
- kind: `http`
- method: `POST`
- url: `NEXUS_TOOL_ECHO_URL`
- enabled: `true`
- rate limit por minuto: `0` por default
- input schema: requiere `hello` string
- output schema: requiere `received` object
- egress habilitado para el host configurado de `echo`

Config actual de `cmd/api`:

- `PORT`
- `NEXUS_TOOL_ECHO_URL`
- `NEXUS_RATE_LIMIT_BACKEND`
- `NEXUS_REDIS_URL`

### No implementado todavia

- auth
- audit
- limits por tenant o principal
- persistencia real de tools
- persistencia real de egress
- persistencia real de secrets
- persistencia real de approvals e intents
- quorum break-glass
