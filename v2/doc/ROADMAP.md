# Nexus v2 Product Roadmap

Relacionado:

- [DEFINITION.md](DEFINITION.md)
- [MVP.md](MVP.md)
- [PRE_PROD.md](PRE_PROD.md)
- [PROD_CHECKLIST.md](PROD_CHECKLIST.md)
- [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md)
- [REVIEW.md](REVIEW.md)

## Contexto

Este documento organiza los features post-MVP en fases incrementales.

Cada fase:

- se construye sobre la anterior
- es deployable y vendible por separado
- tiene un diferenciador funcional claro

El MVP ya esta cerrado (ver [MVP.md](MVP.md)).
Fase 0 (hardening) debe cerrarse antes de abrir Fase 1.
No se mezcla hardening con desarrollo de features nuevas.

Los disenos de Fase 1A y 1B fueron revisados y cerrados en [REVIEW.md](REVIEW.md) via colaboracion entre dos agentes de IA (Claude y GPT).

## Posicionamiento

Nexus no compite con Fireblocks feature por feature.

Fireblocks es custodia completa con policy engine incluido.
Nexus es policy engine standalone para cualquier custodia.

El diferenciador funcional central es:

> Fireblocks evalua transacciones. Nexus evalua patrones.

El policy engine de Fireblocks esta disenado como parte de su stack de custodia integrado. Su fortaleza es la evaluacion de transacciones individuales contra reglas estaticas en ese contexto.

Nexus toma un approach diferente: evalua cada accion en contexto de las acciones anteriores, con ventanas temporales, correlacion entre entidades y respuesta adaptativa. Al ser standalone, puede permitirse un modelo de evaluacion stateful que no depende de un stack de custodia especifico.

## Principio de diseno

La arquitectura de features avanzados se inspira en sistemas biologicos que resuelven problemas analogos a los de Nexus.

| Patron biologico | Problema que resuelve | Equivalente en Nexus |
|---|---|---|
| Cascada de coagulacion | Respuesta graduada multi-factor | Risk scoring con factores, anti-factores y amplificacion no-lineal |
| Inflamacion local | Respuesta proporcional localizada | Sensibilidad adaptativa por recurso ante incidentes |
| Fiebre sistemica | Escalada global ante amenaza coordinada | Lockdown progresivo por scope |
| Red de micorrizas | Propagacion de senales entre entidades | Grafo de recursos con propagacion de amenazas |
| Sistema inmune adaptativo | Aprendizaje de amenazas confirmadas | Anticuerpos: policies auto-generadas de incidentes |
| Murmuracion de estorninos | Deteccion colectiva sin coordinador | Senalizacion entre instancias de data-plane |

La metafora biologica es un framework interno de pensamiento. En Fases 1-2, el pitch comercial usa lenguaje operativo: "risk scoring multi-factor", "policies temporales", "4-eyes", "circuit breakers". La terminologia biologica como diferenciador tecnico ("Nexus Immune System") se introduce a partir de Fase 3 cuando el producto ya tenga traccion.

---

## Fase 0: Hardening (cerrada en entorno local)

Estado: cerrada en entorno local. 4 deploy blockers pendientes (secrets, TLS, e2e contra deploy, compose vs target).
Prerequisito: MVP cerrado.
Entregable: sistema listo para produccion.

Esta fase no agrega dominio nuevo.

Se ejecuta segun:

- [PRE_PROD.md](PRE_PROD.md)
- [PROD_CHECKLIST.md](PROD_CHECKLIST.md)

Exit criteria: [PROD_CHECKLIST.md](PROD_CHECKLIST.md) completo y firmado.

---

## Fase 1A: Risk scoring + canaries

Estado: implementada en runtime actual.
Prerequisito: Fase 0 cerrada.
Entregable: risk scoring multi-factor con respuesta graduada y deteccion de reconocimiento.

Nota de implementacion actual:

- `v2` ya corre `risk_pressure` / `safety_pressure`, hysteresis, baselines, known destinations, trap policies y `canary_triggered`
- `RiskProfile` todavia vive como preset builtin `balanced/v1`; la administracion versionada desde `control-plane` sigue pendiente como refinamiento

### 1A.1 Risk scoring multi-factor (cascada)

Funcion principal:

```
evaluate_risk(action, resource, actor, baselines, recent_actions) -> RiskResult
```

El resultado separa dos presiones:

- `risk_pressure`: sum de factores pro-riesgo activos + amplificaciones
- `safety_pressure`: sum de factores anti-riesgo activos + atenuaciones
- `raw_score`: risk_pressure - safety_pressure (puede ser negativo internamente)
- `decision_score`: clamp(0.0, 1.0, raw_score * sensitivity_modifier)

Cada factor reporta `evidence_quality`: `observed | inferred | missing | stale`.

Pro-factores:

| Factor | Evaluacion | Peso default (balanced) |
|---|---|---|
| `amount_anomaly` | monto > baseline.avg + (3/confidence) * baseline.stddev | 0.15 (0.05 en cold start, excepto criticality critical) |
| `velocity_spike` | acciones en 30m > baseline.p95 del actor | 0.20 |
| `new_destination` | destino no visto o con decay bajo | 0.15 |
| `off_hours` | hora fuera de typical_hours del actor | 0.10 |
| `actor_deviation` | comportamiento atipico (composite) | 0.20 |
| `recent_incident` | incidentes abiertos en el resource group | 0.10 |

Anti-factores:

| Factor | Evaluacion | Peso default (balanced) |
|---|---|---|
| `known_destination` | destino frecuente con decay alto | 0.20 |
| `within_baseline` | todos los pro-factores inactivos | 0.15 |
| `business_hours` | hora dentro de typical_hours | 0.10 |
| `verified_actor` | 2FA reciente o IP conocida | 0.15 |

Amplificaciones (solo sobre risk_pressure, cap global x3.0):

| Combinacion | Multiplicador |
|---|---|
| `amount_anomaly` + `velocity_spike` | x1.5 |
| `new_destination` + `actor_deviation` | x2.0 |
| `amount_anomaly` + `new_destination` + `off_hours` | x2.5 |

Atenuaciones (solo sobre safety_pressure):

| Combinacion | Multiplicador |
|---|---|
| `known_destination` + `verified_actor` | x1.5 |
| `within_baseline` + `business_hours` | x1.3 |

Bandas de decision con hysteresis de ±0.03 en bordes:

```
conservative: [0.15, 0.30, 0.50, 0.70]
balanced:     [0.20, 0.40, 0.60, 0.80]
aggressive:   [0.30, 0.50, 0.70, 0.90]
```

Mapeo: allow / enhanced_log / additional_auth / require_approval / deny.

RiskProfile: entidad versionada e inmutable en control-plane. Presets conservative/balanced/aggressive + custom acotado. Override solo de: factor enabled/disabled, threshold bands, multiplicadores dentro de rangos. No se exponen pesos raw.

Codigo: `data-plane/internal/action/risk/` con cascade.go, factors.go, amplification.go, profile.go, baselines.go, result.go.

### 1A.2 Behavioral baselines

Estadisticas simples calculadas del historial de acciones. No ML.

Modelo:

```
Baseline {
  scope_type:  "resource" | "actor"
  scope_id:    string
  metric:      string
  avg:         float64
  stddev:      float64
  p95:         float64
  sample_size: int
  window_days: int
  computed_at: time
}
```

Confidence por metrica (no por scope completo) con curva saturante:

```
confidence = 1 - exp(-sample_size / 10.0)
// 3 dias -> 0.26, 7 dias -> 0.50, 14 dias -> 0.75, 30 dias -> 0.95
```

Known destinations con decay exponencial:

```
destination_confidence = exp(-days_since_last_seen / 30.0)
```

Destinos internos (source y destination ambos registrados en Nexus) pesan menos que destinos externos.

Metricas por recurso en 1A: daily_tx_count, daily_volume, avg_tx_amount, unique_destinations_daily.
Metricas por actor en 1A: daily_action_count, typical_hours (peso bajo, nunca factor dominante solo).
Metricas avanzadas de actor: 1B+.

Cold start: new_destination activo con peso completo. amount_anomaly con peso reducido (0.05) excepto criticality critical. Score base cold start ~0.20 (enhanced_log), no 0.30.

Job de computo cada hora en data-plane. Baselines en tabla PostgreSQL propia del data-plane.

```sql
CREATE TABLE baselines (
  scope_type   TEXT NOT NULL,
  scope_id     TEXT NOT NULL,
  metric       TEXT NOT NULL,
  avg          DOUBLE PRECISION NOT NULL,
  stddev       DOUBLE PRECISION NOT NULL,
  p95          DOUBLE PRECISION NOT NULL,
  sample_size  INT NOT NULL,
  window_days  INT NOT NULL,
  computed_at  TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (scope_type, scope_id, metric)
);

CREATE TABLE known_destinations (
  resource_id  TEXT NOT NULL,
  destination  TEXT NOT NULL,
  first_seen   TIMESTAMPTZ NOT NULL,
  last_seen    TIMESTAMPTZ NOT NULL,
  tx_count     INT NOT NULL DEFAULT 1,
  PRIMARY KEY (resource_id, destination)
);
```

### 1A.3 Canary resources

Un canary es un recurso que no deberia recibir acciones en operacion normal.

`is_canary` vive SOLO en control-plane. No viaja al data-plane. Cuando se marca un recurso como canary, control-plane auto-genera una trap policy:

```
resource.labels._nexus_trap == true => deny, is_trap=true
```

Una sola policy cubre todos los canaries via label interno. No revela IDs. El data-plane no sabe que hay canaries — solo ve policies normales con `is_trap=true`.

Cuando una trap policy matchea:
- accion bloqueada (deny)
- incidente critical con trigger `canary_triggered`
- alert en pagerduty

Canary policies: policies cuya expresion describe situaciones imposibles en operacion legitima. Se marcan con `is_trap=true`. Ejemplo: `actor.role == "backup-service" && action.type == "withdrawal"`.

Cambios en codigo:
- control-plane/resources: campo `is_canary` + label `_nexus_trap`
- control-plane/policies: campo `is_trap`
- data-plane/action: cuando policy con `is_trap=true` matchea, forzar incidente critical
- control-workers/incidents: trigger type `canary_triggered`

### Exit criteria Fase 1A

- risk scoring multi-factor con risk_pressure/safety_pressure separados
- baselines calculandose con confidence saturante por metrica
- canary resources detectando acciones via trap policies
- cada decision explicable con desglose de factores y evidence_quality
- hysteresis funcional en bordes de bandas
- tests de cascada con combinaciones de factores y amplificaciones
- smoke que demuestre canary trigger + incidente critical

---

## Fase 1B: Stateful runtime controls

Estado: diseno cerrado.
Prerequisito: Fase 1A completa.
Entregable: controles temporales, approvals multi-step y agrupacion de recursos.

No-objetivos explicitos de 1B:
- no hacer CEP general-purpose
- no hacer BPM/workflow engine
- no hacer grafo de propagacion todavia

### 1B.1 Bucketed counters (proyeccion incremental durable)

Tabla de aggregates actualizada en write path (sincronico, no async):

```sql
CREATE TABLE action_aggregates (
  scope_type    TEXT NOT NULL,
  scope_id      TEXT NOT NULL,
  bucket_size   TEXT NOT NULL,
  bucket_start  TIMESTAMPTZ NOT NULL,
  action_type   TEXT,
  count         INT NOT NULL DEFAULT 0,
  sum_amount    DOUBLE PRECISION NOT NULL DEFAULT 0,
  max_amount    DOUBLE PRECISION NOT NULL DEFAULT 0,
  distinct_destinations INT NOT NULL DEFAULT 0,
  denied_count  INT NOT NULL DEFAULT 0,
  PRIMARY KEY (scope_type, scope_id, bucket_size, bucket_start, action_type)
);

CREATE INDEX idx_agg_scope_time ON action_aggregates (scope_type, scope_id, bucket_size, bucket_start DESC);
```

Tres granularidades: 1m (retener 2h), 5m (retener 12h), 1h (retener 7d).
3 UPSERTs atomicos por accion en la misma transaccion del write.
Purge periodico por job.

Sobreestimacion en bordes de bucket: aceptable para seguridad, etiquetada como `bucketed_window_estimate` en audit.

### 1B.2 Window CEL functions

4 funciones aproximadas y conservadoras (sobreestiman, nunca subestiman):

```
window.count(scope_type, scope_id, duration) -> int
window.sum(scope_type, scope_id, duration) -> float
window.denied(scope_type, scope_id, duration) -> int
window.max(scope_type, scope_id, duration) -> float
```

Traduccion automatica de duracion a granularidad: "30m" -> buckets 1m, "2h" -> buckets 5m, "7d" -> buckets 1h.

Ejemplos de policies temporales:

```
// Bloquear si withdrawals del wallet superan $500K en 2h
window.sum("resource", resource.id, "2h") + action.amount > 500000

// Requerir aprobacion si mas de 10 acciones del actor en 30m
window.count("actor", actor.id, "30m") > 10

// Bloquear si mas de 3 denies del actor en 1h
window.denied("actor", actor.id, "1h") > 3

// Limitar volumen total del grupo a $2M/dia
window.sum("resource_group", resource.group_id, "24h") + action.amount > 2000000
```

### 1B.3 Multi-step approvals

Entidad ApprovalPolicy en control-plane:

```
ApprovalPolicy {
  id, name
  mode:              "single" | "dual" | "quorum"
  required_count:    int
  quorum_pool_size:  int       // solo quorum: M en "N de M"
  sod_enabled:       bool      // proposer no puede aprobar
  escalation_to_policy_id: *string  // una sola transicion, no cadena
  escalation_after:  duration
  auto_reject_after: duration
}
```

Restricciones de 1B:
- una sola escalacion posible (profundidad maxima = 1)
- cadenas y arboles de escalacion son Fase 2+
- rechazo = final (soft_rejection es extension futura documentada)
- `require_approval bool` en policies queda como compat tecnica; modelo real es `approval_policy_id`
- si no hay policy explicita, se usa `default_single` built-in

Integridad en approval flow:
- idempotencia: mismo actor + mismo action_id + misma decision = no-op
- optimistic locking: campo `version` en Action, UPDATE con WHERE version = expected
- approval policy snapshot inmutable en Action al entrar en pending_approval:

```
Action {
  ...
  approval_snapshot: {
    policy_id, policy_version, mode, required_count,
    sod_enabled, escalation, captured_at
  }
}
```

La accion se evalua siempre contra el snapshot, nunca contra la policy actual.

Segregation of duties:

```
if policy.sod_enabled && action.proposer == approver -> reject
if approver already in action.approvals -> reject
```

### 1B.4 Resource groups

Entidad simple en control-plane:

```sql
CREATE TABLE resource_groups (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  description TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE resources ADD COLUMN group_id TEXT REFERENCES resource_groups(id);
```

Un recurso pertenece a 0 o 1 grupo. Membership multiple es Fase 3+ (control_scopes).

Internamente, resource_group implementa `ControlScope`:

```go
type ControlScope interface {
  ScopeType() string  // "resource_group"
  ScopeID() string
}
```

La cascada, window counters y breakers futuros operan sobre `ControlScope`, no sobre `ResourceGroup` directamente. En 1B solo existe un tipo concreto.

Disciplina semantica: resource groups son solo agregacion operativa. No representan signer graph, ownership, lineage, ni trust topology. Eso es Fase 3+.

Integrado con bucketed counters: scope_type = "resource_group". Cuando se crea una accion sobre un recurso con grupo, se actualizan buckets del recurso Y del grupo.

### Exit criteria Fase 1B

- bucketed counters actualizandose en write path
- al menos 4 funciones CEL de ventana temporal disponibles
- multi-step approvals funcional con dual y quorum
- segregation of duties funcional
- escalacion single-hop funcional
- resource groups con counters de grupo
- tests de window rules bloqueando patron de drain lento
- tests de approval con SoD, optimistic locking e idempotencia
- smoke end to end: accion -> window rule -> approval multi-step -> lease

---

## Fase 1C: Operator analysis

Estado: diseno cerrado.
Prerequisito: Fase 1B completa (o en paralelo si hay bandwidth).
Entregable: herramientas de analisis y calibracion para operadores.

1C no cambia el path critico. Cambia la capacidad de operar y calibrar.
1B se puede deployar y vender sin 1C.

No-objetivos explicitos de 1C:
- no modifica estado, nunca afecta acciones reales
- no corre en el path critico
- no reimplementa la cascada ni window rules (usa el mismo evaluation engine de 1A/1B)

### Arquitectura: un solo evaluation engine

El evaluator core de la cascada (1A) usa un `ContextProvider` interface:

```go
type ContextProvider interface {
  GetResource(id string) (*Resource, error)
  GetBaseline(scopeType, scopeId, metric string) (*Baseline, error)
  GetWindowCount(scopeType, scopeId string, duration time.Duration) (int, error)
  GetWindowSum(scopeType, scopeId string, duration time.Duration) (float64, error)
}
```

- Runtime (1A/1B): `LiveContextProvider` — lee de DB live, baselines actuales, buckets vivos.
- Simulation (1C): `HistoricalContextProvider` — lee de raw history, baselines snapshot, proyeccion temporal en memoria del job.

Mismo evaluator. Distinto context provider. Cero divergencia entre runtime y simulation.

### Data access

1C vive en control-plane pero necesita datos del data-plane. Control-plane lee data-plane via 3 endpoints internos gruesos (read-only, service key):

- `GET /internal/actions/history`: export paginado de acciones raw con filtros
- `GET /internal/baselines/snapshot`: baselines al dia mas cercano a un timestamp
- `GET /internal/incidents/{id}/actions`: set de acciones asociadas a un incidente

Simulation/backtest usan acciones raw, no buckets retenidos. Los buckets son para enforcement runtime. 1C reconstruye su propia proyeccion temporal read-only en memoria del job.

Baseline snapshots: el job de baselines (1A) guarda un snapshot diario compactado:

```sql
CREATE TABLE baseline_snapshots (
  scope_type    TEXT NOT NULL,
  scope_id      TEXT NOT NULL,
  metric        TEXT NOT NULL,
  snapshot_date DATE NOT NULL,
  avg           DOUBLE PRECISION NOT NULL,
  stddev        DOUBLE PRECISION NOT NULL,
  p95           DOUBLE PRECISION NOT NULL,
  sample_size   INT NOT NULL,
  PRIMARY KEY (scope_type, scope_id, metric, snapshot_date)
);
```

Retencion: 90 dias. Purge diario.

### Job model (async)

Modelo canonico async. Sync solo para jobs chicos (<500 acciones, <5s).

```
POST /v1/analysis/jobs
  body: { mode, params }
  -> { job_id, status: "pending" }

GET /v1/analysis/jobs/{id}
  -> { job_id, status, progress, result_summary }

GET /v1/analysis/jobs/{id}/result
  -> resultado completo paginado

DELETE /v1/analysis/jobs/{id}
  -> cancelar job en ejecucion
```

El job persiste una copia inmutable del input (mode, params, policy/profile versions, range temporal). Permite explicar y re-ejecutar.

Limites: 10,000 acciones evaluables por job. 5 jobs concurrentes por api key. Resultados cacheados con TTL 1 hora.

### 4 modos (un endpoint, un motor)

**simulation**: policy nueva contra historial raw. Responde: "si activo esta policy, cuantas acciones de los ultimos 30 dias hubiera bloqueado?"

**replay**: incidente con risk profile alternativo y baselines snapshot. Responde: "si hubieramos tenido el perfil conservative, lo habriamos detectado antes?"

**backtest**: policy existente con cambios contra historial. Responde: "si bajo el threshold de $500K a $300K, cuanto cambia el impacto?"

**compare**: dos risk profiles side-by-side contra el mismo set de acciones. Responde: "que diferencia hay entre conservative y balanced para nuestro patron de operaciones?"

### Clasificacion de fidelidad

Cada resultado declara su nivel de fidelidad:

- `snapshot_replay`: usa baseline snapshot del dia mas cercano + raw history
- `historical_recompute`: recalcula baselines desde raw history hasta timestamp T
- `approximate_replay`: usa baselines actuales (no se llama "replay")

No existe `exact_replay` como etiqueta — con snapshots diarios no hay exactitud temporal suficiente para ese claim.

### Donde vive en el codigo

```
control-plane/
  internal/
    analysis/
      handler.go           // endpoint /v1/analysis/jobs
      handler/dto/dto.go   // request/response DTOs
      usecases.go          // orquesta: crea job, fetch datos, evalua, cachea
      evaluator.go         // re-usa ContextProvider interface de 1A
      historical_context.go // HistoricalContextProvider
      data_client.go       // cliente HTTP read-only al data-plane /internal/*
      job.go               // job model, lifecycle, cache
      models.go            // SimulationResult, ReplayResult, etc.
      usecases_test.go
      evaluator_test.go

data-plane/
  internal/
    action/
      internal_handler.go  // endpoints /internal/actions/history, /internal/baselines/snapshot, /internal/incidents/{id}/actions
```

### Exit criteria Fase 1C

- analysis engine funcional con ContextProvider interface
- 4 modos funcionales: simulation, replay, backtest, compare
- fidelidad declarada en cada resultado
- job model async con polling y cancelacion
- input inmutable persistido en cada job
- tests del evaluator con HistoricalContextProvider
- smoke que demuestre simulation + replay end to end
- verificacion de que evaluator core es identico en runtime y simulation

---

## Fase 2: Response adaptation

Estado: pendiente diseno.
Prerequisito: Fase 1B completa.
Entregable: respuesta automatica proporcional a amenazas.

Secuencia interna: circuit breakers -> inflamacion -> fiebre -> lockdown.

### 2.1 Circuit breakers por recurso

Mecanico, local, inmediato. Primer paso antes de inflamacion.

- rate caps efimeros por recurso
- freeze temporal por recurso
- downgrade de lease TTL a segundos
- require-approval forzado por ventana corta

### 2.2 Inflamacion local

Solo si agrega valor sobre breakers. Go/no-go: si el cliente promedio tiene <5 recursos, circuit breakers alcanzan. Si tiene 10+, inflamacion agrega valor.

Requisitos para justificar inflamacion:
- decay temporal (no solo on/off)
- efecto sobre scopes relacionados (no solo el recurso afectado)
- impacto en lease TTL y approval mode

Sensitivity modifier:
- source of truth: control-plane
- read model caliente: data-plane con TTL corto
- consistencia eventual explicita

### 2.3 Fiebre sistemica

3+ recursos inflamados simultaneamente en el mismo scope = fiebre.
Todas las acciones requieren aprobacion. Lease TTL global al minimo.
Auto-resolucion cuando recursos inflamados bajan de 3. TTL maximo de 4h.

### 2.4 Lockdown

5+ recursos comprometidos confirmados o incidente critical confirmado por humano.
Todas las acciones bloqueadas. Solo quorum de admins puede desbloquear.

### Exit criteria Fase 2

- circuit breakers por recurso funcionales
- inflamacion con decay y propagacion (si aplica)
- fiebre activandose por scope
- lockdown activable manual y automaticamente
- tests de escalada breaker -> inflamacion -> fiebre -> lockdown

---

## Fase 3: Resource graph + dashboard

Estado: pendiente diseno.
Prerequisito: Fase 2 funcional.
Entregable: grafo de recursos, propagacion de senales e interfaz operativa.

### 3.1 Dashboard operativo minimo

Grafana extendido para ops. UI custom minima solo para approval workflows.
No construir plataforma visual prematura.

### 3.2 Grafo de recursos (micorrizas)

Recursos conectados en grafo. Propagacion de senales con decay.
Hubs (3+ conexiones) amplifican senales.
Control_scopes adicionales: signer_scope, operator_scope, fund_flow_scope.

### 3.3 Canaries avanzados

High-fidelity canaries con historial sintetico.
Rotating canaries dentro del grafo.

### Exit criteria Fase 3

- grafo de recursos funcional
- propagacion de senales con decay
- dashboard operativo minimo
- al menos un control_scope adicional

---

## Fase 4: Adaptive layer [post-PMF]

Estado: pendiente.
Prerequisito: Fases 1-3 funcionales + volumen real de incidentes.

Advertencia: fase mas peligrosa del roadmap. Review humano obligatorio. Explicabilidad total. Rollback inmediato. No se vende como near-term.

- anticuerpos: policies auto-generadas de incidentes confirmados, con TTL y boost
- behavioral fingerprinting de actores
- tolerancia: reduccion de sensibilidad para falsos positivos recurrentes

---

## Fase 5: Multi-instance signaling [post-PMF]

Estado: pendiente.
Prerequisito: multiples clientes en produccion con escalado horizontal.

- threat signaling entre instancias via pub/sub
- sensibilidad colectiva distribuida sin coordinador central
- consensus-free, cada instancia ajusta localmente

---

## Fase 6: Generalization + enterprise

Estado: pendiente.
Prerequisito: 5-10 clientes crypto pagando + Fases 1-2 en produccion.

- multi-tenancy (tenant = resource group raiz)
- action types y resource types dinamicos
- RBAC
- compliance hooks (pre_decision, post_approval, pre_execution)
- dashboard enterprise
- AI runtime (asiste, no decide)

---

## Resumen de fases

| Fase | Nombre | Diferenciador | Prerequisito | Diseno |
|---|---|---|---|---|
| 0 | Hardening | Sistema listo para produccion | MVP cerrado | N/A |
| 1A | Risk scoring + canaries | Risk scoring multi-factor con respuesta graduada | Fase 0 cerrada | Cerrado |
| 1B | Stateful runtime controls | Windows temporales, approvals multi-step, resource groups | Fase 1A | Cerrado |
| 1C | Operator analysis | Simulation, replay, backtesting | Fase 1B | Cerrado |
| 2 | Response adaptation | Breakers, inflamacion, fiebre, lockdown | Fase 1B | Pendiente |
| 3 | Graph + dashboard | Grafo de recursos, propagacion, interfaz operativa | Fase 2 | Pendiente |
| 4 | Adaptive layer | Anticuerpos, fingerprinting, tolerancia | Fases 1-3 + volumen | Pendiente |
| 5 | Multi-instance | Deteccion colectiva distribuida | Clientes en produccion | Pendiente |
| 6 | Generalization | Nuevas verticales y enterprise | 5-10 clientes crypto | Pendiente |

A marzo de 2026:

- Fase 0: cerrada en entorno local (4 deploy blockers pendientes)
- `1A` ya esta implementada en runtime en `v2` (RiskProfile CRUD pendiente para 1B)
- `1B` y `1C` siguen con diseno cerrado, no implementadas
- graceful degradation usa `DegradationCollector` per-request via context (auditado por GPT)

## Que se vende en cada fase

- Fase 0: nada, es infraestructura
- Fase 1A: "risk scoring inteligente con respuesta graduada y deteccion de reconocimiento"
- Fase 1B: "policies temporales, 4-eyes, quorum, segregation of duties"
- Fase 1C: "simulation, replay y backtesting de policies"
- Fase 2: "circuit breakers y respuesta automatica adaptativa"
- Fase 3: "dashboard operativo y deteccion de ataques coordinados"
- Fase 4: "aprendizaje de amenazas" (post product-market fit)
- Fase 5: "escalado horizontal con deteccion distribuida"
- Fase 6: "apertura a nuevas verticales"

## Restricciones de diseno

Estas restricciones fueron acordadas durante el review de diseno y deben mantenerse:

Fase 1A:
- hysteresis ±0.03 en bordes de bandas de decision
- trap policies sin filtrar el canary trivialmente (usar labels internos)
- confidence saturante por metrica, no lineal
- cold start conservador: amount_anomaly con peso reducido excepto criticality critical
- typical_hours con peso bajo, nunca factor dominante solo

Fase 1B:
- bucketed counters sincronicos en write path, no async
- windows conservadoras y aproximadas (sobreestiman, nunca subestiman)
- una sola escalacion posible, no cadenas
- optimistic locking en approval state
- snapshot inmutable de approval policy en Action
- resource_group con ambicion semantica limitada: solo agregacion operativa

Fase 1C:
- un solo evaluation engine con ContextProvider interface, cero divergencia runtime/simulation
- async como modelo canonico; sync solo para jobs chicos
- simulation/backtest usan raw history, no buckets retenidos
- replay usa baseline snapshots diarios, no baselines actuales
- fidelidad declarada: snapshot_replay | historical_recompute | approximate_replay (no existe exact_replay)
- cada job persiste copia inmutable del input

General:
- `control_scope` como interfaz interna desde 1B; resource_group es el primer tipo concreto
- fases 1-2 optimizan por claridad operativa, no por sofisticacion emergente
- la metafora biologica es framework interno, no pitch comercial en fases tempranas

## Lo que no cambia

Independientemente de la fase:

- CEL como lenguaje de policies
- lease como mecanismo de autorizacion efimera
- audit trail inmutable
- best effort en side-effects
- arquitectura de tres servicios (data-plane / control-plane / control-workers)
- determinismo en el path critico
- la IA nunca decide acciones criticas
