# ADR 0005 - Clerk For Identity

- Status: accepted
- Date: 2026-03-06

## Context

Nexus necesita auth de usuarios, JWT/JWKS y org membership.

## Decision

Usar Clerk para identidad de usuario y orgs, integrando JWT/JWKS en backend.

## Consequences

- frontend simplificado
- sync de users/memberships vía webhooks

## Alternatives Considered

- auth casera
- proveedor distinto sin soporte org-first equivalente
