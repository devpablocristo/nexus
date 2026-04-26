# Nexus Companion

Servicio Go que modela **tareas** del “compañero de trabajo” y las integra con **Nexus Governance** mediante HTTP.

## Rol canónico en el ecosistema

- categoría: `ProductAgent` transversal
- nombre comercial: `Companion`
- dependencia soberana: `Nexus Governance` (`GovernanceService`)
- alcance: coordinar trabajo y propuestas multi-sistema bajo governance; no reemplaza a agentes embebidos de producto

## Modelo De Producto

Companion se diseña como un **empleado digital generalista**. Su core no debe depender de un negocio especifico.

Capacidades base:

- conversación;
- tareas;
- memoria;
- observación/watchers;
- planificación;
- ejecución;
- reporting a Nexus.

Las capacidades especificas de un sistema externo viven en **connectors**. Pymes es un adapter concreto de Companion en esta etapa, no una libreria reusable del ecosistema.

## Requisitos

- PostgreSQL (base dedicada `nexus_companion` en la misma instancia que `Nexus Governance`).
- `Nexus Governance` accesible con `NEXUS_BASE_URL` y `NEXUS_API_KEY` (misma clave que un rol válido en `NEXUS_API_KEYS` del servicio de governance, p. ej. `admin`).

## Variables de entorno

| Variable | Obligatoria | Descripción |
|----------|-------------|-------------|
| `PORT` | no | Default `8080` |
| `DATABASE_URL` | sí | Postgres `nexus_companion` |
| `NEXUS_API_KEYS` | sí | API keys con scopes. Legacy `admin=...` sigue funcionando en local/dev |
| `NEXUS_AUTH_ISSUER_URL` | no | Issuer OIDC/JWKS para aceptar Bearer JWT humano además de API key |
| `NEXUS_AUTH_AUDIENCE` | no | Audience esperada del JWT humano (opcional) |
| `REVIEW_BASE_URL` | sí | Ej. `http://review:8080` |
| `REVIEW_API_KEY` | sí | Valor de la API key hacia `Nexus Governance` |
| `COMPANION_REVIEW_SYNC_INTERVAL_SEC` | no | Segundos entre polls de sync; `0` = sin loop. Default `30` |

## Routing detrás del console (nginx / Vite)

- Nexus Governance: `/v1/...`
- Companion: `/companion/v1/...` (el proxy quita el prefijo `/companion/` y reenvía `/v1/...` al servicio Companion).

La UI usa proxy same-origin en `console`; las API keys quedan del lado servidor para desarrollo local. Para acceso remoto seguro, `Companion` acepta `Authorization: Bearer` cuando `NEXUS_AUTH_ISSUER_URL` está configurado.

## Cliente HTTP reutilizable (core)

En el repo **core**:

- `backend/go/httpclient`: `Caller.DoJSON` — usado por `internal/reviewclient` contra `Nexus Governance`.
- `backend/go/fsm`: máquina de estados declarativa (From+Event→To) — reglas de transición de tareas en Companion.

## Flujo “Propose”

1. `POST /v1/tasks/{id}/propose` crea un `TaskAction` y envía `POST /v1/requests` a `Nexus Governance` con `action_type: companion.propose` (requiere migración `0009_companion_action_type`).
2. Metadata en `params.nexus`: `origin`, `task_id`, `proposed_by`, `human_owner`, etc.
3. Se persiste `review_request_id` en la acción. El **estado de la tarea** se deriva de la respuesta de `Nexus Governance` vía FSM: `allowed` → `done` (con `closed_at`), `denied` → `failed`, `pending_approval` → `waiting_for_approval`.

## Sincronización con Nexus Governance

- **Loop** (opcional): `COMPANION_REVIEW_SYNC_INTERVAL_SEC` — intervalo en segundos; por defecto `30`. `0` desactiva el ticker en segundo plano.
- **Manual**: `POST /v1/tasks/{id}/sync` — para tareas en `waiting_for_approval`, consulta el último `review_request_id` del último `propose` y aplica la misma FSM si `Nexus Governance` ya resolvió (`approved`, `rejected`, `expired`, etc.).

## Connectors

Connectors son las manos y ojos de Companion. Viven dentro de Companion hasta que haya un contrato agnostico y reutilizacion real fuera de este producto.

- Read-only operations pueden ejecutarse sin approval.
- Write/side-effect operations requieren Nexus `allowed` o `approved`.
- Cada capability declara `operation`, `mode`, `risk_class`, `requires_review`, `input_schema` y `evidence_fields`.
- Companion reporta el resultado de ejecuciones autorizadas a Nexus con `/result`.

Ver [../doc/CONNECTORS.md](../doc/CONNECTORS.md).

## Frontera De Confianza

Companion deriva `actor_id`, `org_id`, `roles`, `scopes` y `service_principal` desde API key/JWT. Los headers `X-User-ID` y `X-Org-ID` entrantes se descartan en el middleware y se reinyectan desde el principal efectivo.

Ejemplo de API key:

```text
admin=dev-key|service_principal=true|org_id=org-a|scopes=companion:tasks:read+companion:tasks:write+companion:connectors:execute+companion:connectors:admin
```

Tasks, connectors y executions quedan asociados a `org_id`; las respuestas se filtran por tenant cuando el principal lo trae. Toda ejecucion con side effect requiere una request de Nexus `allowed` o `approved`, se persiste como execution y se reporta a Nexus con `/result`.

## Postgres: volumen ya inicializado

Si el volumen de Postgres existía **antes** de añadir `postgres-init`, creá la base manualmente:

```bash
docker compose exec postgres psql -U postgres -c 'CREATE DATABASE nexus_companion;'
```

O recreá el volumen `postgres-data`.

## Arranque local (desde `v3/`)

```bash
docker compose up -d --build
```

Si el volumen de Postgres es antiguo y falta la base:

```bash
bash scripts/dev/ensure-companion-db.sh
```

Smoke E2E Companion → Nexus Governance (con APIs levantadas):

```bash
bash scripts/smoke/run-companion-review-flow.sh
```

- Nexus Governance: `http://localhost:${NEXUS_REVIEW_PORT:-18084}`
- Companion: `http://localhost:${NEXUS_COMPANION_PORT:-18085}`
- Console: `http://localhost:${NEXUS_CONSOLE_PORT:-13001}`

## Limitaciones (slice)

- Sin websockets: la UI hace polling del detalle de tarea; el servidor además puede sincronizar estados en background.
- `investigate` solo cambia estado y opcionalmente agrega mensaje; sin lógica de investigación real.
- Políticas CEL en `Nexus Governance` aplican a `companion.propose` como a cualquier otro `action_type`.
