# Service Boundaries

This document defines ownership between `nexus-core` and `nexus-saas`.

## Ownership

- `nexus-core` (data-plane):
  - run/simulate execution pipeline
  - policy evaluation and approvals
  - DLP, egress, idempotency, audit
  - tool/policy/secret operational APIs
- `nexus-saas` (business-plane):
  - tenant plans and hard limits
  - usage metering aggregation
  - SaaS admin activity records
  - org onboarding and SaaS-facing identity/SSO endpoints

## Rules

- `nexus-core` must not own SaaS billing/plan state.
- `nexus-saas` must not expose core operational domains (`run`, `tools`, `audit`, `gateway` workflows).
- Cross-service communication must happen over internal HTTP contracts.
- Databases are separate: `core_db` and `saas_db`.

## Internal Contracts

- Core -> SaaS:
  - `POST /internal/usage/events`
  - `GET /internal/entitlements/:org_id`

