# Nexus — diseño de piezas faltantes (Companion, Memory, Connectors, Workspace)

Documento de referencia para decisiones técnicas. Complementa la visión de producto (Nexus = compañero IA gobernado; Review = núcleo soberano).

---

## RFC-A — Companion runtime (más allá del vertical slice)

### A.1 Rol y límites

- **Companion** orquesta trabajo por **task**, propone acciones y persiste trazabilidad.
- **No** evalúa CEL, **no** es fuente de verdad de approvals, **no** sustituye a Review.
- Toda acción sensible hacia el mundo real pasa por **Review** (y approvals si aplica) antes de **Execute** en un connector.

### A.2 Entidades (evolución del modelo actual)

| Entidad | Uso |
|---------|-----|
| `Task` | Objetivo operativo; estado agregado. |
| `TaskMessage` | Hilo humano/sistema. |
| `TaskAction` | Unidad atómica de “algo que pasó” o “algo que se intentó”: `investigate`, `propose`, `sync_review`, `execute_connector`, `fail`, etc. |
| `TaskPlan` (nuevo, opcional fase 1b) | Lista ordenada de steps deseados; permite reanudar sin re-LLM. Puede ser JSON en `task.context_json` al inicio. |

### A.3 Máquina de estados (transiciones explícitas)

Estados ya persistidos en SQL: `new`, `investigating`, `proposing`, `waiting_for_input`, `waiting_for_approval`, `executing`, `verifying`, `done`, `failed`, `escalated`.

**Transiciones mínimas recomendadas (v1 “runtime”):**

| Desde | Evento / acción | Hacia | Notas |
|-------|-----------------|-------|--------|
| `new` | `investigate` | `investigating` | Idempotente si ya está. |
| `new`, `investigating` | `propose` (Review OK) | `waiting_for_approval` | Si Review devuelve `require_approval` o queda pendiente. |
| `new`, `investigating` | `propose` (Review allow) | `executing` o `verifying` | Si la política permite sin approval y hay siguiente paso automático; si no, `done` o `waiting_for_input`. |
| `waiting_for_approval` | `sync_review` (pull) | `executing` / `failed` / `waiting_for_input` | Según `request.status` + `decision` en Review. |
| `*` | `human_input` | `investigating` o `proposing` | Desbloqueo explícito. |
| `*` | `escalate` | `escalated` | Con `reason_code` + texto. |
| `executing` | `execute_ok` | `verifying` o `done` | |
| `executing` | `execute_err` | `failed` o `investigating` | Política de reintentos en `TaskAction`. |
| `verifying` | `verify_ok` | `done` | |
| `done`, `failed`, `escalated` | — | terminal | `closed_at` opcional. |

**Regla de oro:** ninguna transición a `executing` por acción sensible sin evidencia de Review (`review_request_id` + estado coherente en sync).

### A.4 Taxonomía de `TaskAction.action_type`

| Tipo | Payload mínimo | Efecto |
|------|------------------|--------|
| `investigate` | `note?` | Estado + mensaje sistema. |
| `propose` | `note?`, `target_system?`, … | Crea fila; llama `POST /v1/requests`; guarda `review_request_id` o `error_message`. |
| `sync_review` | `review_request_id` | Pull a Review; opcional actualización de task. |
| `execute_connector` | `connector_id`, `operation`, `payload` (acotado) | Solo si reglas de negocio lo permiten (ver RFC-C). |
| `human_override` | `user_id`, `reason` | Auditoría. |

### A.5 Sincronización con Review

**Fase conservadora (recomendada):**

- Worker o goroutine por instancia: `sync_stale_tasks` cada N segundos o **al abrir detalle** (ya hay polling UI; el servicio puede materializar “estado derivado”).
- `GET /v1/requests/{id}` con API key servidor (Companion → Review).
- Tabla opcional `companion_review_sync_state(task_id, review_request_id, last_status, last_checked_at)` para no martillar Review.

**Fase posterior:**

- Review expone `POST` interno de webhook (firma HMAC) al cambiar request/approval; Companion invalida cache y avanza task. Requiere diseño de seguridad y idempotencia.

### A.6 Idempotencia

- `Idempotency-Key` en Submit a Review ya soportado; mantener convención `companion-{action_id}` o `companion-task-{task_id}-{action_type}-{hash_payload}`.
- `TaskAction` único lógico para un mismo intento de propose (evitar duplicar requests por doble click).

---

## RFC-B — Memory (continuidad operativa v1)

### B.1 Principios

- Memoria **operativa**, no “todo el historial de chat”.
- **Opt-in** por tipo; retención y borrado acordes a compliance.
- Review **no** almacena memoria larga de Companion; puede referenciar IDs en `params.nexus`.

### B.2 Tipos de memoria (v1)

| `memory_kind` | Clave natural | Contenido |
|---------------|---------------|-----------|
| `task_summary` | `task_id` | Resumen ejecutivo actualizable (texto + versión). |
| `task_facts` | `task_id` | JSON estructurado (hechos confirmados, no especulación libre). |
| `playbook_snippet` | `org_id` + `slug` | Fragmento reusable (markdown corto). |
| `user_preference` | `user_id` + `key` | Preferencias operativas (idioma, canal, severidad). |

Excluido en v1: embeddings globales, memoria infinita por conversación sin task.

### B.3 Modelo de datos (propuesta)

Tablas dedicadas en DB **Companion** (misma instancia Postgres, DB `nexus_companion`) para empezar:

- `companion_memory_entries`  
  - `id`, `kind`, `scope_type` (`task`|`org`|`user`), `scope_id`, `key`, `payload_json`, `content_text`, `version`, `created_at`, `updated_at`, `expires_at` nullable.

Índices: `(scope_type, scope_id, kind)`, `(expires_at)` para purge.

### B.4 API (conceptual)

- `GET /v1/memory?scope_type=task&scope_id={uuid}&kind=task_summary`
- `PUT /v1/memory` (upsert con versión optimista opcional)
- `DELETE /v1/memory/{id}` (soft delete o hard según política)

Autorización: misma API key que Companion en v1; futuro: scopes por usuario.

### B.5 Escritores

| Actor | Puede escribir |
|-------|----------------|
| Companion (usecase) | `task_summary`, `task_facts` tras pasos explícitos. |
| Humano (Workspace) | `task_facts`, `playbook_snippet`, `user_preference`. |
| Batch | TTL / purge de `expires_at`. |

### B.6 Retención

- Defaults por `kind` (ej. `task_summary` 90 días post-`task.closed_at`).
- Job `memory_purge` diario.

---

## RFC-C — Connectors y ejecución post-Review

### C.1 Rol

- **Connectors** son la única capa con credenciales hacia sistemas externos.
- Companion **orquesta** “cuándo”; el connector **cómo** (protocolo, formato).

### C.2 Interfaz lógica (Go)

```text
Connector interface {
  ID() string
  Capabilities() []Capability        // ej. jira.create_issue, slack.post_message
  Validate(spec ExecutionSpec) error
  Execute(ctx, spec ExecutionSpec) (ExecutionResult, error)
}
```

`ExecutionSpec`: `connector_id`, `operation`, `payload` (map acotado), `idempotency_key`, `correlation` (`task_id`, `review_request_id`).

`ExecutionResult`: `external_ref`, `status`, `raw_log_ref` (opcional), `retryable`.

### C.3 Registro

- Tabla `companion_connectors`: `id`, `name`, `enabled`, `config_json` (sin secretos en claro; referencias a env/vault).
- v1: conectores **compilados en binario** (plugins estáticos) + fila en DB para habilitar/deshabilitar.

### C.4 Reglas de gating (obligatorias)

Antes de `Execute` para operaciones `side_effect = true`:

1. Debe existir `TaskAction` de tipo `propose` con `review_request_id` seteado.
2. Estado en Review para esa request: `allow` o flujo de approval **completado a favor** (definir mapping exacto desde `request.status` + approvals en Review).
3. Opcional: segunda verificación `GET /v1/requests/{id}` inmediatamente antes de ejecutar (anti race).

Operaciones `read_only` (ej. listar issues): pueden tener política distinta (solo Review si el org lo exige).

### C.5 Trazabilidad

- Nuevo `TaskAction` `execute_connector` con payload + `external_ref` en resultado.
- Logs estructurados: `task_id`, `review_request_id`, `connector_id`, `operation`.

### C.6 Despliegue evolutivo

| Fase | Forma |
|------|--------|
| C-v1 | Un solo binario Companion + 1 connector mock (log only). |
| C-v2 | Worker opcional con cola (Redis/Postgres `outbox`) si el execute es lento. |
| C-v3 | Conectores como procesos separados con mTLS. |

---

## Workspace (evolución de `v3/console`)

No requiere rename inmediato del repo; sí una **arquitectura de información**:

1. **Home / Bandeja** — tasks activas, bloqueadas, recientes.
2. **Gobernanza** — approvals + link a request + link a task Companion (`params.nexus.task_id` cuando exista).
3. **Operación** — requests, policies, action types, delegations (actual).
4. **Sandbox / herramientas** — simulación, replay (actual).
5. **Ajustes** — config, (futuro) conectores y memoria por org.

Navegación: agrupar pestañas por estas áreas reduce sensación de “backoffice suelto”.

---

## Orden de implementación sugerido

1. **Companion:** worker `sync_review` + transiciones de estado documentadas en código + tests.
2. **Workspace:** bandeja task-centric + deep link request ↔ task.
3. **Memory:** tablas + API mínima + UI solo lectura de `task_summary`.
4. **Connectors:** interfaz + mock + un connector real detrás de flag.

---

## Checklist de coherencia con la visión

- [ ] Review sigue sin chat/memoria de largo plazo.
- [ ] Companion no evalúa políticas.
- [ ] Ejecución externa pasa por Review + `TaskAction` trazable.
- [ ] Workspace acerca al humano al flujo task → review → resultado.
- [ ] Memory acotada y gobernada por retención.

---

*Última actualización: diseño inicial consolidado para iteración; ajustar con ADRs cuando se implemente cada bloque.*
