# nexus-control-operators

Deterministic control-plane service for Nexus.

This service runs the internal deterministic workers:
- `sentry`
- `coordinator`
- `mitigation`
- `recovery`

It operates asynchronously over operational events and is not part of the synchronous `/v1/run` request path.

## Image

Built with `nexus-control-operators/Dockerfile` (from repo root: `docker compose build nexus-control-operators`).
