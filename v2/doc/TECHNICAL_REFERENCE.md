# Nexus v2 Technical Reference

Relacionado:

- [README.md](README.md)
- [DEFINITION.md](DEFINITION.md)
- [MVP.md](MVP.md)
- [ENDPOINT_FLOWS.md](ENDPOINT_FLOWS.md)
- [ROADMAP.md](ROADMAP.md)
- [OPS.md](OPS.md)

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
- artefactos agnosticos y multilenguaje viven en `v2/pkgs/contracts`
- codigo Go agnostico compartido vive en `v2/pkgs/go-pkg`
- infraestructura AWS de `v2` vive en `v2/infra`

Patron esperado:

- `POST /v1/<resource>`
- `GET /v1/<resource>`
- `GET /v1/<resource>/{id}`
- `PATCH /v1/<resource>/{id}`
- `DELETE /v1/<resource>/{id}` para hard delete
- `POST /v1/<resource>/{id}/archive` para soft delete
- `POST /v1/<resource>/{id}/restore` para restaurar soft delete

## Mapa actual por servicio

- `data-plane` (engine determinista)
  - ejecuta `/actions`
  - decide lifecycle de `actions`
  - emite `audit`
  - abre `incidents` deterministas cuando corresponde
  - ante anomalias, notifica al agente IA (ai-runtime)

- `control-workers` (operadores deterministas)
  - opera `incidents`
  - opera `alerts`
  - ejecuta playbooks y side-effects

- `ai-runtime` (agente IA de Nexus)
  - recibe anomalias del engine
  - notifica a los equipos responsables con contexto y opciones
  - unico interlocutor para humanos via chat (web + app)
  - no decide allow/deny — presenta opciones; el humano ejecuta

- `control-plane` (administracion + SaaS)
  - administra `resources`, `action policies`, `audit`
  - expone write interno de `audit`
  - integra saas-core: billing, auth, tenancy, metering, admin

## Estado actual de Fase 1A

`Fase 1A` ya esta activa en runtime sobre `data-plane/internal/action/risk` y `control-plane`.

- reemplaza el scoring fijo anterior por un evaluator dedicado
- separa `risk_pressure` y `safety_pressure`
- aplica amplificaciones y atenuaciones con cap conservador
- usa hysteresis en bordes de decision
- calcula contexto historico desde PostgreSQL:
  - baselines por `resource`
  - baselines por `actor`
  - `known destinations` con decay
  - incidentes abiertos para `recent_incident`
- soporta canaries via:
  - `control-plane/resources.is_canary`
  - label interna `_nexus_trap`
  - trap policy builtin `is_trap=true`
  - incidente `canary_triggered` y alert `critical`
- hoy corre con un `RiskProfile` builtin:
  - `name=balanced`
  - `version=1`
- la administracion versionada de `RiskProfile` desde `control-plane` todavia no existe

### Factores activos hoy

- `amount_anomaly`
  - en cold start usa peso reducido `0.05`
  - excepto si `resource.criticality == critical`, donde mantiene `0.15`
- `velocity_spike`
- `new_destination`
  - se activa en `withdrawal` cuando hay `destination_address`
  - en cold start se trata como destino nuevo
- `off_hours`
- `actor_deviation`
- `recent_incident`
- `known_destination`
- `within_baseline`
- `business_hours`
- `verified_actor`

Factores adicionales de `1A`:

- cada factor expone `evidence_quality`
- `typical_hours` tiene peso bajo y no domina solo
- la confidence de baseline es saturante por metrica, no lineal

### Salida actual de riesgo

`GET /v1/actions/{id}/risk` y el campo `risk` de `ActionResponse` ya exponen:

- `level`
- `score`
- `summary`
- `profile`
- `risk_pressure`
- `safety_pressure`
- `raw_score`
- `decision_score`
- `recommended_decision`
- `factors`
- `amplifications`
- `attenuations`

La `recommended_decision` de riesgo es informativa por ahora.

- todavia NO reemplaza el decision lifecycle de `actions`
- `allow / deny / require_approval` sigue saliendo de policy evaluation
- el evaluator de `1A` mejora explicabilidad, scoring contextual y deteccion de reconocimiento

### Canaries y trap policies

`control-plane` agrega una trap policy builtin:

- `action_type="*"`
- `resource_type="*"`
- `expression=resource.labels["_nexus_trap"] == "true"`
- `is_trap=true`

Cuando una accion matchea esa policy:

- la accion queda `blocked`
- el incidente usa `trigger=canary_triggered`
- la severidad sube a `critical`
- `control-workers` enruta la alerta como `ops-p1`

## Idempotencia

`POST /v1/actions` soporta el header `Idempotency-Key`.

- si el key ya fue usado dentro del TTL (24h), retorna la respuesta cacheada con header `X-Idempotency-Replay: true`
- si el key no existe, crea la accion normalmente y cachea la respuesta
- approve/reject/lease/execute usan idempotencia semantica via state machine (no key generica)
- la tabla `idempotency_keys` en PostgreSQL tiene purge automatico por TTL

## Graceful degradation

El `data-plane` cachea resources y policies del `control-plane` localmente.

- soft TTL: 30s (refresca si el upstream esta disponible)
- hard TTL: 15m para resources, 5m para policies
- si el `control-plane` no responde y el cache esta dentro del hard TTL: usa cache
- si el cache expiro o no existe: fail closed (deny)
- cada entry de cache incluye version, fetched_at, expires_at

Tracking de degradacion per-request:

- al inicio de cada `Create()`, se inyecta un `DegradationCollector` en el `context.Context`
- si un caching resolver sirve de cache stale (upstream fallo), marca `resourceDegraded` o `policiesDegraded` en el collector del context
- al emitir audit de `action_created`, si el collector indica degradacion, se agrega `"degraded_context": true` al campo `Data` del audit record
- el collector es request-local (no hay estado compartido entre requests concurrentes)
- implementacion: `cache.go` (WithDegradationCollector, DegradationFromContext) + `usecases.go` (inyeccion y lectura)

Esto permite que el `data-plane` siga decidiendo si el `control-plane` tiene un downtime breve, y que el audit trail refleje fielmente cuando una decision se tomo con datos de cache.

## Observabilidad minima actual

La primera capa de observabilidad de pre-prod ya esta activa.

- los tres servicios emiten logs estructurados JSON
- `v2/pkgs/go-pkg/observability` centraliza:
  - logger JSON por servicio
  - middleware HTTP de access logs
  - middleware HTTP de metricas RED
  - endpoint `/metrics`
  - `X-Request-Id`
  - propagacion de request ID a requests salientes
- cada request entrante:
  - preserva `X-Request-Id` si viene del caller
  - genera uno nuevo si no viene
  - lo devuelve en el response header
- los clients HTTP internos propagan el mismo request ID hacia:
  - `data-plane -> control-plane`
  - `data-plane -> control-workers`
  - `control-workers -> control-plane`
- los access logs actuales incluyen como minimo:
  - `service`
  - `event=http_request_completed`
  - `request_id`
  - `method`
  - `path`
  - `route`
  - `status`
  - `duration_ms`
  - `remote_addr`
- los fallos best effort de `audit` e `incidents` ya se loguean de forma estructurada con el logger del request
- `/metrics` queda bajo auth igual que el resto de endpoints de negocio
- las metricas RED actuales expuestas son:
  - `nexus_http_requests_total`
  - `nexus_http_request_errors_total`
  - `nexus_http_request_duration_seconds`
- las metricas minimas de negocio actuales son:
  - `nexus_actions_total`
  - `nexus_incidents_created_total`
  - `nexus_alerts_created_total`
- los smoke scripts actuales ya verifican `/metrics` autenticado en:
  - `data-plane`
  - `control-plane`
  - `control-workers`
- el stack local de observabilidad de pre-prod vive en:
  - `v2/ops/observability/prometheus`
  - `v2/ops/observability/grafana`
- `docker compose` ya levanta:
  - `nexus-prometheus`
  - `nexus-grafana`
  - exporters PostgreSQL por servicio y por `audit`
- Prometheus scrapea:
  - `data-plane`
  - `control-plane`
  - `control-workers`
  - `postgres-data-plane`
  - `postgres-control-plane`
  - `postgres-control-workers`
  - `postgres-audit`
- los scrapes autenticados usan una API key dedicada:
  - `NEXUS_PROMETHEUS_API_KEY`
- las reglas minimas de alerta cargadas hoy son:
  - `NexusServiceDown`
  - `NexusHighErrorRate`
  - `NexusHighLatencyP95`
  - `NexusDatabaseDown`
- Grafana provisiona automaticamente el dashboard:
  - `Nexus Pre-Prod Overview`
- `make smoke-observability` valida localmente:
  - readiness de Prometheus
  - readiness de Grafana
  - targets healthy
  - reglas cargadas
  - dashboard provisionado

## Auth minima actual

La auth minima del MVP ya esta activa.

- todos los endpoints de negocio requieren API key
- `/healthz` y `/readyz` quedan libres
- `/metrics` sigue protegido por API key
- se acepta `X-API-Key`
- tambien se acepta `Authorization: Bearer <key>`
- cada servicio valida auth inbound con `NEXUS_API_KEYS`
- `data-plane` autentica `control-plane` y `control-workers` con:
  - `NEXUS_CONTROL_PLANE_API_KEY`
  - `NEXUS_CONTROL_WORKERS_API_KEY`
- `control-workers` autentica `control-plane` con:
  - `NEXUS_CONTROL_PLANE_API_KEY`
- en AWS, la fuente de verdad prevista para esas API keys es Secrets Manager via `v2/infra`
- los errores de auth devuelven siempre:
  - `401 UNAUTHORIZED`
  - `valid api key required`
  sin detallar si faltaba header, si el formato era invalido o si la key no coincide

### Readiness, shutdown y headers HTTP

- `/healthz` es liveness simple
- `/readyz` ejecuta readiness real sobre dependencias locales:
  - `data-plane`: ping de PostgreSQL cuando corre con DB real
  - `control-plane`: ping de PostgreSQL principal y de audit cuando corren con DB real
  - `control-workers`: ping de PostgreSQL cuando corre con DB real
- `docker compose` y el ALB de `v2/infra` apuntan a `/readyz`, no a `/healthz`
- los tres servicios arrancan con graceful shutdown sobre `SIGTERM` y `SIGINT`
- el timeout actual de shutdown es `15s`
- headers de seguridad base activos en todas las respuestas:
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Referrer-Policy: no-referrer`
  - `Permissions-Policy: camera=(), microphone=(), geolocation=()`
- HSTS se activa solo cuando:
  - `NEXUS_HSTS_MAX_AGE` esta configurado
  - y el request llega como HTTPS o `X-Forwarded-Proto=https`
- CORS queda denegado por defecto
- si se necesita exponer algun endpoint via browser, se habilita con:
  - `NEXUS_CORS_ALLOWED_ORIGINS`
  - `NEXUS_CORS_ALLOW_CREDENTIALS`

## Persistencia principal actual

Las superficies principales ya pueden correr en memoria o con PostgreSQL.

- `control-plane/resources` usa `resources.InMemoryRepository` o `resources.PostgresRepository`
- `control-plane/policies` usa `policies.InMemoryRepository` o `policies.PostgresRepository`
- `control-plane/audit` usa `audit.InMemoryRepository` o `audit.PostgresRepository`
- `data-plane/actions` usa `action.InMemoryRepository` o `action.PostgresRepository`
- `control-workers/incidents` usa `incidents.InMemoryRepository` o `incidents.PostgresRepository`
- `control-workers/alerts` usa `alerts.InMemoryRepository` o `alerts.PostgresRepository`
- en `docker compose`, esas seis superficies ya corren con PostgreSQL real y sobreviven restart

### Configuracion actual de pool y timeouts

- `v2/pkgs/go-pkg/postgres` centraliza:
  - parseo de config por base de datos
  - `pgxpool`
  - `application_name`
  - `statement_timeout`
  - migrations `up-only`
- cada base de datos usa un prefijo de env propio:
  - `NEXUS_DATA_PLANE_DB_*`
  - `NEXUS_CONTROL_PLANE_DB_*`
  - `NEXUS_CONTROL_WORKERS_DB_*`
  - `NEXUS_AUDIT_DB_*`
- parametros configurables actuales:
  - `MIN_CONNS`
  - `MAX_CONNS`
  - `MAX_CONN_LIFETIME`
  - `MAX_CONN_IDLE_TIME`
  - `HEALTH_CHECK_PERIOD`
  - `CONNECT_TIMEOUT`
  - `STATEMENT_TIMEOUT`
- sizing inicial actual de pre-prod:
  - `data-plane`: `min=1`, `max=8`
  - `control-plane`: `min=1`, `max=8`
  - `control-workers`: `min=1`, `max=8`
  - `audit`: `min=1`, `max=4`
- timeouts actuales por defecto:
  - `connect_timeout=5s`
  - `statement_timeout=5s`

### Scripts operativos actuales

- `scripts/ops/postgres-backup.sh`
  - genera dump SQL con `--create --clean --if-exists`
- `scripts/ops/postgres-restore.sh`
  - restaura un dump SQL y reinicia el servicio que usa esa base
- `make smoke-persistence`
  - valida restart de servicios con estado persistido
- `make smoke-db-restore`
  - valida backup + restore manual de `control-plane`

## Baseline AWS actual

`v2/infra` ya define un baseline Terraform para AWS alineado al shape actual del sistema.

Incluye:

- VPC con subnets publicas y privadas
- NAT gateway opcional
- ECR para:
  - `data-plane`
  - `control-plane`
  - `control-workers`
- RDS PostgreSQL compartido
- Secrets Manager para runtime credentials
- API keys generadas por defecto en Secrets Manager
- ALB publico para:
  - `data-plane`
  - `control-plane`
- ECS Fargate para los tres servicios
- Cloud Map private DNS para trafico inter-servicio

Queda explicitamente fuera de este primer corte:

- `ai-runtime`
- CDN
- Redis/cache
- Route53
- WAF
- multi-region

Notas operativas:

- `v2/infra` ya no requiere poner API keys en `*.tfvars` por defecto
- los secretos de staging/prod se resuelven desde Secrets Manager
- `docker compose` sigue siendo solamente dev/local

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
- `incident_id`
- `alert_id`
- `resource_id`
- `resource_type`
- `actor`
- `summary`
- `data`
- `occurred_at`

`GET /v1/audit` soporta:

- `action_id`
- `incident_id`
- `alert_id`
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

Correlacion actual:

- `incident_created` persiste `action_id` e `incident_id`
- `alert_created` persiste `action_id`, `incident_id`, `alert_id`, `resource_id` y `resource_type`
- `GET /v1/audit` ya puede filtrar por `action_id`, `incident_id` y `alert_id`

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
- cuando `source_kind=action`, la respuesta expone `action_id` explicitamente
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
- la respuesta expone `incident_id`, `action_id`, `resource_id` y `resource_type`
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
