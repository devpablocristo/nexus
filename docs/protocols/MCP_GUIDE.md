# MCP Guide

Guía del endpoint `POST /mcp` expuesto por `nexus-core`.

## Autenticación y scopes

- Auth soportada: `Authorization: Bearer <JWT>` o `X-NEXUS-CORE-KEY`.
- Scope requerido:
  - `mcp:read` para `tools/list` y `tools/get`
  - `mcp:call` para `tools/call`
- Headers útiles:
  - `Idempotency-Key`
  - `X-Timeout-Ms`

## Envelope JSON-RPC

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "tool_name": "echo",
    "input": { "message": "hello" },
    "context": { "source": "agent" }
  }
}
```

## Métodos soportados

- `tools/list`
- `tools/get`
- `tools/call`

## `tools/list`

Request:

```json
{ "jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": {} }
```

Response:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "items": [
      {
        "name": "echo",
        "kind": "http",
        "method": "POST",
        "url": "https://example.internal/echo",
        "input_schema": {},
        "output_schema": {},
        "action_type": "read",
        "classification": "internal",
        "sensitivity": "low",
        "risk_level": 1,
        "enabled": true
      }
    ]
  }
}
```

## `tools/get`

Params:

```json
{
  "tool_name": "echo"
}
```

## `tools/call`

Params reales:

```json
{
  "request_id": "req-123",
  "tool_name": "echo",
  "input": { "message": "hello" },
  "context": { "source": "agent" },
  "idempotency_key": "idem-123",
  "timeout_ms": 2500
}
```

`tools/call` entra al mismo pipeline determinista que `/v1/run`.

## Errores frecuentes

- Request mal formada: HTTP `400`, envelope JSON-RPC con `error.code = -32600`, `error.data.error_code = VALIDATION_ERROR`.
- Método inexistente: HTTP `200`, envelope JSON-RPC con `error.code = -32601`, `error.data.error_code = NOT_FOUND`.
- Auth/scope insuficiente: HTTP `200`, envelope JSON-RPC con `error.data.error_code = UNAUTHORIZED`.
- Policy deny / approval required / rate limited: HTTP `200`, envelope JSON-RPC con `error.data.http_status` y `error.data.error_code` alineados con el catálogo compartido.

## Límites

- Sigue DLP, policies, approvals, idempotencia, egress, timeout budget y output schema del gateway.
- No bypassa enforcement.
- No expone contratos internos de `nexus-saas` ni de operators.
