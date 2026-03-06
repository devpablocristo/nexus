# ADR 0006 - Stripe For Billing

- Status: accepted
- Date: 2026-03-06

## Context

La capa SaaS requiere checkout, portal, webhooks y dunning.

## Decision

Usar Stripe como sistema de billing y suscripción.

## Consequences

- estado de billing sincronizado en `tenant_settings`
- checkout/portal/webhooks como contratos obligatorios

## Alternatives Considered

- billing manual
- PSP alternativo no integrado
