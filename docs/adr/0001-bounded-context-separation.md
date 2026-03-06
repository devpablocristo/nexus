# ADR 0001 - Bounded Context Separation

- Status: accepted
- Date: 2026-03-06

## Context

Nexus mezcla data plane, business plane, operators y UI en un monorepo.

## Decision

Separar ownership por servicio: `nexus-core`, `nexus-saas`, `nexus-control-operators`, `nexus-ai-operators`, `nexus-tower`.

## Consequences

- menos acoplamiento accidental
- contracts y docs obligatorios entre contextos

## Alternatives Considered

- monolito único
- sharing de ownership “por conveniencia”
