# Shared Contracts and Clients

This directory centralizes cross-project contracts for the monorepo:

- `contracts/`: event schema, error code catalog, OpenAPI snapshot.
- `clients/ts/`: Nexus Core TypeScript API client.
- `clients/python/`: Nexus Core Python API client.
- `shared-types/`: portable TypeScript domain types.

All service-to-service integrations (`nexus-operator`, `nexus-tower`) must use Nexus Core APIs, never direct DB access.
