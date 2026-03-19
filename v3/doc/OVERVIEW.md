# Nexus Review — Qué es y cómo funciona

## En una frase

Nexus Review es un sistema que **revisa, aprueba o rechaza acciones** antes de que se ejecuten, y aprende de las decisiones para automatizarse.

---

## El problema

Equipos de operaciones reciben cientos de acciones por día: silenciar alertas, ejecutar scripts, resolver incidentes, escalar problemas. Muchas de estas acciones las ejecutan bots o servicios automáticos.

**¿Quién decide si está bien?** Hoy nadie. O alguien revisa a mano en Slack. O se ejecuta y después se ve qué pasó.

---

## La solución

Nexus Review se pone **entre** quien pide la acción y la ejecución:

```
Alguien quiere hacer algo
        ↓
   Nexus Review evalúa
        ↓
   ┌────┼────┐
   ↓    ↓    ↓
 ✅    ❌    ⏳
Aprobar  Denegar  Pedir aprobación humana
```

### Tres decisiones posibles

1. **Aprobar** — la acción es segura, se ejecuta automáticamente
2. **Denegar** — la acción viola una regla, se bloquea
3. **Pedir aprobación** — la acción es riesgosa, un humano decide

---

## Los tres pilares

### 1. Decidir

Nexus evalúa cada acción contra un conjunto de **reglas** (policies). Cada regla dice: "si la acción es X en el sistema Y, entonces aprobar/denegar/pedir aprobación".

Ejemplo de regla: *"Si alguien quiere silenciar una alerta crítica fuera de horario laboral → pedir aprobación humana"*

También clasifica el **riesgo** de la acción evaluando 6 factores simultáneamente (tipo de acción, horario, historial del actor, frecuencia, tasa de éxito, sensibilidad del sistema destino). Cuando varios factores coinciden, el riesgo se amplifica — similar a como funciona la cascada de coagulación en biología. Una acción de riesgo alto siempre requiere aprobación, aunque no haya regla que la bloquee.

Además, el resultado de cada ejecución (éxito o fallo) retroalimenta el factor de tasa de éxito (F5). Si una acción históricamente falla mucho, su riesgo sube automáticamente. Esto crea un **feedback loop**: las decisiones mejoran con cada ejecución reportada.

### 2. Registrar

Todo queda guardado: quién pidió qué, cuándo, qué decidió Nexus, quién aprobó, cuánto tardó, qué pasó después.

Esto permite reconstruir la historia completa de cualquier acción (**replay**). Útil para postmortems, auditorías y compliance.

### 3. Aprender

Nexus analiza las decisiones humanas acumuladas. Si detecta que un tipo de acción fue aprobado el 95% de las veces en las últimas 2 semanas, le propone al equipo: *"¿Querés que esto se apruebe automáticamente?"*

El humano revisa la propuesta y decide si aceptarla. Si acepta, Nexus crea la regla y deja de pedir aprobación para esas acciones.

**Resultado: menos interrupciones con el tiempo.**

---

## Tipos de acción (ontología tipada)

Cada acción tiene un **tipo** definido: `alert.silence`, `runbook.execute`, `treasury.transfer`, etc. Estos tipos están registrados con metadata que incluye:

- **Categoría** — alert, runbook, incident, config, deploy, data, iam, treasury
- **Clase de riesgo** — low, medium, high, critical
- **Reversible** — si la acción se puede deshacer
- **Requiere break-glass** — si necesita múltiples aprobadores por defecto
- **Schema** — validación JSON de los parámetros esperados

Nexus viene con **9 tipos de acción** pre-configurados, pero se pueden crear, editar y eliminar desde la pestaña **Actions** de la consola (o via API).

Si una request llega con un `action_type` que no está registrado, Nexus la **rechaza** (403 FORBIDDEN). Esto garantiza que solo se procesen acciones conocidas y tipadas.

---

## Delegaciones de agentes

Nexus modela explícitamente **quién delega qué a quién**. Un owner (humano o equipo) puede delegar autoridad a un agente, especificando:

- **Qué acciones puede hacer** — lista de action types permitidos
- **Sobre qué recursos** — lista de recursos específicos
- **Propósito** — para qué se otorga la delegación
- **Riesgo máximo** — el nivel más alto de riesgo que el agente puede asumir
- **Expiración** — cuándo caduca la delegación

Si un agente intenta una acción que **no tiene delegada**, Nexus la rechaza (403 FORBIDDEN). Esto garantiza que cada agente opera dentro de los límites que su owner definió.

Las delegaciones se gestionan desde la pestaña **Agents** de la consola (o via API).

---

## Quién puede pedir acciones

Nexus no le importa quién pide. Acepta acciones de:

- **Agentes IA** (bots de operaciones, triage automático) — deben tener delegación vigente
- **Servicios** (deploy pipelines, monitoring, scripts) — deben tener delegación vigente
- **Humanos** (ingenieros, SREs, operadores)

---

## Ejemplos reales

| Acción | Quién pide | Nexus decide |
|--------|-----------|-------------|
| Silenciar alerta crítica por 4 horas | ops-bot | ⏳ Pedir aprobación (alto riesgo) |
| Escalar alerta al equipo on-call | ops-bot | ✅ Aprobar (bajo riesgo) |
| Ejecutar restart del API gateway | deploy-service | ⏳ Pedir aprobación (alto riesgo) |
| Resolver incidente INC-2847 | sre@empresa.com | ✅ Aprobar (riesgo medio, regla permite) |
| Borrar datos de producción | admin-script | ❌ Denegar (regla bloquea deletes en prod) |

---

## La experiencia del aprobador

Cuando Nexus decide que una acción necesita aprobación humana, la acción aparece en la **bandeja de entrada** (Inbox):

```
┌──────────────────────────────────────────────────┐
│  Nexus Review — Inbox                  3 pendientes │
├──────────────────────────────────────────────────┤
│                                                    │
│  🔴 ALTO   Silenciar alerta CPU-CRITICAL          │
│  "ops-bot quiere silenciar por 4h. Hay una         │
│   migración de DB en curso que explica el spike.   │
│   Recomendación: aprobar con duración reducida."   │
│                                                    │
│  Nota: ________________________________           │
│  Escribir APPROVE: ____________________           │
│  [Confirmar aprobación]  [Cancelar]                │
│                                                    │
│  🟡 MEDIO  Resolver incidente INC-2847            │
│  ...                                               │
│                                                    │
│  🟢 BAJO   Escalar alerta a equipo backend        │
│  ...                                               │
└──────────────────────────────────────────────────┘
```

Cada acción pendiente muestra:
- **Nivel de riesgo** con color (rojo = alto, amarillo = medio, verde = bajo)
- **Resumen generado por IA** que explica qué se pide, por qué se frenó, y qué recomienda Nexus
- **Campos de confirmación** obligatorios para prevenir aprobaciones accidentales

El aprobador típicamente decide en **menos de 10 segundos** gracias al resumen de IA.

---

## Las reglas (policies)

Las reglas se crean de dos formas:

### Manual

Un administrador crea la regla desde la interfaz:
- Nombre: "Bloquear deletes en producción"
- Condición: acción = delete Y sistema = producción
- Efecto: denegar

### Aprendida (automática)

Nexus detecta que un tipo de acción fue aprobado muchas veces y propone una regla:

```
💡 Propuesta: "Auto-aprobar escalaciones de alerta"
   96% aprobadas en los últimos 14 días (274 de 285)
   [Aceptar]  [Descartar]
```

Si el administrador acepta, la regla se crea automáticamente y futuras acciones de ese tipo se aprueban sin intervención.

---

## La consola

La interfaz web tiene 9 pestañas:

| Sección | Qué muestra |
|---------|-------------|
| **Inbox** | Acciones pendientes de aprobación con resumen IA. Badge de break-glass e indicador de progreso para aprobaciones multi-aprobador |
| **Requests** | Todas las requests con timeline inline y replay integrado |
| **Policies** | Crear, editar, archivar, eliminar reglas. Soporte shadow mode (evalúa sin actuar) con contador de hits y botón "Promote to enforced" |
| **Actions** | CRUD de action types: nombre, categoría, clase de riesgo, reversible, break-glass, schema JSON. 9 tipos pre-configurados |
| **Agents** | CRUD de delegaciones: owner → agente → action types permitidos → recursos → propósito → riesgo máximo → expiración |
| **Sandbox** | 5 modos: Simulate (dry-run), Batch Test (regression testing), Approval Sim (simular approve/reject), Shadow Monitor (policies en modo shadow), Replay Test (CEL contra historial) |
| **Learning** | Propuestas automáticas de nuevas reglas |
| **Dashboard** | Métricas: cuántas acciones, cuántas aprobadas, cuántas denegadas |
| **Config** | Configuración de riesgo, aprobaciones, learning, IA y general (5 secciones expandibles) |

La pestaña activa se mantiene al refrescar la página (F5).

Disponible en **inglés y español** (selector en la barra superior, con persistencia en localStorage).

---

## Break-glass: aprobación de emergencia

Algunas acciones son tan críticas que requieren **múltiples aprobadores**. Nexus soporta break-glass: cuando una request se marca como `break_glass: true`, se requieren N aprobaciones (configurable por `action_type` y `risk_level`).

Reglas:
- **Un rechazo cancela todo** — cualquier aprobador puede vetarla
- **El mismo aprobador no puede decidir dos veces** — se requieren personas distintas
- **Aprobación parcial visible** — el Inbox muestra el progreso (ej: "2/3 aprobaciones")
- **Configurable** — las reglas de break-glass se definen en la sección Config

Ejemplo: *"Borrar tabla en producción requiere 3 aprobadores. Si uno rechaza, se cancela."*

---

## Evidence packs (evidencia exportable)

Nexus puede generar un **paquete de evidencia** completo y firmado para cualquier request. El evidence pack incluye toda la cadena:

- **Request**: quién pidió qué, cuándo, con qué parámetros
- **Policy evaluation**: qué reglas se evaluaron y cuál aplicó
- **Approval**: quién aprobó/rechazó, cuándo, con qué nota
- **Execution**: resultado reportado (éxito/fallo, duración)
- **Attestation**: prueba verificable del executor (ver siguiente sección)
- **Timeline**: todos los eventos de audit ordenados cronológicamente
- **Firma HMAC-SHA256**: garantiza integridad del pack

Endpoint: `GET /v1/requests/{id}/evidence`. También disponible como botón de descarga en la consola.

Es lo que un auditor o regulador necesita ver: no solo "tenemos logs", sino **evidencia verificable y estructurada**.

---

## Outcome attestation (prueba de ejecución)

Después de que una acción se ejecuta, el executor (PEP, gateway, o servicio) puede **attestar** qué hizo realmente. No es solo "success: true" — es una prueba verificable con:

- **Status**: qué pasó (success, failure, partial)
- **Provider refs**: IDs externos que vinculan al registro real (ej: `{"tx_id": "bank_tx_555"}`)
- **Signature**: firma del executor
- **Attester**: identidad de quién attesta (ej: `pep:treasury_gateway`)
- **Metadata**: información adicional del executor

Endpoints: `POST /v1/requests/{id}/attest` + `GET /v1/requests/{id}/attestation`

Solo requests con status `executed` o `failed` pueden ser attestadas. La attestation se incluye automáticamente en los evidence packs.

---

## Simular antes de actuar

El **Sandbox** ofrece 5 modos de simulación:

### Simulate (dry-run)
Enviar una request de prueba. Nexus la evalúa exactamente igual que una real, pero no la persiste. El resultado muestra:

- **Decisión**: qué haría Nexus (aprobar, denegar, pedir aprobación)
- **Factores de riesgo**: cuáles se activaron y por qué
- **Amplificación**: si hay combinaciones sospechosas que potenciaron el riesgo
- **Score final**: el puntaje numérico y el nivel resultante

### Batch Test (regression testing)
Definir un conjunto de requests de prueba (hasta 100) y ejecutarlas todas a la vez. Nexus devuelve resultados agregados: cuántas permitidas, denegadas, requieren aprobación, desglose por nivel de riesgo. Ideal para regression testing de policies.

### Approval Simulation
Simular qué pasa si apruebo o rechazo una request pendiente, sin ejecutarlo. Muestra el resultado esperado incluyendo quorum de break-glass y decisiones parciales previas.

### Shadow Monitor
Seguimiento en tiempo real de policies en modo shadow: cuántos hits acumulan, qué requests afectarían, con botón para promover a enforced.

### Replay Test
Probar una expresión CEL propuesta contra el historial real de requests. Muestra cuántas serían afectadas y cómo cambiaría el resultado.

---

## Configuración

Todo es configurable desde la pestaña Config de la consola (o via API):

| Sección | Qué se configura |
|---------|-----------------|
| **Risk** | Qué acciones son alto/medio riesgo, umbrales de decisión |
| **Approvals** | TTL de aprobaciones, comportamiento de expiración |
| **Learning** | Umbrales de confianza, tamaño mínimo de muestra, ventana de tiempo |
| **AI** | Parámetros del contextualizador IA |
| **General** | Configuraciones generales del servicio |

Los cambios se aplican inmediatamente. Se puede restaurar la configuración por defecto con un solo click.

---

## Cómo funciona por dentro (simplificado)

```
1. Llega una acción
2. Nexus verifica que el action_type esté registrado (si no → 403 FORBIDDEN)
3. Nexus verifica que el agente tenga delegación vigente para esa acción (si no → 403 FORBIDDEN)
4. Nexus busca si alguna regla aplica (incluyendo shadow policies que evalúan sin actuar)
5. Si una regla dice "denegar" → deniega
6. Si una regla dice "pedir aprobación" → va al inbox
7. Si ninguna regla aplica → clasifica riesgo con 6 factores:
   - Tipo de acción, horario, historial del actor, frecuencia,
     tasa de éxito (alimentada por resultados reales), sensibilidad del destino
   - Si hay combinaciones sospechosas → amplifica el riesgo
   - Riesgo alto → va al inbox
   - Riesgo bajo/medio → aprueba automáticamente
8. Si va al inbox:
   - IA genera resumen para el aprobador
   - Si es break-glass → requiere N aprobadores (un rechazo cancela todo)
   - El aprobador decide (con confirmación obligatoria)
9. El requester ejecuta y reporta resultado (éxito/fallo)
10. El executor attesta qué hizo realmente (firma + refs del provider)
11. El resultado retroalimenta el factor de éxito → mejora futuras evaluaciones
12. Todo queda registrado paso a paso
13. Se puede exportar un evidence pack firmado con toda la cadena
14. Con el tiempo, Nexus propone nuevas reglas basadas en lo que los humanos aprobaron
```

---

## Stack técnico (resumen)

| Componente | Tecnología |
|-----------|-----------|
| Backend | Go (lenguaje de programación) |
| Base de datos | PostgreSQL (relacional) |
| Motor de reglas | CEL (Google Common Expression Language) |
| Resúmenes IA | Claude (Anthropic) |
| Frontend | React (JavaScript) |
| Infraestructura | Docker (contenedores) |

---

## Métricas clave del MVP (Q2)

- **45+ endpoints** de API funcionando
- **12 módulos** de backend (requests, policies, approvals, audit, learning, dashboard, config, shared, actiontypes, delegations, evidence + execution_stats/attestation en requests)
- **9 pestañas** en la consola web (Inbox, Requests, Policies, Actions, Agents, Sandbox, Learning, Dashboard, Config)
- **3 containers** Docker (backend, frontend, base de datos)
- **9 migraciones** de base de datos
- **9 action types** pre-configurados (alert.silence, alert.escalate, runbook.execute, incident.resolve, config.update, deploy.trigger, delete, iam.grant_role, treasury.transfer)
- **i18n** inglés y español con persistencia en localStorage
- **Cascade risk scoring** con 6 factores y amplificación multiplicativa
- **Ontología tipada** — action types registrados con schema, riesgo y metadata. Acción desconocida = 403
- **Delegation graph** — owner delega a agente con scope y expiración. Agente sin delegación = 403
- **Evidence packs** — JSON firmado (HMAC-SHA256) exportable con toda la cadena de evidencia
- **Outcome attestation** — prueba verificable del executor con signature y provider refs
- **Feedback loop** — resultados de ejecución retroalimentan el scoring de riesgo
- **Break-glass** — aprobación multi-aprobador para operaciones críticas
- **Sandbox** — 5 modos: simulate, batch test, approval sim, shadow monitor, replay test
- **Shadow policies** — evalúan sin actuar, con contador de hits y promoción a enforced

---

## Roadmap simplificado

| Fase | Estado | Qué incluye |
|------|--------|------------|
| **PoC** | ✅ Completo | Motor de decisión, reglas CEL, inbox con IA, replay, learning, consola web |
| **MVP (Q2)** | ✅ Completo | Ontología tipada, delegation graph, evidence packs, outcome attestation, sandbox avanzado (5 modos), tests 50%+ |
| **Enforcement (Q3)** | Próximo | Execution leases (JWT), PEP/SDK Go, integración vertical #1 |
| **Enterprise (Q4)** | Futuro | Multi-tenant, compliance packs, self-hosted |
