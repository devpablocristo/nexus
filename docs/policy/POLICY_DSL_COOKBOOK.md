# Policy DSL Cookbook

Casos prácticos basados en las estructuras reales del runtime.

## Deny por PII detectada

```json
{
  "effect": "deny",
  "priority": 10,
  "conditions": {
    "all": [
      { "path": "tool.classification", "op": "eq", "value": "external" },
      { "path": "context.dlp.credit_card.count", "op": "gt", "value": 0 }
    ]
  },
  "reason_template": "Credit card data cannot be sent to external tools",
  "enabled": true
}
```

## Require approval para writes

```json
{
  "effect": "allow",
  "priority": 20,
  "conditions": {
    "path": "tool.action_type",
    "op": "eq",
    "value": "write"
  },
  "limits": {
    "require_approval": true,
    "require_idempotency": true
  },
  "reason_template": "Write tools require HITL approval"
}
```

## Rate limit específico por tool

```json
{
  "effect": "allow",
  "priority": 30,
  "conditions": {
    "path": "tool.name",
    "op": "eq",
    "value": "search-api"
  },
  "limits": {
    "rate_limit": { "per_minute": 120 }
  }
}
```

## Deny por actor/rol/contexto

```json
{
  "effect": "deny",
  "priority": 15,
  "conditions": {
    "all": [
      { "path": "context.role", "op": "neq", "value": "admin" },
      { "path": "tool.action_type", "op": "eq", "value": "write" }
    ]
  },
  "reason_template": "Only admins can use write tools"
}
```

## Protección de egress y sensitivity

```json
{
  "effect": "deny",
  "priority": 25,
  "conditions": {
    "all": [
      { "path": "tool.classification", "op": "eq", "value": "external" },
      { "path": "tool.sensitivity", "op": "eq", "value": "high" }
    ]
  },
  "reason_template": "High-sensitivity tools cannot call external targets"
}
```

## Limitar tamaño de payload

```json
{
  "effect": "allow",
  "priority": 40,
  "conditions": {
    "path": "tool.name",
    "op": "eq",
    "value": "documents-api"
  },
  "limits": {
    "max_bytes_input": 32768,
    "max_bytes_context": 16384
  }
}
```
