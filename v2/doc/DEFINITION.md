# Nexus v2 Product Definition

Relacionado:

- [README.md](README.md)
- [MVP.md](MVP.md)
- [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md)
- [ROADMAP.md](ROADMAP.md)
- [OPS.md](OPS.md)

## Que es Nexus

Nexus es el nexo unico en dos sentidos. **Siempre que hay una interaccion con el producto, esta Nexus.**

**Con los clientes**: del otro lado suelen estar **agentes** (bots, sistemas de los clientes). Nexus recibe sus requests, los procesa, gobierna la autoridad delegada antes de que toquen infraestructura que mueve fondos. Decide, limita, supervisa e interviene. No custodia ni ejecuta.

**Con los equipos** (interno): del otro lado estan **humanos** (treasury, ops, security, compliance). Un solo **agente de Nexus** — el mismo para todos — se comunica por el **mismo chat**: notifica anomalias, contextualiza, ofrece opciones rapidas (botones, forms) y, en evolucion posterior, hace de nexo entre equipos y responde preguntas sobre documentacion y proyectos. No hace falta hablar con otro equipo para coordinar ni buscar en otra herramienta: se habla con Nexus.

En resumen: una voz, dos nexos — agentes (clientes) y humanos (equipos) — y un solo lugar para operar y preguntar: el mismo chatbot, en web y en app movil.

## Tesis del producto

La automatizacion financiera util ya no va a consistir en bots tontos ejecutando una sola instruccion. Va a consistir en agentes que:

- reciben objetivos
- elaboran planes
- ejecutan multiples pasos
- consumen autonomia delegada
- pueden desviarse del comportamiento esperado

Y los equipos humanos que supervisan esas operaciones estan fragmentados: treasury no sabe que hace security, ops no sabe que aprobo compliance, y nadie tiene una vista unificada.

El problema no es solo "esta transaccion es valida?". El problema es:

- que autoridad tiene este agente
- dentro de que objetivo y presupuesto puede operar
- cuanto riesgo ya consumio
- si se esta desviando de lo esperado
- cuando hay que intervenir, reducir scope o revocar la sesion
- quien necesita saberlo y que opciones tiene

Nexus existe para resolver eso. Para los agentes Y para los humanos.

## Problema que resuelve

Las empresas con operaciones financieras automatizadas ya tienen bots, scripts, playbooks y, cada vez mas, agentes con permisos altos y contexto incompleto.

Sin una capa de gobierno explicita:

- un agente puede operar fuera de los limites que el humano cree haber delegado
- una accion tecnicamente valida puede ser operacionalmente catastrofica
- el equipo termina revisando excepciones tarde, no gobernando autonomia a tiempo
- approvals, evidencia, auditoria e incidentes quedan dispersos
- la coordinacion entre equipos depende de Slack, emails y reuniones
- la operacion se vuelve dificil de explicar, revisar y corregir

El problema no es solo ejecutar.
El problema es hacer utilizable la autonomia sin perder control, y coordinar a los humanos que supervisan sin que dependan de canales informales.

## Propuesta de valor

Nexus hace posible delegar autonomia operativa sin entregar autoridad ilimitada, y coordina a los equipos humanos alrededor de las decisiones que importan.

Mensaje central:

> Ningun bot o agente deberia operar dinero con autoridad plena fuera de Nexus. Ningun equipo deberia coordinar respuestas criticas sin Nexus.

Nexus:

- define perimetros de autonomia para agentes y sistemas automatizados
- evalua cada accion en el contexto de esa autonomia delegada
- mide riesgo, presupuesto consumido y desvio del comportamiento esperado
- decide si deja pasar, frena, escala o reduce el alcance operativo
- deja auditoria explicable de que paso y por que
- le da a cada equipo humano el contexto que necesita con las opciones que corresponden
- mantiene runbooks, policies y handoffs como documentacion viva
- conecta treasury, ops, security y compliance sin que tengan que buscarse entre si

## Los 6 pilares

Todo lo que Nexus hace cae en uno de estos pilares:

### 1. Delegate
Delegar autoridad de forma explicita y acotada. El humano (o un agente superior) define que puede hacer el agente, sobre que, por cuanto, y durante cuanto tiempo.

### 2. Govern
Aplicar reglas de gobernanza en tiempo real. Policies, limites, restricciones, compliance rules. Evaluar cada accion en el contexto de la autonomia delegada.

### 3. Contain
Contener el blast radius si algo sale mal. Reducir autonomia, revocar sesion, bloquear scope. Intervenir proporcionalmente antes de que el dano escale.

### 4. Explain
Explicar cada decision de forma auditable y comprensible. El analista IA contextualiza para humanos. Cada equipo recibe la explicacion en su lenguaje.

### 5. Prove
Demostrar ante auditores, reguladores y terceros que todo se hizo correctamente. Audit trail inmutable, evidence chain, policy snapshots, replay.

### 6. Learn
Mejorar continuamente basado en lo que observa. Baselines que maduran, anticuerpos de incidentes, policy tuning. Simulation y replay para calibrar controles.

## Primitive central

La forma final del producto gira alrededor de:

- `AgentSession` — identidad y ciclo de vida del agente
- `GoalEnvelope` — intencion declarada contra la que se mide comportamiento
- `CapabilityLease` — autonomia delegada acotada
- `AutonomyBudget` — presupuesto de riesgo consumible
- `Intervention` — respuesta proporcional
- `Audit` — evidencia verificable para todos

En el estado actual de `v2`, la implementacion opera sobre:

- `Action`, `Resource`, `Policy`, `Approval`, `Lease`

Eso es la base actual del runtime. No es la forma final del producto.

## Las dos partes del sistema

Nexus se construye en dos partes claras:

### 1. Engine determinista (data-plane)

Recibe requests, los procesa (evalua, decide, emite leases). Si hay una **anomalia** (riesgo alto, desvio de baseline, accion bloqueada, require_approval, etc.), no resuelve solo: **notifica al agente IA de Nexus**. La autoridad de decision sigue en el engine; determinista, auditable, predecible.

### 2. Agente IA de Nexus (ai-runtime)

Recibe las anomalias del engine. **Notifica al equipo responsable** del area donde ocurrio, **contextualiza** (que paso, por que, que impacto) y **crea botones con acciones rapidas** (aprobar, rechazar, escalar, ver mas). El agente no decide allow/deny: presenta opciones; el humano ejecuta.

Bucle inicial: **anomalia → notificar → contexto y opciones a los responsables.** A partir de ahi el mismo agente puede evolucionar para ser el **nexo entre equipos** y el lugar donde se pregunta por **documentacion de proyectos**, runbooks y demas — un solo chatbot para operar y para preguntar.

### Interfaz unica: un agente, un chat

Para todos los equipos es **el mismo agente**, que se comunica siempre por el **mismo chat**. Una web donde el agente puede crear forms, botones, presentar diagramas y contexto; y una **app movil** con la misma experiencia. Un solo lugar, una sola voz.

Implementacion:
- Core determinista (data-plane) + operadores (control-workers) para side-effects y playbooks
- Agente IA (ai-runtime) como unico interlocutor para humanos
- Superficie de administracion (control-plane) para configuracion, auditoria y gestion

### Evolucion posterior: nexo real entre equipos

El bucle inicial (anomalia → contexto → opciones) usa solo lo que ya fluye por Nexus: acciones, aprobaciones, incidentes, alertas, cambios de config.

Para que Nexus sea el nexo real entre equipos necesita ademas saber que hace cada equipo: quien es dueno de que, en que trabajan, handoffs, dependencias. Sin eso, no puede conectar “el equipo A necesita X” con “el equipo B hace X”.

La direccion es integrar las herramientas donde los equipos ya trabajan (repos, trackers de tareas, etc.) para que Nexus tenga esa vista sin que nadie escriba nada adicional. Sin esa integracion, el rol de nexo entre equipos queda a medias.

## Nicho inicial

El nicho inicial de Nexus es:

- operaciones criticas en infraestructuras cripto automatizadas

Casos iniciales:

- withdrawals
- treasury transfers
- hot to cold wallet moves

Crypto es el wedge, no el techo. Las primitivas se disenan agnosticas desde el dia 1 para servir a cualquier servicio financiero AI-native.

## Punto de enforcement

```text
human
  |
  v
agent / bot / system
  |
  v
Nexus
(governs delegated autonomy + coordinates teams)
  |
  v
wallet / signer / execution system
  |
  v
blockchain
```

El sistema ejecutor solo deberia proceder si la autoridad vigente emitida por Nexus sigue siendo valida para esa accion, ese recurso, esa sesion, ese objetivo, y ese presupuesto.

## Diferenciador

Nexus no es otro policy engine. Nexus no es otro alerting tool.

- no solo evalua transacciones — gobierna autonomia delegada
- no solo bloquea o permite — interviene proporcionalmente
- no solo alerta — explica, contextualiza y ofrece acciones
- no solo sirve a un equipo — conecta a todos los equipos alrededor de las decisiones

> Nexus no es la puerta que mira una transaccion. Es el nexo que gobierna agentes y coordina humanos en operaciones financieras criticas.

## Componentes del sistema

### 1. Engine determinista (data-plane + control-workers)

Data-plane: recibe requests, evalua, decide, emite leases. Cuando detecta anomalia, notifica al agente IA. Control-workers: abren incidentes, envian alertas, ejecutan playbooks. La autoridad de decision esta aqui; determinista, auditable.

### 2. Agente IA de Nexus (ai-runtime)

Un solo agente para todos los equipos. Consume anomalias del engine; notifica al equipo responsable, contextualiza y ofrece acciones rapidas (botones, forms). Puede evolucionar a nexo entre equipos y respuesta sobre documentacion y proyectos. No es la autoridad final — el humano decide; el agente presenta opciones.

### 3. Superficie de administracion (control-plane)

Configuracion, recursos, policies, auditoria, tenant settings, billing. Expone la gestion del sistema.

### 4. Interfaz de chat (web + app movil)

El mismo chat para todos: web (forms, botones, diagramas) y app movil. Un solo punto de contacto con Nexus para humanos.

## Estado actual de v2

Hoy `v2` tiene una base real y operativa centrada en control por accion, con saas-core integrado para billing, auth y tenancy.

### Implementado en runtime

- risk scoring multi-factor con cascada y amplificacion no-lineal
- decision graduada: `allow`, `enhanced_log`, `additional_auth`, `require_approval`, `deny`
- baselines estadisticas por recurso y actor con confidence saturante
- known destinations con decay exponencial
- canary resources y trap policies
- hysteresis, cold start conservador
- graceful degradation con DegradationCollector per-request
- idempotencia en POST /v1/actions
- `RiskProfile` versionado (builtin `balanced/v1`)

### Integrado via saas-core

- auth (JWT/JWKS, API keys, Clerk webhooks, OIDC)
- billing (Stripe checkout/portal/webhooks, plans, subscriptions, dunning)
- tenancy (orgs, memberships, roles, tenant settings, hard limits)
- usage metering (counters, dedup, middleware)
- admin (tenant lifecycle, activity log)

### Direccion futura

- Bucle cerrado: engine notifica al agente IA → agente notifica a responsables con contexto y botones
- Interfaz unica: mismo chat en web y app movil (forms, botones, diagramas)
- Evolucion del agente: nexo entre equipos, preguntas sobre documentacion y proyectos
- `AgentSession`, `GoalEnvelope`, `CapabilityLease`, `AutonomyBudget`; intervenciones proporcionales

El roadmap esta en [ROADMAP.md](ROADMAP.md).
La guia operativa esta en [OPS.md](OPS.md).

## Que Nexus no es

- un custodio
- un signer
- un wallet
- un sistema que mueve fondos por si mismo
- un copiloto generalista
- un SIEM generalista
- un chatbot genérico sin gobierno ni contexto operativo (Nexus es un agente con contexto del producto y los equipos)
- un agente autonomo con poder final sin controles (el engine decide; el agente notifica y ofrece opciones; el humano ejecuta)

## Buyer inicial

- Head of Security
- COO o Head of Operations
- Treasury Lead
- responsables de plataforma o infraestructura operativa

Cliente inicial ideal:

- exchanges cripto con bots en produccion
- custodios
- plataformas con treasury automatizado

## Negocio

Nexus se vende como runtime de gobierno para operaciones financieras automatizadas y coordinacion de equipos alrededor de decisiones criticas.

El valor economico viene de:

- reducir riesgo operacional
- reducir probabilidad de perdida de fondos
- hacer utilizable la automatizacion de alto impacto
- centralizar governance, approvals, evidencia y auditoria
- eliminar la coordinacion informal entre equipos para respuestas criticas
- dar una capa de supervision, intervencion y documentacion viva

## Expansion

1. crypto ops automatizadas
2. autonomia delegada para agentes financieros
3. fintech y pagos
4. banca
5. sistemas criticos operados por agentes

## Frase de producto

> Nexus: el nexo entre agentes (clientes) y lo protegido, y entre los equipos que supervisan. Un engine determinista que detecta anomalias; un solo agente que notifica, contextualiza y ofrece acciones rapidas en el mismo chat (web y app). Una voz, un lugar.

## Definicion corta

> Nexus es el runtime de gobierno para agentes financieros y el nexo con los equipos humanos. El engine procesa requests y, ante anomalias, notifica al agente IA; el agente avisa a los responsables, da contexto y opciones rapidas en un solo chat (web y app). Mismo agente para todos; puede evolucionar a nexo entre equipos y consulta de documentacion. No custodia fondos ni firma: gobierna la autonomia y pone el canal unico para operar y preguntar.
