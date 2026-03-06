# ADR 0003 - Core vs SaaS Dual Database

- Status: accepted
- Date: 2026-03-06

## Context

El data plane y el business plane tienen ritmos y ownership distintos.

## Decision

Mantener PostgreSQL separados: `nexus` y `nexus_saas`.

## Consequences

- límites claros de escritura
- coordinación por APIs/contratos internos

## Alternatives Considered

- una sola base compartida
