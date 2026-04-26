# Plan para completar Nexus (visión ecosistema)

Objetivo: llevar el repo desde el **estado actual** (`v3/nexus` + `console`) hasta cubrir las piezas de la visión. **Companion runtime**, **Memory** y **Connectors** ahora viven en un proyecto independiente; este roadmap se enfoca en el plano de governance del repo Nexus.

Principios: no reescribir Review; conservador; trazabilidad; cada fase con **criterios de salida** verificables.

Referencia de diseño: [NEXUS_ECOSYSTEM_DESIGN.md](./NEXUS_ECOSYSTEM_DESIGN.md).

---

## Estado actual (línea base)

- Review: governance core operativo.
- Companion: tasks, messages, actions, `propose` → Review, `review_request_id`, **FSM** (`core/backend/go/fsm` + reglas en Companion), estado de tarea derivado de la **respuesta** Review (allow/deny/pending_approval), **worker de sync** (`COMPANION_REVIEW_SYNC_INTERVAL_SEC`), **`POST /v1/tasks/{id}/sync`**, UI Tasks con botón sync.
- Console: múltiples vistas técnicas + Tasks aislada.
- Falta: Memory, Connectors, UX unificada (Workspace), auth/observabilidad/E2E en CI, `TaskAction sync_review` persistido (opcional Fase 1.3).

---

## Fase 0 — Cimientos y calidad (1–2 semanas)

**Objetivo:** que lo existente sea repetible y testeable.

| # | Entregable | Notas |
|---|------------|--------|
| 0.1 | Script smoke E2E: crear task → propose → assert request en Review + vínculo | `scripts/smoke/run-companion-review-flow.sh` + `make smoke` |
| 0.2 | Script `CREATE DATABASE nexus_companion` para volúmenes viejos | `scripts/dev/ensure-companion-db.sh` + `v3/README.md` |
| 0.3 | CI: job opcional que levante compose y ejecute smoke (o integración con testcontainers) | Puede ser nightly si es pesado |
| 0.4 | Correlación básica en logs: `task_id` / `review_request_id` en Companion | `slog` en create/propose (`internal/tasks/usecases.go`) |

**Salida:** smoke verde en local con `docker compose`; documentación de arranque unificada (`v3/README.md` o enlace a companion + review).

---

## Fase 1 — Companion runtime + sincronía Review (3–5 semanas)

**Objetivo:** la task **vive** según decisiones de Review, no solo al crear el request.

| # | Entregable | Notas |
|---|------------|--------|
| 1.1 | Máquina de estados explícita en código + tests | Tabla de transiciones en RFC-A; rechazar transiciones ilegales 409 |
| 1.2 | Tras `propose`, derivar estado task desde **respuesta** Review (allow / deny / require_approval / pending) | Ajustar flujo actual que fuerza `waiting_for_approval` |
| 1.3 | `TaskAction` tipo `sync_review` + persistencia de último snapshot | Opcional tabla `companion_review_sync` |
| 1.4 | Worker o ticker: tasks en `waiting_for_approval` hacen pull a Review cada N s + actualizan task | Backoff; métricas |
| 1.5 | API `POST /v1/tasks/{id}/sync` (manual) + uso opcional desde UI | |
| 1.6 | UI Tasks: mostrar estado derivado de Review (sin nuevo polling agresivo si el server ya sincroniza) | |

**Salida:** task pasa de “esperando” a “desbloqueada” o “fallida” según Review **sin** nueva propuesta; tests de usecase + smoke actualizado.

**Depende de:** Fase 0 (smoke).

### Progreso Fase 1 en repo

| # | Estado | Notas |
|---|--------|--------|
| 1.1 | Hecho | FSM en core + transiciones Companion + tests; conflicto HTTP `409` mensaje genérico |
| 1.2 | Hecho | `Propose` usa `review_submit.status` → `done` / `failed` / `waiting_for_approval` |
| 1.3 | Pendiente | Sin acción `sync_review` ni tabla `companion_review_sync` (opcional) |
| 1.4 | Parcial | Ticker + batch; sin backoff ni métricas dedicadas |
| 1.5 | Hecho | `POST /v1/tasks/{id}/sync` + uso en UI |
| 1.6 | Parcial | Estado en listado/detalle; polling UI 4s; server sync reduce necesidad de sync manual |

**Smoke:** `scripts/smoke/run-companion-review-flow.sh` valida vínculo + **estado coherente** con Review y llama a `/sync`.

---

## Fase 2 — Plan ligero y acciones de sistema (2–3 semanas)

**Objetivo:** ordenar el trabajo sin orquestador distribuido.

| # | Entregable | Notas |
|---|------------|--------|
| 2.1 | `TaskPlan` en `context_json` o tabla `companion_task_plans` | Steps: `propose`, `wait_review`, `human`, `execute` (stub) |
| 2.2 | Endpoint o usecase `advance` que marque step completado y dispare el siguiente permitido | |
| 2.3 | `TaskAction` para `human_input`, `escalate` con códigos | |

**Salida:** al menos un flujo demo: investigate → propose → sync → “siguiente step” mock execute (sin connector real).

**Depende de:** Fase 1.

---

## Fase 3 — Memory v1 (3–4 semanas)

**Objetivo:** continuidad operativa acotada.

| # | Entregable | Notas |
|---|------------|--------|
| 3.1 | Migración `companion_memory_entries` (o equivalente) | Ver RFC-B |
| 3.2 | API `GET/PUT /v1/memory` con scope `task` + kinds `task_summary`, `task_facts` | Auth: misma API key v1 |
| 3.3 | Usecases: Companion escribe summary tras hitos; humano puede PATCH | |
| 3.4 | Job `purge` por `expires_at` | |
| 3.5 | Workspace: panel en detalle de task (lectura/edición simple) | |

**Salida:** una task puede guardar y recuperar resumen y hechos; retención documentada.

**Depende de:** Fase 0; encaja en paralelo con Fase 2 si hay equipo.

---

## Fase 4 — Connectors v1 (4–6 semanas)

**Objetivo:** primera ejecución externa **gated** por Review.

| # | Entregable | Notas |
|---|------------|--------|
| 4.1 | Interfaz `Connector` en código + registro en DB | RFC-C |
| 4.2 | Connector **mock** (`log_only`) + `TaskAction` `execute_connector` | |
| 4.3 | Reglas de gating: no ejecutar sin `review_request_id` + estado Review permitido | Tests que fallen si se salta |
| 4.4 | Un connector real detrás de flag (ej. HTTP genérico firmado o Slack webhook read-only) | Elegir el de menor riesgo |
| 4.5 | Documentar contrato de secretos (env / futuro vault) | |

**Salida:** smoke: propose → approval si aplica → execute mock → `external_ref` en action.

**Depende de:** Fase 1 (sync); recomendable después de Fase 2 para “execute” como step.

---

## Fase 5 — Workspace (producto) (3–5 semanas)

**Objetivo:** una experiencia humana coherente, no solo más pestañas.

| # | Entregable | Notas |
|---|------------|--------|
| 5.1 | IA de navegación: agrupar vistas (Bandeja, Gobernanza, Operación, Herramientas, Ajustes) | Renombrar copy; repo puede seguir `console` |
| 5.2 | “Home” con tasks activas + pendientes de atención | Datos desde Companion |
| 5.3 | Desde Inbox/approval: link a task si `params.nexus.task_id` existe | Review sin cambios grandes: UI lee request |
| 5.4 | Desde task: link a request Review ya existente | |
| 5.5 | (Opcional) unificar API keys: BFF en Companion que proxifica solo lo necesario | Reduce secretos en browser |

**Salida:** un usuario puede seguir el hilo task ↔ review sin perderse.

**Depende de:** Fase 1 mínimo; mejora con Fase 3–4.

---

## Fase 6 — Plataforma producción (paralelo / continuo)

**Objetivo:** listo para entorno controlado, no solo dev.

| # | Entregable | Notas |
|---|------------|--------|
| 6.1 | Modelo de auth humano (OIDC o sesión server-side) | Sustituir o complementar API key en UI |
| 6.2 | OpenTelemetry o trazas entre `companion` ↔ `review` | Headers W3C trace |
| 6.3 | Runbooks: backup DBs, rotación keys, límites de rate | |
| 6.4 | (Opcional) Multi-tenant: `org_id` en tasks y memory | Solo si hay requisito claro |

**Salida:** checklist “go-live” en `doc/DEPLOYMENT.md` o anexo.

---

## Fase 7 — Evolución (cuando el negocio lo pida)

- Webhooks Review → Companion (push en lugar de solo pull).
- Outbox / cola para executes largos.
- Más connectors; embeddings solo sobre playbooks acotados.
- Multi-agente, etc. (fuera del plan base).

---

## Orden recomendado (una sola línea)

`0 → 1 → 2 → 4 → 5` con `3` en paralelo tras `0`; `6` en paralelo desde `1`.

## Estimación global

Orden de magnitud **4–8 meses** con 1–2 devs backend + 0.5–1 frontend, asumiendo context switching y sin contar Fase 7. Ajustar según alcance de connector real y auth.

## Criterio de “Nexus visión base completa”

- [x] Task con ciclo de vida gobernado por Review (sync incluida) — *slice Companion v1: FSM + pull + API sync; sin webhooks Review→Companion*.
- [ ] Memory v1 en uso para al menos resumen de task.
- [ ] Al menos un execute externo gated (aunque sea mock + uno real trivial).
- [ ] Workspace con home + enlaces task ↔ request.
- [ ] Smoke E2E + rumbo a auth no basado solo en key en el browser.

---

*Plan vivo: revisar al cerrar cada fase.*
