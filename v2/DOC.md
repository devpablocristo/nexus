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
- los CRUD handlers deben reutilizar `v2/pkgs/go-pkg/handlers`
- el bootstrap HTTP transversal y agnostico debe reutilizar `v2/pkgs/go-pkg/httpserver`
- no se aceptan helpers HTTP locales tipo `decodeJSON`, `writeJSON`, `parseLimit`, `parseArchived`
- `./scripts/quality/check-crud-pattern.sh` hace cumplir esta regla en `qa`
- `qa` tambien valida `v2/pkgs/go-pkg`
- `make milestone` corre `qa + acceptance`

Patron esperado:

- `POST /v1/<resource>`
- `GET /v1/<resource>`
- `GET /v1/<resource>/{id}`
- `PATCH /v1/<resource>/{id}`
- `DELETE /v1/<resource>/{id}` para hard delete
- `POST /v1/<resource>/{id}/archive` para soft delete
- `POST /v1/<resource>/{id}/restore` para restaurar soft delete

## Mapa actual por servicio

- `control-plane`
  - administra `resources`
  - administra `action policies`
  - administra lectura de `audit`
  - expone write interno de `audit`

- `control-workers`
  - opera `incidents`
  - opera `alerts`

- `data-plane`
  - ejecuta `/actions`
  - decide lifecycle de `actions`
  - emite `audit`
  - abre `incidents` deterministas cuando corresponde

## Auth minima actual

La auth minima del MVP ya esta activa.

- todos los endpoints de negocio requieren API key
- `/healthz` y `/readyz` quedan libres
- se acepta `X-API-Key`
- tambien se acepta `Authorization: Bearer <key>`
- cada servicio valida auth inbound con `NEXUS_API_KEYS`
- `data-plane` autentica `control-plane` y `control-workers` con:
  - `NEXUS_CONTROL_PLANE_API_KEY`
  - `NEXUS_CONTROL_WORKERS_API_KEY`
- `control-workers` autentica `control-plane` con:
  - `NEXUS_CONTROL_PLANE_API_KEY`

## Persistencia principal actual

Las superficies principales ya pueden correr en memoria o con PostgreSQL.

- `control-plane/resources` usa `resources.InMemoryRepository` o `resources.PostgresRepository`
- `control-plane/policies` usa `policies.InMemoryRepository` o `policies.PostgresRepository`
- `control-plane/audit` usa `audit.InMemoryRepository` o `audit.PostgresRepository`
- `data-plane/actions` usa `action.InMemoryRepository` o `action.PostgresRepository`
- `control-workers/incidents` usa `incidents.InMemoryRepository` o `incidents.PostgresRepository`
- `control-workers/alerts` usa `alerts.InMemoryRepository` o `alerts.PostgresRepository`
- en `docker compose`, esas seis superficies ya corren con PostgreSQL real y sobreviven restart

## `/audit` en `control-plane`

`audit` es la traza inmutable de Nexus.

No sigue el patron CRUD normal:

- no tiene `PATCH`
- no tiene `DELETE`
- no tiene `archive`
- no tiene `restore`

Superficie actual:

- `POST /internal/audit`
- `GET /v1/audit`
- `GET /v1/audit/{id}`

### Contrato actual

`POST /internal/audit` es write interno entre servicios.

Campos actuales:

- `event_type`
- `source_service`
- `action_id`
- `resource_id`
- `resource_type`
- `actor`
- `summary`
- `data`
- `occurred_at`

`GET /v1/audit` soporta:

- `action_id`
- `resource_id`
- `actor_id`
- `event_type`
- `from`
- `to`
- `limit`

### Persistencia actual

- `v2/pkgs/go-pkg/postgres` centraliza pool PostgreSQL y runner de migrations
- la estrategia actual de migrations es `up-only`, con SQL numerado y embebido por modulo
- `control-plane/resources` usa PostgreSQL cuando `NEXUS_CONTROL_PLANE_DATABASE_URL` esta configurado
- `control-plane/policies` usa PostgreSQL cuando `NEXUS_CONTROL_PLANE_DATABASE_URL` esta configurado
- `resources` y `policies` comparten el mismo pool y la misma base de `control-plane`
- `docker compose` ya levanta `control-plane-postgres` con volumen persistente `control-plane-postgres-data`
- `control-plane` usa PostgreSQL para `audit` cuando `NEXUS_AUDIT_DATABASE_URL` esta configurado
- si no hay `NEXUS_AUDIT_DATABASE_URL`, cae a repo en memoria
- en `docker compose`, `audit` ya corre con PostgreSQL real y volumen persistente `audit-postgres-data`

### Emision actual

`control-plane` emite `audit` localmente para cambios admin en:

- `resources`
- `policies`

`data-plane` emite `audit` best effort hacia `control-plane` para:

- `action_created`
- `action_blocked`
- `action_approved`
- `action_rejected`
- `action_leased`
- `action_executed`
- `action_execution_failed`

`control-workers` emite `audit` best effort hacia `control-plane` para:

- `incident_created`
- `alert_created`

### Comportamiento de fallo actual

- si falla el write inter-servicio de `audit`, la operacion principal no se revierte
- el fallo no se silencia: queda logueado en el proceso emisor
- los cambios admin locales de `control-plane` tambien hacen write best effort y loguean si falla

### Flujo interno actual

- `POST /internal/audit`
  - `audit.Handler.createInternal`
  - `handlers.DecodeJSON`
  - `audit.Usecases.Create`
  - `audit.InMemoryRepository.Create` o `audit.PostgresRepository.Create`
- `GET /v1/audit`
  - `audit.Handler.list`
  - `handlers.ParseLimit`
  - `audit.Usecases.List`
  - `audit.InMemoryRepository.List` o `audit.PostgresRepository.List`
- `GET /v1/audit/{id}`
  - `audit.Handler.getByID`
  - `audit.Usecases.GetByID`
  - `audit.InMemoryRepository.GetByID` o `audit.PostgresRepository.GetByID`

## `/incidents` en `control-workers`

`control-workers` abre y opera incidentes deterministas a partir de eventos del dominio.

### Endpoints actuales

- `POST /v1/incidents`
- `GET /v1/incidents`
- `GET /v1/incidents/{id}`
- `PATCH /v1/incidents/{id}`
- `DELETE /v1/incidents/{id}`
- `POST /v1/incidents/{id}/archive`
- `POST /v1/incidents/{id}/restore`

### Scope actual

`source_kind` soportados:

- `action`

`trigger` soportados:

- `blocked_action`
- `approval_rejected`
- `approval_expired`
- `execution_failed`

`status` soportados:

- `open`
- `acknowledged`
- `resolved`

`severity` soportados:

- `low`
- `medium`
- `high`
- `critical`

`risk_level` soportados:

- `low`
- `medium`
- `high`
- `critical`

Campos actuales:

- `id`
- `source_kind`
- `source_id`
- `action_type`
- `resource_id`
- `resource_type`
- `trigger`
- `risk_level`
- `severity`
- `status`
- `summary`
- `reason`
- `details`
- `archived_at`
- `resolved_at`
- `created_at`
- `updated_at`

### Comportamiento actual

- `POST /v1/incidents` abre un incidente nuevo
- la severidad se deriva de forma determinista a partir de `trigger + risk_level`
- si `summary` no viene, se deriva automaticamente a partir de `trigger + action_type`
- el incidente arranca en `status=open`
- al crearse, emite `incident_created` a `control-plane /internal/audit` si `NEXUS_CONTROL_PLANE_URL` esta configurado
- `PATCH /v1/incidents/{id}` hoy permite actualizar `status`, `summary`, `reason` y `details`
- cuando `status=resolved`, se completa `resolved_at`
- cuando el status vuelve a `open` o `acknowledged`, `resolved_at` se limpia
- si la severidad derivada es `high` o `critical`, intenta abrir un alert determinista
- `DELETE` hace hard delete
- `archive` hace soft delete
- `restore` restaura un incidente archivado
- `GET /v1/incidents` soporta filtros por `source_kind`, `trigger`, `severity`, `status`, `archived` y `limit`

### Flujo interno actual

- `POST /v1/incidents`
  - `incidents.Handler.create`
  - `incidents.Usecases.Create`
  - `incidents.normalizeCreate`
  - `incidents.deriveSeverity`
  - `incidents.deriveSummary`
  - `incidents.InMemoryRepository.Create` o `incidents.PostgresRepository.Create`
  - `incidents.Usecases.emitAlert`
  - `[si severity=high|critical] alerts.Usecases.Create`
- `GET /v1/incidents`
  - `incidents.Handler.list`
  - `handlers.ParseLimit`
  - `handlers.ParseArchived`
  - `incidents.Usecases.List`
  - `incidents.InMemoryRepository.List` o `incidents.PostgresRepository.List`
- `PATCH /v1/incidents/{id}`
  - `incidents.Handler.updateByID`
  - `incidents.Usecases.UpdateByID`
  - `incidents.InMemoryRepository.GetByID` o `incidents.PostgresRepository.GetByID`
  - `incidents.InMemoryRepository.Update` o `incidents.PostgresRepository.Update`
- `POST /v1/incidents/{id}/archive`
  - `incidents.Handler.archiveByID`
  - `incidents.Usecases.ArchiveByID`
  - `incidents.InMemoryRepository.Archive` o `incidents.PostgresRepository.Archive`
- `POST /v1/incidents/{id}/restore`
  - `incidents.Handler.restoreByID`
  - `incidents.Usecases.RestoreByID`
  - `incidents.InMemoryRepository.Restore` o `incidents.PostgresRepository.Restore`

### Integracion actual con `data-plane/actions`

Si `data-plane` corre con `NEXUS_CONTROL_WORKERS_URL`, hoy abre incidentes automaticamente en estos casos:

- una accion creada queda `blocked`
- una accion pendiente de approval es `rejected`
- una accion falla durante `execute`

La integracion actual es explicita pero no autoritativa:

- `data-plane` sigue decidiendo aunque `control-workers` no este disponible
- si la apertura del incidente falla, la transicion principal de `Action` no se revierte
- si la apertura del incidente falla, hoy queda logueado en el proceso del `data-plane`

### Integracion actual con `alerts`

Si `control-workers` crea un incidente con severidad suficiente, hoy abre alerts automaticamente asi:

- `severity=critical` -> `channel=pagerduty`, `route=ops-p1`
- `severity=high` -> `channel=slack`, `route=ops-p2`
- `severity=medium` o `low` -> no abre alert automatico

La integracion actual tambien es no autoritativa:

- el incidente se crea aunque falle la apertura del alert
- la falla de alerting no revierte la apertura del incidente

### Limites actuales de `incidents`

- el repo actual puede ser en memoria o PostgreSQL
- la apertura automatica hoy solo cubre `blocked_action`, `approval_rejected` y `execution_failed`
- `approval_expired` todavia no se abre automaticamente desde `data-plane`
- el alerting automatico hoy solo depende de `severity`
- todavia no hay event bus ni ejecucion async

## `/alerts` en `control-workers`

`control-workers` usa `alerts` como outbox determinista de notificaciones operativas.

### Endpoints actuales

- `POST /v1/alerts`
- `GET /v1/alerts`
- `GET /v1/alerts/{id}`
- `PATCH /v1/alerts/{id}`
- `DELETE /v1/alerts/{id}`
- `POST /v1/alerts/{id}/archive`
- `POST /v1/alerts/{id}/restore`

### Scope actual

`source_kind` soportados:

- `incident`

`channel` soportados:

- `slack`
- `pagerduty`
- `email`

`status` soportados:

- `pending`
- `dispatched`
- `suppressed`
- `acknowledged`

`severity` soportados:

- `low`
- `medium`
- `high`
- `critical`

Campos actuales:

- `id`
- `source_kind`
- `source_id`
- `channel`
- `route`
- `severity`
- `status`
- `summary`
- `body`
- `details`
- `archived_at`
- `created_at`
- `updated_at`

### Comportamiento actual

- `POST /v1/alerts` abre un alert nuevo
- `status` default es `pending`
- al crearse, emite `alert_created` a `control-plane /internal/audit` si `NEXUS_CONTROL_PLANE_URL` esta configurado
- `PATCH /v1/alerts/{id}` hoy permite actualizar `status`, `summary`, `body` y `details`
- `DELETE` hace hard delete
- `archive` hace soft delete
- `restore` restaura un alert archivado
- `GET /v1/alerts` soporta filtros por `source_kind`, `channel`, `severity`, `status`, `archived` y `limit`
- los alerts automaticos hoy se abren solo desde `incidents`

### Flujo interno actual

- `POST /v1/alerts`
  - `alerts.Handler.create`
  - `alerts.Usecases.Create`
  - `alerts.normalizeCreate`
  - `alerts.InMemoryRepository.Create` o `alerts.PostgresRepository.Create`
- `GET /v1/alerts`
  - `alerts.Handler.list`
  - `handlers.ParseLimit`
  - `handlers.ParseArchived`
  - `alerts.Usecases.List`
  - `alerts.InMemoryRepository.List` o `alerts.PostgresRepository.List`
- `PATCH /v1/alerts/{id}`
  - `alerts.Handler.updateByID`
  - `alerts.Usecases.UpdateByID`
  - `alerts.InMemoryRepository.GetByID` o `alerts.PostgresRepository.GetByID`
  - `alerts.InMemoryRepository.Update` o `alerts.PostgresRepository.Update`
- `POST /v1/alerts/{id}/archive`
  - `alerts.Handler.archiveByID`
  - `alerts.Usecases.ArchiveByID`
  - `alerts.InMemoryRepository.Archive` o `alerts.PostgresRepository.Archive`
- `POST /v1/alerts/{id}/restore`
  - `alerts.Handler.restoreByID`
  - `alerts.Usecases.RestoreByID`
  - `alerts.InMemoryRepository.Restore` o `alerts.PostgresRepository.Restore`

### Limites actuales de `alerts`

- el repo actual puede ser en memoria o PostgreSQL
- alerting todavia no entrega a Slack/PagerDuty reales
- hoy funciona como outbox/registro determinista interno
- routing y canales son fijos por severidad
- todavia no hay playbooks ni dispatch async

No hay CRUD publico de secrets en `v2` todavia.

## `/resources`

`control-plane` administra la fuente de verdad de los recursos protegidos que despues usan las acciones del `data-plane`.

### Endpoints actuales

- `POST /v1/resources`
- `GET /v1/resources`
- `GET /v1/resources/{id}`
- `PATCH /v1/resources/{id}`
- `DELETE /v1/resources/{id}`
- `POST /v1/resources/{id}/archive`
- `POST /v1/resources/{id}/restore`

### Scope actual

- `type` soportados:
  - `wallet`
  - `treasury`
  - `vault`
- `criticality` soportados:
  - `low`
  - `medium`
  - `high`
  - `critical`

Campos actuales:

- `id`
- `type`
- `name`
- `environment`
- `chain`
- `labels`
- `criticality`
- `archived_at`
- `created_at`
- `updated_at`

### Semantica CRUD actual

- `DELETE /v1/resources/{id}` hace hard delete
- `POST /v1/resources/{id}/archive` hace soft delete
- `POST /v1/resources/{id}/restore` restaura un recurso archivado
- `GET /v1/resources` oculta archivados por default
- `PATCH /v1/resources/{id}` no permite modificar un recurso archivado
- las mutaciones admin intentan emitir `audit`; si falla, no revierten la operacion

### Flujo interno actual

- `POST /v1/resources`
  - `resources.Handler.create`
  - `actors.FromRequest`
  - `resources.Usecases.Create`
  - `resources.normalizeCreate`
  - `resources.InMemoryRepository.Create` o `resources.PostgresRepository.Create`
  - `resources.Handler.emitAudit`
  - `audit.SinkAdapter.Write`
- `GET /v1/resources`
  - `resources.Handler.list`
  - `handlers.ParseLimit`
  - `handlers.ParseArchived`
  - `resources.Usecases.List`
  - `resources.InMemoryRepository.List` o `resources.PostgresRepository.List`
- `PATCH /v1/resources/{id}`
  - `resources.Handler.updateByID`
  - `actors.FromRequest`
  - `resources.Usecases.UpdateByID`
  - `resources.InMemoryRepository.Update` o `resources.PostgresRepository.Update`
  - `resources.Handler.emitAudit`
  - `audit.SinkAdapter.Write`
- `POST /v1/resources/{id}/archive`
  - `resources.Handler.archiveByID`
  - `actors.FromRequest`
  - `resources.Usecases.ArchiveByID`
  - `resources.InMemoryRepository.Archive` o `resources.PostgresRepository.Archive`
  - `resources.Handler.emitAudit`
  - `audit.SinkAdapter.Write`
- `POST /v1/resources/{id}/restore`
  - `resources.Handler.restoreByID`
  - `actors.FromRequest`
  - `resources.Usecases.RestoreByID`
  - `resources.InMemoryRepository.Restore` o `resources.PostgresRepository.Restore`
  - `resources.Handler.emitAudit`
  - `audit.SinkAdapter.Write`

### Limites actuales de `resources`

- el repo actual puede ser en memoria o PostgreSQL
- todavia no existe ownership por tenant/org
- `data-plane/actions` consume estos resources cuando corre con `NEXUS_CONTROL_PLANE_URL`
- si `control-plane` no esta configurado, `actions` vuelve al fallback local sin fuente de verdad externa

## `/policies` en `control-plane`

`control-plane` administra las action policies que despues consume `data-plane/actions`.

### Endpoints actuales

- `POST /v1/policies`
- `GET /v1/policies`
- `GET /v1/policies/{id}`
- `PATCH /v1/policies/{id}`
- `DELETE /v1/policies/{id}`
- `POST /v1/policies/{id}/archive`
- `POST /v1/policies/{id}/restore`

### Scope actual

Campos actuales:

- `id`
- `action_type`
- `resource_type`
- `effect`
- `priority`
- `expression`
- `reason`
- `require_approval`
- `approval_ttl_seconds`
- `enabled`
- `archived_at`
- `created_at`
- `updated_at`

### Semantica actual

- `effect` soporta `allow` o `deny`
- `expression` se valida con CEL al crear o actualizar
- CEL ve dos variables: `action` y `resource`
- `DELETE /v1/policies/{id}` hace hard delete
- `POST /v1/policies/{id}/archive` hace soft delete
- `POST /v1/policies/{id}/restore` restaura una policy archivada
- `GET /v1/policies` soporta `action_type`, `resource_type` y `archived`
- `data-plane/actions` usa first match wins sobre las policies ordenadas por `priority`
- las mutaciones admin intentan emitir `audit`; si falla, no revierten la operacion

### Flujo interno actual

- `POST /v1/policies`
  - `policies.Handler.create`
  - `actors.FromRequest`
  - `policies.Usecases.Create`
  - `policies.Evaluator.Validate`
  - `policies.InMemoryRepository.Create` o `policies.PostgresRepository.Create`
  - `policies.Handler.emitAudit`
  - `audit.SinkAdapter.Write`
- `GET /v1/policies`
  - `policies.Handler.list`
  - `policies.Usecases.List`
  - `policies.InMemoryRepository.List` o `policies.PostgresRepository.List`
- `PATCH /v1/policies/{id}`
  - `policies.Handler.patchByID`
  - `actors.FromRequest`
  - `policies.Usecases.UpdateByID`
  - `policies.Evaluator.Validate`
  - `policies.InMemoryRepository.GetByID` o `policies.PostgresRepository.GetByID`
  - `policies.InMemoryRepository.Save` o `policies.PostgresRepository.Save`
  - `policies.Handler.emitAudit`
  - `audit.SinkAdapter.Write`
- `POST /v1/policies/{id}/archive`
  - `policies.Handler.archiveByID`
  - `actors.FromRequest`
  - `policies.Usecases.ArchiveByID`
  - `policies.InMemoryRepository.ArchiveByID` o `policies.PostgresRepository.ArchiveByID`
  - `policies.Handler.emitAudit`
  - `audit.SinkAdapter.Write`
- `POST /v1/policies/{id}/restore`
  - `policies.Handler.restoreByID`
  - `actors.FromRequest`
  - `policies.Usecases.RestoreByID`
  - `policies.InMemoryRepository.RestoreByID` o `policies.PostgresRepository.RestoreByID`
  - `policies.Handler.emitAudit`
  - `audit.SinkAdapter.Write`

### Limites actuales de `policies`

- todavia no existe publicacion/versionado de policies
- `data-plane/actions` consume estas policies via `control-plane`, no por embedding local
