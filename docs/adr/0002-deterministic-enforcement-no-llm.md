# ADR 0002 - Deterministic Enforcement Without LLM

- Status: accepted
- Date: 2026-03-06

## Context

El gateway decide allow/deny y ejecuta controles de alto riesgo.

## Decision

No permitir LLM en el path crítico de enforcement.

## Consequences

- el runtime AI queda fuera de `/v1/run`
- mayor auditabilidad y menor riesgo operativo

## Alternatives Considered

- usar LLM para decisiones allow/deny
