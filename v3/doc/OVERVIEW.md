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

También clasifica el **riesgo** de la acción (bajo, medio, alto). Una acción de riesgo alto siempre requiere aprobación, aunque no haya regla que la bloquee.

### 2. Registrar

Todo queda guardado: quién pidió qué, cuándo, qué decidió Nexus, quién aprobó, cuánto tardó, qué pasó después.

Esto permite reconstruir la historia completa de cualquier acción (**replay**). Útil para postmortems, auditorías y compliance.

### 3. Aprender

Nexus analiza las decisiones humanas acumuladas. Si detecta que un tipo de acción fue aprobado el 95% de las veces en las últimas 2 semanas, le propone al equipo: *"¿Querés que esto se apruebe automáticamente?"*

El humano revisa la propuesta y decide si aceptarla. Si acepta, Nexus crea la regla y deja de pedir aprobación para esas acciones.

**Resultado: menos interrupciones con el tiempo.**

---

## Quién puede pedir acciones

Nexus no le importa quién pide. Acepta acciones de:

- **Agentes IA** (bots de operaciones, triage automático)
- **Servicios** (deploy pipelines, monitoring, scripts)
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

La interfaz web tiene 5 secciones:

| Sección | Qué muestra |
|---------|-------------|
| **Inbox** | Acciones pendientes de aprobación con resumen IA |
| **Policies** | Crear, editar, archivar, eliminar reglas |
| **Replay** | Historia completa de cualquier acción (quién, cuándo, qué pasó) |
| **Learning** | Propuestas automáticas de nuevas reglas |
| **Dashboard** | Métricas: cuántas acciones, cuántas aprobadas, cuántas denegadas |

Disponible en **inglés y español** (selector en la barra superior).

---

## Cómo funciona por dentro (simplificado)

```
1. Llega una acción
2. Nexus busca si alguna regla aplica
3. Si una regla dice "denegar" → deniega
4. Si una regla dice "pedir aprobación" → va al inbox
5. Si ninguna regla aplica → clasifica riesgo:
   - Riesgo alto → va al inbox
   - Riesgo bajo/medio → aprueba automáticamente
6. Si va al inbox:
   - IA genera resumen para el aprobador
   - El aprobador decide (con confirmación obligatoria)
7. Todo queda registrado paso a paso
8. Con el tiempo, Nexus propone nuevas reglas basadas en lo que los humanos aprobaron
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

## Métricas clave del PoC

- **21 endpoints** de API funcionando
- **6 módulos** de backend (requests, policies, approvals, audit, learning, dashboard)
- **5 secciones** en la consola web
- **28 tests** automatizados de smoke y end-to-end
- **~60% cobertura** de tests unitarios
- **3 containers** Docker (backend, frontend, base de datos)
- **i18n** inglés y español

---

## Roadmap simplificado

| Fase | Estado | Qué incluye |
|------|--------|------------|
| **PoC** | ✅ Completo | Motor de decisión, reglas CEL, inbox con IA, replay, learning, consola web |
| **MVP** | Próximo | Optimizaciones de performance, validación de reglas al crear, paginación, rate limiting |
| **Producción** | Futuro | Multi-equipo, webhooks, SDK para integración, CI/CD, monitoring avanzado |
