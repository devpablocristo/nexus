# A2A Guide

Guía del endpoint `POST /a2a/call` de `nexus-core`.

## Qué es

`/a2a/call` es una superficie HTTP explícita para integraciones agente-a-agente que quieren un contrato REST/JSON directo en vez de JSON-RPC.

## Auth y permisos

- Auth: JWT o `X-NEXUS-CORE-KEY`.
- Scope requerido: `a2a:call`.
- Headers opcionales:
  - `Idempotency-Key`
  - `X-Timeout-Ms`

## Request

```json
{
  "request_id": "req-789",
  "tool_name": "echo",
  "input": { "message": "hello" },
  "context": { "source": "agent-b" },
  "timeout_ms": 2500,
  "idempotency_key": "idem-123"
}
```

`timeout_ms` e `idempotency_key` pueden venir en body o por header. Si faltan en body, el handler usa los headers.

## Response

```json
{
  "request_id": "req-789",
  "decision": "allow",
  "tool_name": "echo",
  "status": "success",
  "reason": "",
  "result": { "ok": true },
  "error": { "code": "", "message": "" },
  "latency_ms": 41,
  "idempotency": {
    "present": true,
    "outcome": "new"
  }
}
```

## Qué puede y qué no puede hacer

Puede:

- invocar tools vía el gateway determinista
- aprovechar auth, DLP, policies, approvals e idempotencia
- recibir la decisión y el resultado normalizado

No puede:

- saltar `/v1/run` ni el pipeline de enforcement
- decidir approvals por fuera de los endpoints de approvals
- escribir directamente en DB
- hablar con `nexus-ai-operators` o `nexus-control-operators` como si fueran surface pública

## Relación con approvals y policies

- Si una policy hace `deny`, la respuesta sale bloqueada con `POLICY_DENIED`.
- Si una policy requiere HITL, el response usa `APPROVAL_REQUIRED`.
- Si una policy requiere idempotencia y falta la key, el response usa `IDEMPOTENCY_REQUIRED`.
