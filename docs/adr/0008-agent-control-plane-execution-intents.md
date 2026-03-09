# ADR 0008 - Agent Control Plane Execution Intents

- Status: proposed
- Date: 2026-02-19

## Context

Nexus ya tiene un pipeline fuerte de enforcement en `nexus-core`, pero hoy una ejecución sensible que requiere approval termina en un `202 Accepted` y queda modelada como un freno puntual, no como un workflow formal entre propuesta, autorización y ejecución.

Eso deja un hueco conceptual y operativo:

- no existe un objeto estable que represente la intención de ejecución
- approval y ejecución no quedan unidos por un recurso first-class
- la decisión humana no habilita un segundo paso explícito y gobernado
- Tower y SaaS no tienen una pieza mínima sobre la cual construir review, evidencia y control posterior

La nueva tesis de producto exige que Nexus trate acciones sensibles como intents gobernados y no como llamadas directas a tools con approvals ad hoc.

## Decision

Introducir `execution_intents` como recurso first-class del gateway para el wedge inicial del control plane.

La primera implementación adopta estas reglas:

- cuando una política requiere approval, `POST /v1/run` no ejecuta directo
- el gateway crea un `execution_intent` persistido con input, contexto, actor, scopes, policy y `risk_class`
- el approval queda vinculado al intent mediante `intent_id`
- aprobar o rechazar el approval actualiza el estado del intent
- la ejecución posterior ocurre mediante un paso explícito `POST /v1/run/intents/{id}/execute`
- el path directo de `POST /v1/run` sigue existiendo para casos que no requieren approval

Esta fase no introduce todavía preflight determinista generalizado ni execution leases efímeras completas. El objetivo es fijar el modelo y el workflow base sobre el que esas capacidades se montarán después.

## Consequences

- Nexus pasa de approvals sueltos a una cadena explícita `intent -> approval -> execute`
- la auditoría puede referenciar un `intent_id` estable entre propuesta y ejecución
- Tower y SaaS ganan un punto de integración mínimo para operar aprobaciones y ejecución posterior
- el runtime mantiene compatibilidad para ejecuciones no sensibles
- el diseño deja espacio para sumar después:
  - preflight artifacts
  - leases efímeras
  - break-glass
  - protected resources

## Alternatives Considered

- seguir usando solo `pending_approvals` sin `execution_intents`
- crear un workflow nuevo solamente en SaaS y dejar el core sin recurso propio
- bloquear todas las ejecuciones sensibles hasta implementar también preflights y leases
