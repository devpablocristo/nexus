# Nexus Review — Product Proposal v0.1

Estado: Draft
Owner: Pablo
Fecha: 17 de marzo de 2026
Objetivo del documento: evaluar si Nexus Review puede ser el primer producto vendible de la startup, con alcance acotado, foco realista y una evolucion razonable hacia una plataforma mayor de control de autonomia.

## 1. Resumen ejecutivo

Nexus Review es una capa de control para agentes que evalua, aprueba o bloquea cada accion propuesta, registra todas las acciones de punta a punta —propuesta, contexto, decision, aprobacion humana si la hubo y resultado final— y aprende con el tiempo de aprobaciones, rechazos y patrones para mejorar reglas, riesgo y autonomia; por ejemplo, si un agente quiere actualizar un registro sensible, Nexus decide si pasa, se frena o requiere aprobacion humana y deja todo ese ciclo documentado.

Nexus Review es una capa de evaluacion, aprobacion, registro y replay para agentes internos que proponen acciones dentro de workflows empresariales.

La idea no es arrancar por el Nexus "full" ni por un runtime critico final. La idea es resolver primero un dolor mas concreto y vendible: hoy muchas empresas ya tienen agentes internos o copilots, pero no cuentan con una forma clara de:

- revisar que accion propuso el agente,
- aplicar reglas simples y consistentes,
- elevar a aprobacion humana cuando corresponde,
- y conservar un registro completo de todo lo que el agente intento hacer, incluyendo lo permitido, lo rechazado, lo aprobado, lo ejecutado y lo fallido.

Nexus Review entra en ese hueco. No custodia credenciales criticas ni bloquea pagos desde el dia uno. En su primera version, funciona como una capa de oversight operativo para acciones internas no criticas, disenada para que la empresa mantenga control total sobre la actividad de sus agentes.

La hipotesis es que esta version:

- resuelve un dolor real,
- puede ser construida por un equipo de 3 personas,
- tiene mas chance de venderse que un runtime critico full,
- y deja una ruta logica para evolucionar hacia una plataforma mayor de control de autonomia.

## 2. Problema

Muchas empresas estan experimentando o ya operando con agentes internos para tareas como:

- clasificar o mover trabajo,
- actualizar registros,
- generar respuestas,
- proponer decisiones operativas,
- ejecutar pequenas acciones dentro de herramientas internas,
- preparar casos para revision humana.

El problema no es que no existan agentes. El problema es que cuesta confiar en ellos operativamente.

Hoy suele faltar una capa clara para:

- revisar acciones propuestas,
- aplicar politicas de forma consistente,
- decidir cuando algo requiere aprobacion humana,
- dejar evidencia clara,
- reconstruir lo ocurrido ante errores o incidentes,
- y garantizar que el usuario o la organizacion no pierdan visibilidad ni control sobre lo que el agente intento hacer.

En otras palabras: el cuello de botella ya no es solamente "construir agentes", sino operarlos con confianza, control y trazabilidad.

## 3. Por que ahora

Hay cuatro razones para pensar que este producto tiene sentido ahora.

### 3.1 Los agentes ya llegaron a la empresa

La conversacion ya no es solamente experimental. Cada vez mas organizaciones estan llevando agentes a workflows internos reales.

### 3.2 La confianza operativa sigue siendo insuficiente

El problema dejo de ser unicamente "que tan inteligente es el modelo" y paso a ser "como lo superviso, cuando lo dejo actuar, como conservo control y como reconstruyo errores".

### 3.3 Hay espacio para una capa intermedia

Entre el agente experimental y la autonomia fuerte hay una necesidad clara: una capa que ordene decisiones, approvals y trazabilidad sin exigir todavia la confianza extrema que requiere un runtime critico final.

### 3.4 Hay una entrada mas realista para un equipo chico

Entrar por enforcement critico exige mas credenciales, confianza institucional y ventas enterprise complejas. Entrar por review, approval, registro y replay es una posicion comercial mucho mas creible para un equipo de 3 personas.

## 4. Propuesta

### 4.1 Que es Nexus Review

Nexus Review es una capa donde un agente no ejecuta libremente, sino que propone una accion dentro de un workflow empresarial.

Nexus recibe esa propuesta, la evalua con reglas y riesgo, y produce una de estas salidas:

- **allow**
- **require approval**
- **deny**

Pero su funcion no termina ahi.

Nexus Review tambien registra integramente el ciclo de vida de cada accion propuesta por el agente, incluyendo:

- la propuesta original,
- la evaluacion aplicada,
- la decision automatica,
- la intervencion humana si existio,
- y el resultado final.

Eso significa que Nexus Review documenta tanto:

- acciones permitidas,
- como acciones rechazadas,
- acciones elevadas a aprobacion,
- acciones aprobadas,
- acciones ejecutadas,
- y acciones fallidas.

### 4.2 Que resuelve

Nexus Review permite que una empresa:

- despliegue agentes internos con mas confianza,
- reduzca la revision manual desordenada,
- aplique politicas simples de manera consistente,
- tenga aprobacion humana donde corresponde,
- mantenga control total sobre lo que los agentes intentan hacer,
- y pueda reconstruir un caso completo cuando algo sale mal.

### 4.3 Que NO intenta resolver al principio

No intenta ser:

- runtime critico final,
- custodio de credenciales sensibles,
- plataforma universal de agentes,
- SIEM,
- observabilidad generica de LLMs,
- orquestador multiagente enterprise.

## 5. Principio central del producto

La unidad central que procesa Nexus Review es: **la accion propuesta por un agente**.

Monitorea y registra eventos del tipo:

- que quiere hacer el agente,
- sobre que objeto o sistema,
- con que contexto,
- con que justificacion,
- bajo que reglas,
- con que nivel de riesgo,
- que decision automatica tomo Nexus,
- si hubo intervencion humana,
- y cual fue el resultado final.

Por lo tanto, Nexus Review no se define por la herramienta destino, sino por el problema que resuelve:

**revision, aprobacion, registro y trazabilidad integral de acciones propuestas por agentes en workflows empresariales.**

## 6. Tesis de control

Nexus Review parte de una idea simple:

> Si un agente puede actuar, la empresa debe poder ver, entender, revisar y reconstruir todo lo que ese agente intento hacer.

Eso implica cuatro principios:

### 6.1 Nada relevante debe ocurrir "en negro"

Toda accion propuesta por un agente que entre bajo control de Nexus debe quedar registrada.

### 6.2 El registro no se limita a lo aprobado

El valor del producto no esta solo en registrar lo que salio bien, sino tambien en registrar:

- lo que fue rechazado,
- lo que requirio aprobacion,
- lo que fue aprobado pero fallo,
- y lo que termino cancelado o abortado.

### 6.3 La trazabilidad debe ser comprensible

No alcanza con guardar logs tecnicos. El sistema debe permitir entender, en terminos operativos:

- que quiso hacer el agente,
- por que,
- bajo que reglas,
- con que decision,
- y que termino ocurriendo.

### 6.4 El usuario debe conservar capacidad de intervencion

El producto tiene que permitir que un humano:

- revise,
- apruebe,
- rechace,
- audite,
- y reconstruya el comportamiento del agente sin quedar ciego frente a la automatizacion.

## 7. Dolor inicial y recorte del wedge

### Wedge inicial

La primera version no apunta a "gobernanza de todos los agentes empresariales".

Apunta a un problema mas acotado:

**revision, aprobacion, registro y replay de acciones propuestas por agentes internos en workflows empresariales no criticos.**

### Que significa "workflow no critico"

Un workflow no critico es uno donde:

- existe riesgo o friccion real,
- hay valor en revisar y dejar evidencia,
- pero un error no implica todavia perdida financiera severa, incidentes regulatorios graves o acceso a infraestructura extremadamente sensible.

### Criterios para elegir la primera superficie

La primera superficie de entrada deberia cumplir estas condiciones:

- alta frecuencia de acciones,
- dolor entendible por el buyer,
- posibilidad real de errores o friccion,
- impacto visible al ordenar approvals y replay,
- riesgo comercial razonable,
- implementacion posible para 3 personas.

### Superficies iniciales posibles

Todavia no esta decidido cual sera la primera. Las candidatas iniciales son:

- soporte interno / IT ops,
- operaciones administrativas internas,
- revision documental y aprobaciones,
- workflows internos de producto o engineering,
- backoffice liviano.

## 8. Usuario, buyer e ICP inicial

### Usuario diario

- Platform Engineer
- AI Engineer
- Engineering Manager
- Security / Platform Ops
- Operations Lead, dependiendo del workflow elegido

### Buyer inicial

- Head of Engineering
- Platform Lead
- AI Platform Lead
- CTO de empresa mediana tech
- eventualmente un Operations Lead, si la entrada fuera por operaciones

### ICP inicial

Empresas de software o empresas digitalizadas de aproximadamente 200 a 3000 empleados que ya usan o estan lanzando agentes internos o copilots para mover trabajo, actualizar sistemas, proponer acciones o automatizar tareas internas.

La entrada inicial seria por equipos tecnicos u operativos con necesidad real de oversight, no por banca, compliance extremo ni casos financieros criticos.

## 9. Como funcionaria el producto

### 9.1 Ingesta de propuesta

Cada agente manda a Nexus una proposed_action con campos como:

- agent_id
- owner / principal
- workflow
- target_system
- action_type
- target_resource
- params
- reason
- context_summary
- declared_risk

### 9.2 Evaluacion

Nexus corre checks deterministas y simples, por ejemplo:

- esta accion requiere owner explicito,
- este tipo de cambio necesita aprobacion,
- ciertas combinaciones de contexto deben revisarse,
- ciertas acciones fuera de horario deben elevarse,
- ciertas categorias de objetos no pueden tocarse automaticamente.

### 9.3 Risk tiering

Clasificacion inicial:

- **low**
- **medium**
- **high**

### 9.4 Decision

Para cada accion propuesta, Nexus emite una decision:

- **allow**
- **require approval**
- **deny**

### 9.5 Approval inbox

Si corresponde, la propuesta va a una bandeja para revision humana con:

- resumen,
- accion propuesta,
- impacto esperado,
- reglas aplicadas,
- recomendacion,
- decision posible: aprobar / rechazar / pedir cambio.

### 9.6 Registro integral

Cada accion propuesta debe quedar documentada de extremo a extremo, incluyendo:

- propuesta original,
- contexto aportado,
- evaluacion aplicada,
- decision automatica,
- intervencion humana si existio,
- ejecucion o no ejecucion,
- resultado final.

### 9.7 Replay

Cada caso queda con timeline completa para reconstruccion posterior:

- que pidio el agente,
- con que contexto,
- que policy se aplico,
- que decision salio,
- quien aprobo o rechazo,
- que ocurrio despues,
- y en que estado final termino el caso.

### 9.8 Metricas

El producto deberia mostrar al menos:

- cantidad de propuestas por agente,
- approvals,
- rechazos,
- acciones por nivel de riesgo,
- politicas mas activadas,
- tiempo promedio de decision,
- tasa de ejecucion exitosa,
- tasa de fallas o abortos posteriores.

## 10. MVP

### 10.1 Definicion del MVP

El MVP no debe cubrir "todos los agentes", ni "todas las herramientas", ni "todos los workflows".

Debe cubrir:

- 1 workflow inicial,
- 2 o 3 conectores,
- 3 a 5 tipos de accion,
- 1 policy engine simple,
- 1 approval inbox,
- 1 vista de replay,
- 1 dashboard basico.

### 10.2 Componentes minimos

- 1 API de ingesta
- 1 policy engine simple
- 1 approval inbox
- 1 vista de replay
- 1 dashboard basico
- conectores a las herramientas necesarias del workflow elegido

### 10.3 Rol de IA en el MVP

La IA no gobierna el camino critico. Solo ayuda con:

- resumir contexto,
- explicar por que un caso cayo en revision,
- sugerir politicas futuras.

La decision principal en v1 debe seguir siendo:

- determinista, o
- humana.

### 10.4 Decision pendiente del MVP

Antes de construir, hay que cerrar esta definicion:

- cual es el workflow inicial,
- cuales son las herramientas minimas involucradas,
- cuales son las 3 a 5 acciones concretas que vamos a soportar.

## 11. Que no vamos a hacer ahora

Para evitar dispersion, esta primera etapa no incluye:

- pagos,
- movimientos de fondos,
- crypto ops,
- custodia de credenciales criticas,
- execution leases complejos,
- runtime final de enforcement,
- soporte multi-tenant enterprise complejo,
- conectores a decenas de herramientas,
- orquestacion multiagente,
- documentacion viva,
- knowledge graph,
- deteccion autonoma avanzada de fraude o riesgo.

## 12. Por que creo que esto se puede construir con 3 personas

Porque el MVP no requiere:

- entrenar modelos propios,
- hacer infraestructura critica de seguridad de maxima sensibilidad,
- resolver una categoria horizontal inmensa,
- ni construir una plataforma enterprise full desde el dia uno.

Si requiere:

- backend solido,
- reglas simples,
- buenas integraciones,
- UX de aprobacion clara,
- buen registro y buen replay,
- y criterio de producto.

Eso entra dentro de lo construible por 3 personas buenas en pocos meses.

## 13. Posicionamiento inicial

No lo venderiamos como "seguridad" pura. Tampoco como "observabilidad de LLMs".

La posicion inicial seria:

> Nexus Review ayuda a desplegar agentes internos sin perder control sobre lo que intentan hacer. Cada accion propuesta puede evaluarse, aprobarse cuando hace falta, registrarse de punta a punta y reconstruirse despues.

La idea es vender:

- confianza operativa,
- oversight,
- aprobacion ordenada,
- trazabilidad integral,
- control total sobre la actividad del agente,
- progresion hacia mayor autonomia.

## 14. Hipotesis de pricing inicial

Hipotesis preliminar:

- **Starter**: USD 1.5k-2.5k / mes
- **Growth**: USD 4k-8k / mes
- luego, expansion por volumen de propuestas, integraciones o workflows cubiertos

No cobraria por asiento al principio. Tiene mas sentido cobrar por valor operacional o superficie controlada.

## 15. Riesgos y dudas abiertas

### Riesgo 1
Que el dolor sea real, pero no suficientemente "budget-worthy" como producto standalone.

### Riesgo 2
Que el cliente prefiera resolver approvals y registro dentro de sus propias herramientas en vez de sumar una capa nueva.

### Riesgo 3
Que quedemos demasiado cerca de "observabilidad" y no se perciba una categoria clara.

### Riesgo 4
Que el wedge tecnico sea facil de copiar si no logramos buen replay, UX y policy layer.

### Riesgo 5
Que empecemos demasiado abstractos y sin un workflow con frecuencia suficiente.

### Riesgo 6
Que no logremos elegir bien la primera superficie y entremos por un caso demasiado chico o demasiado dificil.

## 16. Evolucion esperada del producto

### Etapa 1 — Review
- ingesta
- policy checks
- approvals
- registro integral
- replay
- metricas basicas

### Etapa 2 — Evals
- datasets de casos reales
- comparacion entre versiones de agentes
- scorecards por workflow
- deteccion de drift

### Etapa 3 — Oversight adaptativo
- auto-approval condicionado
- muestreo inteligente
- sugerencia de politicas
- intervencion por anomalias

### Etapa 4 — Control plane de autonomia
- agent identity
- scopes
- owners
- limites por herramienta / horario / entorno
- simulation mode

### Etapa 5 — Enforcement parcial
- permisos efimeros
- write actions mas delicadas
- rollback orchestration
- approvals por multiples niveles

### Etapa 6 — Plataforma amplia
En esta etapa Nexus ya podria convertirse en una capa estandar de control y confianza para autonomia empresarial.

## 17. Que tendria que pasar para que esto sea una buena idea

Para considerar que vale la pena seguir, deberiamos poder validar al menos estas 4 cosas:

1. que el dolor existe de forma clara en empresas que ya usan agentes internos;
2. que un buyer tecnico u operativo si pagaria por resolverlo;
3. que existe un workflow inicial suficientemente frecuente y entendible para entrar;
4. que review + registro + replay + approval aporta valor real por encima de observabilidad pura.

## 18. Proximos pasos propuestos

1. Validar el problema con entrevistas: 10 a 20 conversaciones con equipos que ya esten usando agentes internos.
2. Elegir la primera superficie: definir cual workflow inicial tiene mejor equilibrio entre dolor, riesgo y vendibilidad.
3. Refinar el wedge: decidir exactamente cuales 3 a 5 acciones cubririamos primero.
4. Armar RFC formal de MVP: endpoints, data model, flows, pantallas, conectores.
5. Construir demo navegable + prototype funcional.

## 19. Decision buscada con este documento

Este documento no busca aprobar todavia el desarrollo completo.

Busca decidir si:

- seguimos explorando Nexus Review como direccion real de producto,
- lo descartamos,
- o lo reformulamos antes de pasar a un RFC formal.

## 20. Preguntas para discutir en equipo

1. Este dolor es real y suficientemente urgente?
2. Entrar por review / approval / registro / replay es una mejor jugada que entrar por enforcement critico?
3. Cual es el workflow inicial correcto?
4. La propuesta se siente suficientemente vendible para un equipo de 3 personas?
5. Nos acerca a una startup grande o nos deja en una herramienta tactica demasiado chica?
6. Cual seria la demo minima que haria que alguien diga "esto lo necesito"?

## 21. Nota final

La decision mas importante en esta etapa no es tecnica. Es elegir bien el primer dolor y expresarlo con total claridad.

Si el problema inicial esta bien elegido, Nexus Review puede arrancar como una herramienta chica y evolucionar con logica hacia una plataforma mas grande de control de autonomia.

Si el problema inicial esta mal elegido, el producto corre el riesgo de quedar abstracto, poco vendible o demasiado debil.

La tesis central es esta:

> Si una empresa va a usar agentes, no deberia perder control sobre lo que esos agentes intentan hacer. Nexus Review existe para que cada accion relevante quede evaluada, decidida, registrada y reconstruible.
