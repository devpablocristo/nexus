# Policy DSL Reference

Referencia canónica del lenguaje de policies que evalúa `nexus-core` en el pipeline determinista de `/v1/run`, `/v1/run/simulate`, `/mcp` y `/a2a/call`.

## Modelo de evaluación

- Owner: `data-plane/internal/policy` y `data-plane/internal/gateway`.
- Semántica: first-match por `priority asc, created_at asc`.
- Effect soportado: `allow` o `deny`.
- Default del runtime:
  - tools `read` sin match explícito: allow.
  - tools `write` sin match explícito: deny.
- Una policy no ejecuta side effects. Solo decide y aporta límites al pipeline.

## Forma del documento

```json
{
  "effect": "deny",
  "priority": 10,
  "conditions": {
    "all": [
      { "path": "tool.action_type", "op": "eq", "value": "write" },
      { "path": "context.role", "op": "neq", "value": "admin" }
    ]
  },
  "limits": {
    "require_approval": true,
    "require_idempotency": true,
    "rate_limit": { "per_minute": 60 },
    "max_bytes_input": 32768,
    "max_bytes_context": 16384
  },
  "reason_template": "Write tool requires approval for non-admin actors",
  "enabled": true
}
```

## Namespaces de `path`

- `input.*`: payload validado contra el input schema de la tool.
- `context.*`: contexto enriquecido por auth/DLP/runtime. Ejemplos reales: `context.role`, `context.scopes`, `context.auth_method`, `context.dlp.credit_card.count`.
- `tool.*`: metadata de la tool. Paths soportados: `name`, `kind`, `method`, `url`, `action_type`, `classification`, `sensitivity`, `risk_level`.

Namespaces no reconocidos no rompen el request: simplemente no hacen match.

## Operadores soportados

| Operador | Semántica |
|----------|-----------|
| `exists` | true si el path existe y no es `null` |
| `not_exists` | true si el path no existe o es `null` |
| `eq` / `neq` | comparación escalar |
| `lt` / `lte` / `gt` / `gte` | comparación numérica |
| `in` | membership sobre array literal |
| `contains` | substring para strings, membership para arrays |
| `regex` | regex compilada en Go; error si el patrón es inválido o >1024 chars |

Operadores desconocidos tampoco rompen el request: la condición resulta `false`.

## Composición lógica

- `all`: todas las subcondiciones deben matchear.
- `any`: al menos una subcondición debe matchear.
- `not`: niega la subcondición anidada.

Ejemplo:

```json
{
  "any": [
    { "path": "context.role", "op": "eq", "value": "admin" },
    {
      "all": [
        { "path": "tool.action_type", "op": "eq", "value": "read" },
        { "path": "context.scopes", "op": "contains", "value": "gateway:run" }
      ]
    }
  ]
}
```

## Límites soportados

Los límites viven en `limits` y son aplicados por el gateway, no por el evaluator puro.

- `rate_limit.per_minute`: override de RPM por tool/policy.
- `require_idempotency`: bloquea writes sin `Idempotency-Key`.
- `require_approval`: convierte el match en `APPROVAL_REQUIRED`.
- `max_bytes_input`: corta requests con input demasiado grande.
- `max_bytes_context`: corta requests con context demasiado grande.

## Errores contractuales esperados

- `VALIDATION_ERROR`: JSON inválido, patch inválido, effect inválido.
- `POLICY_DENIED`: una policy `deny` bloquea la ejecución.
- `APPROVAL_REQUIRED`: la policy exige HITL.
- `IDEMPOTENCY_REQUIRED`: write tool sin idempotency cuando la policy lo exige.
- `RATE_LIMITED`: rate limit del tenant o de la tool/policy.

## Ejemplos válidos

```json
{ "path": "context.role", "op": "eq", "value": "admin" }
```

```json
{
  "all": [
    { "path": "tool.classification", "op": "eq", "value": "external" },
    { "path": "context.dlp.credit_card.count", "op": "gt", "value": 0 }
  ]
}
```

## Ejemplos inválidos o no efectivos

- Regex inválida:

```json
{ "path": "input.name", "op": "regex", "value": "[" }
```

- Namespace no soportado:

```json
{ "path": "tenant.plan_code", "op": "eq", "value": "growth" }
```

- Operador no soportado:

```json
{ "path": "input.amount", "op": "starts_with", "value": "1" }
```
