# E2E Tests — Nexus

Scripts de pruebas end-to-end. Cada script acepta `--help` para ver su man page.

## Scripts

| # | Script | Scope | Stack propia | Descripción |
|---|--------|-------|:---:|-------------|
| 01 | `01_run_echo.sh` | Mínimo | No | Flujo básico: POST /v1/run con tool echo |
| 02 | `02_run_my_service.sh` | Tool individual | No | Llama a cualquier tool registrada por nombre |
| 03 | `03_full_core_e2e.sh` | Core completo | No | Suite de 12 secciones: CRUD, egress, run, schema, policies, secrets, simulate, idempotency, audit, authz, delete |
| 04 | `04_core_gateway_isolated.sh` | Gateway aislado | Sí | Stack aislada: auth, tools, policies, DLP, idempotency, MCP, A2A, audit, orgs, alerts |
| 05 | `05_core_jwt_auth.sh` | JWT auth | Sí | Modo JWT-only: API key rechazada, Bearer aceptado, run/tools/MCP con JWT |
| 06 | `06_control_operators.sh` | Control operators | No | Health, métricas Prometheus, consumo de eventos, persistencia, detección de anomalías |
| 07 | `07_ai_operators.sh` | AI operators | No | Health, métricas, auth, tick, consumo de eventos, señal high-risk, acciones/incidentes/proposals, assistant |

## Prerequisitos

### Scripts 01, 02, 03, 06 (usan stack existente)

```bash
make up
make migrate-up
make seed
```

### Scripts 04, 05 (levantan su propia stack)

Solo necesitan Docker, curl y jq. Se auto-contienen.

## Ejecución

```bash
# Ver ayuda de cualquier script:
./scripts/e2e/01_run_echo.sh --help

# Ejecutar:
./scripts/e2e/01_run_echo.sh

# Con variables:
NEXUS_HTTP_PORT=18080 ./scripts/e2e/03_full_core_e2e.sh

# Llamar a una tool específica:
./scripts/e2e/02_run_my_service.sh my-custom-tool

# Suite completa autocontenida a nivel Makefile:
make e2e-all
```

## Arquitectura del pipeline testeado

El gateway ejecuta un pipeline determinista (sin LLM):

1. **Auth** → org_id, actor, role, scopes
2. **Tool lookup** → busca tool por nombre/UUID
3. **Idempotency** → para writes: replay/conflict/in-progress
4. **DLP** → detección de PII
5. **Schema validation** → input contra JSON Schema
6. **Policies** → condiciones + allow/deny
7. **Overrides** → rate limits, egress, secrets
8. **Execution** → HTTP al upstream
9. **Audit** → hash-chain, redacción, export

## Operators (background, no en el path síncrono)

| Worker | Servicio | ¿LLM? | Qué hace |
|--------|----------|:-----:|----------|
| Sentry | control-operators | No | Detección de anomalías (EWMA, baselines) |
| Coordinator | control-operators | No | Máquina de estados de incidentes |
| Mitigation | control-operators | No | Dry-run y aplicación de acciones |
| Recovery | control-operators | No | Monitoreo post-mitigación, rollback |
| Diagnostician | ai-operators | Sí | Diagnóstico inteligente |
| Comms | ai-operators | Sí | Comunicaciones de incidentes |
| Executive Q&A | ai-operators | Sí | Asistente para operadores |
