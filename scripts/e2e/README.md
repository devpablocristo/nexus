# E2E Tests — Nexus

Scripts de pruebas end-to-end para validar el flujo completo del sistema.

## Primer caso: POST /v1/run con tool `echo`

El caso más simple para probar el pipeline del gateway.

### Consumer (quién llama)

El **consumer** es quien invoca el gateway:

- **REST**: curl, script, Postman, o cualquier cliente HTTP
- **MCP**: cliente MCP que llama `tools/call`
- **A2A**: otro agente que llama `/a2a/call`
- **SDK**: Python/TypeScript SDK

En el primer caso usamos **curl** como consumer.

### Cómo maneja el gateway la petición

El gateway ejecuta un **pipeline determinista** (sin LLM) en este orden:

1. **Auth** — Resuelve org_id, actor, role, scopes desde API key/JWT
2. **Tool lookup** — Busca la tool por `(org_id, tool_name)`
3. **Idempotencia** — Para writes: verifica Idempotency-Key
4. **Validación** — Context, DLP summary, schema de input
5. **Políticas** — Evalúa condiciones (first-match), decide allow/deny
6. **Overrides** — Rate limits, egress allowlist, secrets
7. **Ejecución** — HTTP al upstream (mock-tools en demo)
8. **Output** — Validación de schema, auditoría, respuesta

Para `echo` (read tool): no requiere idempotency, políticas por defecto allow.

### Operators que intervienen en este primer caso

**Ninguno.** En el path síncrono de `/v1/run` no intervienen operators.

Los **operators** son workers asíncronos que consumen eventos del event store (`ops_event_store`):

| Worker | Consume | Produce | ¿LLM? |
|--------|---------|---------|-------|
| Sentry | `tool_call.finished`, `policy.denied`, `quota.exceeded` | `anomaly.detected`, `incident.opened` | No |
| Coordinator | `incident.*`, `diagnosis.created`, `action.*` | `incident.state_changed` | No |
| Diagnostician | incidentes abiertos | `diagnosis.created`, `recommended_actions.created` | Sí |
| Mitigation | `recommended_actions.created` | `action.proposed`, `action.applied` | No |
| Recovery | `action.applied`, `tool_call.finished` | `incident.state_changed`, `action.rolled_back` | No |
| Comms | `incident.*`, `diagnosis.created` | `comms.draft_created`, `comms.sent_internal` | Sí |
| Executive Q&A | consultas de operador | respuesta + propuestas | Sí |

**Nota**: El gateway escribe en `audit_events`. Un trigger DB copia a `operational_events` con `tool.call.completed`. Los consumers de `ops_event_store` esperan `tool_call.finished` — actualmente el bridge entre audit y ops_event_store puede estar en evolución.

### ¿Es determinista? ¿Hay IA?

| Componente | Determinista | IA/LLM |
|------------|--------------|---------|
| Gateway `/v1/run` | **Sí** | **No** |
| Políticas (conditions) | **Sí** | No |
| DLP (PII detection) | **Sí** | No |
| Sentry, Coordinator, Mitigation, Recovery | **Sí** (reglas fijas) | No |
| Diagnostician, Comms, Executive Q&A | **No** | **Sí** |

El enforcement del gateway es 100% determinista. La IA entra en los agents de diagnóstico, comunicaciones y Q&A del operador, que corren **después** de que ocurren incidentes.

## Ejecución

```bash
# Prerrequisitos: make up, make migrate-up, make seed
# Desde la raíz del repo:
./scripts/e2e/01_run_echo.sh
```

O con variables explícitas:

```bash
export NEXUS_HTTP_PORT=8080
export NEXUS_DEMO_API_KEY="nexus-core-local-key"  # del seed
./scripts/e2e/01_run_echo.sh
```
