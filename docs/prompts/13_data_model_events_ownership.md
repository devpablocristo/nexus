# Prompt 13 — Modelo de datos, ownership y catálogo operacional de eventos

## Contexto del proyecto

Nexus tiene dos bases PostgreSQL, múltiples servicios y consumidores de eventos, pero hoy el ownership de datos y la semántica narrativa de los eventos están demasiado distribuidos entre código, migraciones y docs parciales.

**Prerequisito**: aplicar `docs/prompts/00_base_transversal.md`.

## Alcance obligatorio

Este prompt busca dejar explícito:
- qué entidad vive en qué servicio
- quién puede escribirla
- cómo cruza información entre bounded contexts
- qué eventos existen, quién los emite y quién los consume

---

## Lo que YA existe

- DB `nexus` para `nexus-core`
- DB `nexus_saas` para `nexus-saas`
- `pkgs/contracts/events.schema.json`
- `docs/SERVICE_BOUNDARIES.md`
- migraciones y modelos por servicio
- stream/consumo de eventos operativos por control operators y AI operators

---

## Qué implementar

### 1. Documento de ownership de datos

Crear `docs/data/DATA_MODEL_AND_OWNERSHIP.md` con:
- tablas principales por servicio
- owner de escritura
- readers autorizados
- claves externas lógicas entre servicios
- políticas de retención/cleanup cuando apliquen

### 2. Mapa de entidades canónicas

Incluir mínimo:
- orgs
- api keys / users / memberships
- tools / policies / egress / secrets
- approvals / audit records
- incidents / actions / alert rules / policy proposals
- notifications / billing / tenant settings / sessions

### 3. Catálogo narrativo de eventos

Crear `docs/events/EVENT_CATALOG.md` con:
- `event_type`
- producer
- consumers
- trigger exacto
- payload esperado
- idempotencia
- ordering/consistency assumptions
- acciones humanas o automáticas derivadas

### 4. Ownership de writes cross-service

Documentar reglas como:
- `nexus-core` escribe audit y enforcement runtime
- `nexus-saas` escribe business state
- `nexus-control-operators` y `nexus-ai-operators` escriben solo vía APIs, nunca directo en DB

### 5. Fixtures/ejemplos de eventos

Agregar ejemplos JSON mínimos de:
- `tool.call.completed`
- `tool.denied`
- `tool.rate_limited`
- `incident.opened`
- `action.applied`
- `proposal.created`

---

## Reglas de implementación

- No mezclar ownership físico con consumo lógico.
- Si una entidad se replica o deriva, indicar source of truth.
- Si un evento es append-only, decirlo explícitamente.
- Si hay eventual consistency, explicitar ventanas y efectos esperados.

---

## Archivos a crear o modificar

### Crear
- `docs/data/DATA_MODEL_AND_OWNERSHIP.md`
- `docs/events/EVENT_CATALOG.md`
- `docs/events/examples/*.json`

### Modificar
- `docs/SERVICE_BOUNDARIES.md`
- `docs/DOC.md`
- `pkgs/contracts/events.schema.json` si necesita mayor precisión

---

## Criterios de aceptación

- [ ] Existe un mapa claro de ownership por servicio/DB
- [ ] Se identifica source of truth por entidad principal
- [ ] Existe catálogo narrativo de eventos con producers y consumers
- [ ] Hay ejemplos JSON reutilizables para testing/docs
- [ ] Queda explícito que operators no escriben directo a DB

---

## Orden de ejecución recomendado

1. Ownership de datos por servicio
2. Mapa de entidades canónicas
3. Catálogo de eventos
4. Ejemplos JSON
5. Alineación con schemas y docs existentes
