# RFC: Nexus — Runtime de gobierno para agentes financieros y nexo de coordinacion entre equipos

Estado: Borrador
Autor: Pablo Cristo
Fecha: 2026-03-16
Version: 2.0

## Resumen

Nexus es el nexo unico en dos sentidos:

**Con los clientes**: los agentes (bots, sistemas) envian requests a Nexus. El engine determinista los procesa, gobierna la autoridad delegada, y decide antes de que toquen infraestructura que mueve fondos.

**Con los equipos**: un solo agente IA de Nexus — el mismo para todos los equipos (treasury, ops, security, compliance) — notifica anomalias, contextualiza, y ofrece acciones rapidas en un unico chat (web y app movil). Los equipos no necesitan coordinarse entre si: se coordinan a traves de Nexus.

El engine no resuelve anomalias solo: notifica al agente IA, que a su vez notifica a los responsables con contexto y opciones. El humano decide; Nexus presenta.

Este RFC describe la arquitectura, mecanismos y justificacion de Nexus tal como esta implementado hoy (MVP + Fase 1A + saas-core) y las extensiones disenadas (Fase 1B, 1C, 1D).

## Problema

Las organizaciones con operaciones crypto automatizadas enfrentan un vacio de control:

1. **Los bots y agentes pueden mover fondos sin supervision humana suficiente.** Una credencial comprometida, un threshold mal configurado o un insider malicioso pueden disparar transacciones irreversibles.

2. **Las soluciones existentes acoplan custodia con politicas.** Plataformas como Fireblocks empaquetan custodia, firma y enforcement de politicas. Si ya tenes tu propia infraestructura de custodia, tenes que migrar todo para obtener controles de politicas.

3. **La evaluacion por transaccion es insuficiente.** La mayoria de los policy engines evaluan cada transaccion de forma aislada. El hack de Bybit ($1.4B, febrero 2025) demostro que los ataques sofisticados son patrones multi-paso — drains lentos, compromisos coordinados, anomalias de velocidad — invisibles para reglas por transaccion.

4. **Construir internamente es caro y nunca se completa.** Los equipos de ingenieria construyen controles ad-hoc que carecen de audit trails, flujos de aprobacion, scoring de riesgo y capacidades de simulacion.

## Principios de diseno

**Determinista en el path critico.** El engine que decide es reproducible dados los mismos inputs. Sin ML, sin modelos probabilisticos en la decision. Un regulador que pregunte "por que se bloqueo esto?" recibe una expresion de politica y un registro de evidencia. La IA contextualiza y sugiere — no decide.

**Doble nexo.** Nexus conecta dos mundos: agentes con infraestructura protegida, y equipos humanos entre si. Una sola superficie para gobernar y para coordinar.

**Composable, no monolitico.** Nexus es la capa de gobierno. Se conecta a cualquier custodia, firma, o ejecucion. Sin vendor lock-in.

**Fail closed.** Si Nexus no puede evaluar una accion (cache expirado, upstream inalcanzable), deniega. El costo de un bloqueo falso es un retraso operativo. El costo de un allow falso son fondos perdidos.

**Auditable y demostrable.** Cada decision produce un registro de auditoria inmutable. El pilar Prove permite demostrar ante terceros (auditores, reguladores) que todo se hizo correctamente.

## Arquitectura

```
Cliente (bot / agente / script)
        |
        v
   Data-Plane          cache (degradacion controlada)
   (decide)
        |
        +---> Control-Plane (recursos, politicas, perfiles de riesgo, auditoria)
        |
        +---> Control-Workers (incidentes, alertas)
```

Tres servicios Go, cada uno con su propia base de datos PostgreSQL:

**Data-Plane** — El motor de decision. Recibe solicitudes de accion, resuelve el recurso objetivo, obtiene las politicas aplicables, evalua la cascada de riesgo y produce una decision. Emite leases para acciones aprobadas. Stateless excepto por un cache en memoria de recursos y politicas para degradacion controlada.

**Control-Plane** — La superficie de configuracion y auditoria. Administra recursos, politicas, perfiles de riesgo y registros de auditoria. Provee APIs administrativas. No participa en el path de decision en runtime (el data-plane cachea sus datos).

**Control-Workers** — El motor de side-effects. Abre incidentes cuando las acciones son bloqueadas o fallan. Genera alertas segun la severidad del incidente. Ejecuta playbooks deterministas. Nunca afecta decisiones.

### Comunicacion entre servicios

- Data-plane llama a control-plane para resolucion de recursos y politicas (cacheado con soft/hard TTL)
- Data-plane llama a control-workers para creacion de incidentes (best-effort, fire-and-forget)
- Data-plane llama a control-plane para emision de auditoria (best-effort)
- Control-workers llama a control-plane para emision de auditoria (best-effort)
- Ningun servicio llama al data-plane

### Degradacion controlada

Si el control-plane no esta disponible:

- Data-plane sirve desde cache local (soft TTL 30s, hard TTL 15m para recursos, 5m para politicas)
- Cada entry de cache almacena `version`, `fetchedAt`, `expiresAt`
- Las decisiones tomadas con cache stale se marcan `degraded_context: true` en el registro de auditoria
- El tracking de degradacion es per-request via `DegradationCollector` en `context.Context` (sin estado compartido entre requests concurrentes)
- Si el cache expiro o esta vacio: fail closed (deny)

## Mecanismos core

### 1. Cascada de riesgo multi-factor

Inspirada en la cascada de coagulacion sanguinea: multiples factores deben alinearse para una respuesta fuerte, y anti-factores pueden atenuar la senal.

**Factores de riesgo** (incrementan riesgo):

| Factor | Peso | Disparador |
|--------|------|-----------|
| amount_anomaly | 0.15 | El monto se desvia de la baseline del recurso |
| velocity_spike | 0.20 | La frecuencia de acciones excede la baseline |
| new_destination | 0.15 | Destino nunca visto antes |
| off_hours | 0.10 | Fuera del horario tipico del actor |
| actor_deviation | 0.20 | El comportamiento del actor se desvia de su baseline |
| open_incidents | 0.10 | Incidentes activos sobre el recurso |

**Factores de seguridad** (reducen riesgo):

| Factor | Peso | Disparador |
|--------|------|-----------|
| known_destination | -0.20 | Destino usado exitosamente antes |
| within_baseline | -0.15 | Todas las metricas dentro de parametros normales |
| business_hours | -0.10 | Dentro del horario tipico del actor |

**Amplificacion no-lineal:** Cuando multiples factores de riesgo se activan simultaneamente, su efecto combinado se amplifica:

- amount_anomaly + velocity_spike: x1.5
- new_destination + actor_deviation: x2.0

La cascada produce dos scores:

- `risk_pressure`: suma de factores de riesgo activos con amplificacion
- `safety_pressure`: suma de factores de seguridad activos con atenuacion

Score final: `clamp(0, 1, risk_pressure + safety_pressure)` con histeresis (mezclado con el score anterior para prevenir oscilaciones).

**Respuesta graduada:**

| Rango de score | Decision | Analogia |
|----------------|----------|----------|
| 0.0 - 0.2 | allow | Tejido sano |
| 0.2 - 0.4 | enhanced_log | Moreton menor |
| 0.4 - 0.6 | additional_auth | Coagulo localizado |
| 0.6 - 0.8 | require_approval | Herida seria |
| 0.8 - 1.0 | deny | Hemorragia |

Cada decision incluye descomposicion completa de factores en el registro de evidencia.

### 2. Baselines estadisticas

Cada recurso y actor acumula baselines de comportamiento computadas cada hora:

- `daily_action_count`: promedio, desviacion estandar
- `avg_amount`: promedio, desviacion estandar
- `typical_hours`: horas de operacion mas frecuentes

Las baselines usan confidence saturante: `confidence = 1 - e^(-count / scale)`. Un recurso con 5 observaciones tiene confidence baja; uno con 100 tiene confidence casi completa. Los pesos de los factores se escalan por confidence — baselines con baja confidence contribuyen menos al score de riesgo.

**Cold start:** Recursos nuevos sin historial arrancan con friccion elevada (thresholds conservadores). La friccion disminuye naturalmente a medida que la baseline madura.

### 3. Recursos canary (deteccion por trampa)

Los recursos canary son señuelos que nunca deberian recibir acciones reales. Si cualquier accion apunta a un canary, dispara un incidente critico inmediato.

Implementacion:

- Control-plane auto-asigna el label interno `_nexus_trap: true` cuando un recurso se marca como canary
- Una politica de trampa a nivel sistema matchea `resource.labels["_nexus_trap"] == "true"` con efecto `deny` e `is_trap: true`
- Data-plane evalua esta politica como cualquier otra — no hay logica especial de canary en el path de decision
- Si una trap policy matchea, el incidente usa trigger `canary_triggered` en lugar de `blocked_action`

Los canaries detectan reconocimiento y compromiso de credenciales con cero datos historicos, cero ML y cero ambiguedad de falsos positivos: si alguien toca un canary, es un ataque o una misconfiguracion critica.

### 4. Evaluacion de politicas (CEL)

Las politicas son expresiones en CEL (Common Expression Language de Google):

```cel
action.amount > 50000 && resource.criticality == "critical"
```

Cada politica tiene:

- `action_type` y `resource_type` (scope)
- `expression` (CEL)
- `effect` (allow | deny | require_approval)
- `priority` (orden de evaluacion)
- `is_trap` (flag de canary)
- `enabled` (toggle)

Las politicas se evaluan en orden de prioridad. La primera politica que matchea determina el efecto. Si ninguna politica matchea, la accion se permite (default-open en la capa de politicas; la cascada de riesgo puede escalar igualmente).

### 5. Ejecucion basada en lease

Una accion aprobada no otorga permiso permanente. Produce un **lease**: un token de autorizacion efimero con:

- Scope: ID de accion especifica, recurso y tipo de accion
- TTL: 2 minutos para recursos criticos, 5 minutos para high, 10 minutos para otros
- Uso unico: se consume al ejecutar

El cliente debe presentar un lease valido para ejecutar. Si el lease expira, la accion debe re-evaluarse. Esto previene ataques de replay y ejecucion fuera de contexto.

### 6. Idempotencia

`POST /v1/actions` acepta un header `Idempotency-Key`. Si la misma key se presenta dentro de 24 horas, se retorna la respuesta original sin crear una accion duplicada. La respuesta incluye `X-Idempotency-Replay: true`.

Las operaciones de cambio de estado (approve, reject, lease, execute) usan idempotencia semantica via la state machine de la accion — intentar una transicion invalida retorna un error, no un duplicado.

### 7. Audit trail

Cada decision, aprobacion, rechazo, lease y ejecucion produce un registro de auditoria en control-plane:

```json
{
  "event_type": "action_created",
  "source_service": "data-plane",
  "action_id": "...",
  "resource_id": "...",
  "actor": {"type": "bot", "id": "treasury-bot-1"},
  "summary": "action created",
  "data": {
    "action_type": "withdrawal",
    "decision": "require_approval",
    "risk_level": "high",
    "risk_score": 0.72,
    "degraded_context": false
  }
}
```

La emision de auditoria es best-effort: si el control-plane no esta disponible, la decision procede igualmente. Retencion: 90 dias en PostgreSQL hot storage.

## Extensiones disenadas (aun no implementadas)

### Fase 1B: Controles stateful de runtime

**Bucketed counters** — Agregados durables (granularidad 1m/5m/1h) actualizados sincronicamente en cada write de accion. Habilitan politicas de ventana temporal:

```cel
window.sum("resource", resource.id, "2h") + action.amount > 500000
```

Cuatro funciones CEL: `window.count`, `window.sum`, `window.denied`, `window.max`. Las aproximaciones son conservadoras (sobreestiman, nunca subestiman) y se etiquetan `bucketed_window_estimate` en auditoria.

**Aprobaciones multi-step** — Politicas de aprobacion que soportan:

- Control dual (4-eyes)
- Quorum (N de M aprobadores)
- Segregacion de funciones (quien propone no puede aprobar)
- Una escalacion (timeout dispara politica de fallback)
- Auto-rechazo por timeout
- Snapshot inmutable de la politica de aprobacion capturado al crear la accion
- Optimistic locking en el estado de aprobacion

**Resource groups** — Agrupacion logica de recursos para controles colectivos. Un recurso pertenece a cero o un grupo. Los grupos habilitan counters y politicas a nivel de grupo.

### Fase 1C: Analisis para operadores

Tooling read-only para operadores, corriendo contra datos historicos:

- **Simulacion de politicas**: "Si activo esta politica, cuantas de las acciones de los ultimos 30 dias hubiera bloqueado?"
- **Replay de incidentes**: "Con un perfil de riesgo conservador, hubieramos detectado este incidente 47 minutos antes?"
- **Backtesting de politicas**: "Si bajo este threshold de $500K a $300K, cuantas acciones mas se bloquean?"
- **Comparacion de perfiles**: "Cual es la diferencia operativa entre balanced y conservative para nuestras wallets de trading?"

Todo el analisis usa el mismo motor de evaluacion que produccion. Los resultados declaran su nivel de fidelidad (`exact_replay`, `historical_recompute`, `approximate_replay`).

## Diferenciacion

| Capacidad | Fireblocks | Build interno | Nexus |
|-----------|-----------|---------------|-------|
| Evaluacion | Por transaccion | Por transaccion | Por patron (ventanas temporales) |
| Risk scoring | Reglas estaticas | Ad-hoc | Cascada multi-factor con amplificacion |
| Deteccion de drain | No | Thresholds manuales | Sliding windows sobre bucketed counters |
| Deteccion por trampa | No | No | Recursos canary + trap policies |
| Aprobaciones | Basicas | Custom | 4-eyes, quorum, SoD, escalacion |
| Simulacion | No | No | Dry-run, replay, backtest, compare |
| Deployment | SaaS (infra del vendor) | Interno | Self-hosted (VPC del cliente) |
| Audit trail | Controlado por vendor | Incompleto | Inmutable, con descomposicion de factores, aware de degradacion |
| Vendor lock-in | Total (custodia + politicas) | Ninguno | Ninguno (solo capa de politicas) |
| Tiempo a valor | Semanas (migracion) | Meses (construir) | Dias (conectar a infra existente) |

## Deployment

Actual: Docker Compose con 3 servicios Go + 4 instancias PostgreSQL + Prometheus + Grafana.

Objetivo: AWS ECS con Terraform (scaffold de infraestructura existe en `v2/infra`).

El sistema esta disenado para deployment de instancia unica inicialmente. Soporte multi-instancia (Fase 5) agrega signaling entre instancias para deteccion colectiva de amenazas.

## Modelo de seguridad

- Todos los endpoints requieren autenticacion por API key excepto `/healthz` y `/readyz`
- Tres tipos de key: admin (acceso completo), service (inter-servicio), prometheus (solo metricas)
- Fail closed en fallo de auth
- Fail closed en expiracion de cache
- Sin secrets en codigo ni docker-compose (objetivo: AWS Secrets Manager)
- Audit trail es append-only
- Los lease tokens tienen scope, tiempo limitado y uso unico

## Limitaciones y no-objetivos

- Nexus no custodia fondos ni firma transacciones
- Nexus no ejecuta operaciones — las autoriza
- Nexus no usa ML ni modelos probabilisticos
- Nexus no provee UI (dashboards Grafana para monitoreo; UI es Fase 3+)
- Nexus no soporta multi-tenancy (Fase 6)
- La cascada de riesgo usa un perfil builtin `balanced/v1`; la administracion de perfiles esta pendiente (Fase 1B)

## Preguntas abiertas

1. **Hash-chain en audit trail**: Deberian los registros de auditoria incluir encadenamiento criptografico (cada registro hashea el anterior) como evidencia de no-alteracion? v1 lo tenia. v2 todavia no. Trade-off: prueba de integridad adicional vs. impacto en performance de escritura.

2. **Break-glass approval**: Deberia existir un override de emergencia que bypasee flujos de aprobacion con logging reforzado? Importante para aprobaciones multi-step (Fase 1B) donde todos los aprobadores pueden no estar disponibles.

3. **Estrategia open-source**: El core es arquitectonicamente adecuado para distribucion open-source con features enterprise como tier de pago. El timing y alcance del open-sourcing es una decision de negocio.

## Referencias

- [DEFINITION.md](DEFINITION.md) — Definicion de producto
- [MVP.md](MVP.md) — Alcance del MVP
- [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md) — Convenciones tecnicas y detalles de implementacion
- [ROADMAP.md](ROADMAP.md) — Roadmap por fases con disenos cerrados para Fase 1A, 1B, 1C
- [REVIEW.md](REVIEW.md) — Historial de decisiones de diseno (colaboracion Claude + GPT)
- [OPS.md](OPS.md) — Guia operativa
- [PRE_PROD.md](PRE_PROD.md) — Checklist de hardening pre-produccion
