# ADR 0004 - Operators Do Not Write Directly To Databases

- Status: accepted
- Date: 2026-03-06

## Context

Los operators ejecutan automatización cross-service.

## Decision

`nexus-control-operators` y `nexus-ai-operators` actúan solo por APIs internas.

## Consequences

- menos acoplamiento a storage
- trazabilidad completa por contratos HTTP/eventos

## Alternatives Considered

- acceso directo a tablas de incidentes/acciones/eventos
