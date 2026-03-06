# ADR 0007 - Terraform And AWS Deployment Model

- Status: accepted
- Date: 2026-03-06

## Context

Nexus requiere infraestructura reproducible y multi-servicio.

## Decision

Usar Terraform sobre AWS como modelo productivo.

## Consequences

- infraestructura declarativa
- módulos reutilizables para networking, ECS, RDS, Redis, secrets y monitoring

## Alternatives Considered

- provisión manual
- IaC no declarativa
