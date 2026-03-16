# Nexus v2 Product Definition

Relacionado:

- [README.md](README.md)
- [MVP.md](MVP.md)
- [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md)
- [ROADMAP.md](ROADMAP.md)
- [OPS.md](OPS.md)

## Que es Nexus

Nexus es una capa determinista de control previo para acciones criticas automatizadas.

Su trabajo es decidir si una accion puede ocurrir antes de que toque infraestructura que puede mover, autorizar o enrutar fondos.

Nexus no custodia fondos.
Nexus no firma transacciones.
Nexus decide si pueden ocurrir.

## Nicho inicial

El nicho inicial de Nexus es:

- operaciones criticas en infraestructuras cripto automatizadas

Casos iniciales:

- withdrawals
- treasury transfers
- hot to cold wallet moves

## Problema que resuelve

Las empresas con operaciones cripto automatizadas ya tienen bots, scripts, playbooks y agentes que pueden proponer o disparar acciones sensibles.

El problema no es solo ejecutar esas acciones.
El problema es mantener control antes de que esas acciones toquen sistemas que pueden mover fondos o cambiar superficies criticas.

Sin una autoridad previa:

- un bot puede mover fondos sin control humano suficiente
- un sistema puede ejecutar una accion valida tecnicamente pero riesgosa operativamente
- la aprobacion, la evidencia y la auditoria quedan dispersas
- el sistema se vuelve dificil de explicar, revisar y gobernar

## Propuesta de valor

Nexus agrega una autoridad determinista antes de la ejecucion.

Mensaje central:

> Ningun bot o agente puede mover fondos sin pasar por Nexus.

Nexus:

- evalua la accion
- aplica policy
- calcula riesgo determinista
- genera evidencia
- decide `allow`, `deny` o `require_approval`
- si corresponde, emite un lease efimero de ejecucion
- deja auditoria explicable

## Lo que protege

Nexus protege acciones criticas como:

- retiros
- transferencias de treasury
- movimientos hot to cold
- cambios de whitelist
- cambios de limites
- rotacion de claves

La primera etapa de producto se enfoca solo en:

- `withdrawal`
- `treasury_transfer`
- `hot_to_cold_move`

## Punto de enforcement

La arquitectura objetivo es:

```text
bot / script / agent
        |
        v
      Nexus
    (decide)
        |
        v
wallet / signer / execution system
        |
        v
blockchain
```

El sistema ejecutor solo deberia poder proceder si presenta una autorizacion valida emitida por Nexus para esa accion, ese recurso y esa ventana temporal.

## Componentes del sistema

### 1. Core determinista

Es la autoridad.

Responsabilidades:

- policy engine
- evaluacion de acciones
- approvals
- limites
- riesgo determinista
- evidencia
- auditoria

El core debe ser:

- auditable
- predecible
- confiable

### 2. Operadores deterministas

Automatizaciones que no piensan.
Ejecutan reglas y playbooks seguros.

Ejemplos:

- recolectar evidencia
- abrir incidentes
- enviar alertas
- activar cuarentenas
- aplicar limites

### 3. Agente IA experto

La IA no decide acciones criticas.

Su rol es:

- analizar eventos
- explicar decisiones del core
- contextualizar riesgo
- priorizar incidentes
- asistir a humanos

En el nicho inicial, el agente sera experto en operaciones cripto.

## Mapa de componentes

La separacion operativa de `v2` queda asi:

- `data-plane = decidir`
- `control-workers = operar`
- `ai-runtime = asistir`
- `control-plane = administrar`

En concreto:

- `data-plane`
  Decide sobre acciones criticas en runtime.
  Evalua policy, riesgo, evidencia, approval y lease antes de la ejecucion.

- `control-workers`
  Ejecuta automatizaciones deterministas y operativas.
  Recolecta evidencia, abre incidentes, manda alertas y corre playbooks seguros.

- `ai-runtime`
  Asiste a humanos y contextualiza el sistema.
  Analiza, explica, prioriza y responde, pero no decide acciones criticas.

- `control-plane`
  Administra configuracion y superficie de gestion.
  Expone CRUDs, recursos, policies y la capa de administracion del sistema.

## Estructura compartida

En `v2`, lo agnostico queda separado asi:

- `v2/pkgs/contracts`
  - contratos y artefactos multilenguaje
  - OpenAPI, errores, eventos y otras definiciones compartidas
- `v2/pkgs/go-pkg`
  - codigo Go compartido y agnostico entre servicios

## Centro actual de `v2`

Hoy el centro de producto y de dominio de `v2` es:

- `control-plane`
  - `resources`
  - `action policies`
  - `audit records`
  - write interno por `/internal/audit`
  - lectura administrativa por `/v1/audit`

- `data-plane`
  - `actions`
  - `approvals`
  - `leases`
  - `execute`
  - emision best effort de `audit` para `action_created`, `action_blocked`, `action_approved`, `action_rejected`, `action_leased`, `action_executed` y `action_execution_failed`

- `control-workers`
  - `incidents`
  - apertura determinista de incidentes desde `data-plane/actions` cuando una accion queda bloqueada, rechazada o falla al ejecutar
  - `alerts`
  - apertura determinista de alerts desde `incidents` segun severidad
  - emision best effort de `audit` para `incident_created` y `alert_created`

La direccion principal de producto es `action/resource/policy/approval/lease`.
El eje anterior `run/tool` ya fue retirado de la superficie publica de `v2`.

Capacidades agregadas post-MVP (Fase 0 — hardening):

- idempotencia en `POST /v1/actions` via header `Idempotency-Key`
- graceful degradation en `data-plane` con cache local de resources y policies
- si `control-plane` no esta disponible, `data-plane` usa cache con TTL
- si el cache expiro o no existe, `data-plane` hace fail closed (deny)
- marcado de `degraded_context` en audit via `DegradationCollector` per-request en context

Capacidades agregadas post-MVP (Fase 1A — ya implementada en runtime):

- risk scoring multi-factor con cascada: factores pro-riesgo y anti-riesgo con amplificacion no-lineal
- 5 niveles de decision graduada: allow, enhanced_log, additional_auth, require_approval, deny
- baselines estadisticas por recurso y actor: daily_action_count, avg_amount, typical_hours
- known destinations con decay exponencial y confidence
- canary resources via label interno `_nexus_trap` y trap policies con `is_trap=true`
- hysteresis: el risk score actual se mezcla con el anterior para evitar oscilaciones
- cold start conservador: recursos nuevos arrancan con mas friccion
- `RiskProfile` versionado (builtin `balanced/v1`; CRUD desde control-plane pendiente para 1B)

El roadmap post-MVP esta documentado en [ROADMAP.md](ROADMAP.md).
La guia operativa esta documentada en [OPS.md](OPS.md).

## Que Nexus no es

Nexus no es:

- un custodio
- un signer
- un wallet
- un sistema que mueve fondos por si mismo
- un agente autonomo con poder de decision final
- un SIEM generalista

## Buyer inicial

Buyers probables al inicio:

- Head of Security
- COO o Head of Operations
- Treasury Lead
- responsables de plataforma o infraestructura operativa

Cliente inicial ideal:

- exchanges cripto
- custodios
- plataformas con treasury automatizado

## Negocio

Nexus es un negocio SaaS de seguridad y control operativo.

Se vende como capa de control para operaciones financieras automatizadas de alto impacto.

Rango de pricing esperado:

- Starter: 1.5k USD / mes
- Growth: 5k USD / mes
- Enterprise: 15k+ USD / mes

El valor economico viene de:

- reducir riesgo operacional
- reducir probabilidad de perdida de fondos
- mejorar gobernanza sobre automatizaciones
- centralizar approvals, evidencia y auditoria
- hacer explicable el control sobre sistemas automatizados

## Alcance inicial real de producto

El alcance inicial real debe mantenerse acotado a:

- crypto ops automatizadas
- pocas acciones iniciales
- pocas decisiones iniciales
- un punto de enforcement claro
- IA fuera del camino critico

Decisiones iniciales:

- `allow`
- `deny`
- `require_approval`

## Expansion

Orden esperado de expansion:

1. crypto ops automatizadas
2. fintech y pagos
3. banca
4. sistemas criticos operados por agentes

## Frase de producto

> Nexus permite que sistemas automatizados ejecuten acciones criticas sin perder control humano.

## Definicion corta

> Nexus es una capa determinista de control previo para operaciones cripto automatizadas.
> Evalua acciones criticas, aplica policy y approval, genera evidencia y, solo si corresponde, emite una autorizacion efimera consumible por el sistema que efectivamente ejecuta.
> Nexus no custodia fondos ni firma transacciones: decide si pueden ocurrir.
