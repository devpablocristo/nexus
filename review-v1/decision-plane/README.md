# Nexus Review v1 — PoC

PoC del [RFC](doc/RFC.md): capa de review, approval, registro y replay para requests (agentes, servicios, humanos). Tres pilares: **Decidir**, **Registrar**, **Aprender**.

## Cómo correr

```bash
# API key por defecto (desarrollo): nxr_dev=dev-key-change-me
export NEXUS_REVIEW_API_KEYS="nxr_dev=dev-key-change-me"
go run ./cmd/api
```

Servidor en `:8080`. Health: `GET /healthz`, `GET /readyz`.

## API (PoC)

- **Requests:** `POST /v1/requests` (header `Idempotency-Key` opcional), `GET /v1/requests/{id}`, `POST /v1/requests/{id}/result`
- **Replay:** `GET /v1/requests/{id}/replay`
- **Policies:** `POST /v1/policies`, `GET /v1/policies`, `GET /v1/policies/{id}`, `PATCH /v1/policies/{id}`
- **Approvals:** `GET /v1/approvals/pending`, `POST /v1/approvals/{id}/approve`, `POST /v1/approvals/{id}/reject`
- **Learning:** `GET /v1/learning/proposals`, `GET /v1/learning/proposals/{id}`, `POST .../accept`, `POST .../dismiss`
- **Dashboard:** `GET /v1/metrics/summary?period=7d`

Autenticación: header `X-API-Key` (SHA256 del valor contra las keys configuradas).

## Estructura

- `cmd/api` — entrada HTTP
- `internal/requests` — módulo principal (CEL, risk, idempotency, AI contextualizer stub)
- `internal/policies` — CRUD políticas CEL
- `internal/approvals` — inbox y approve/reject
- `internal/audit` — request_events y replay
- `internal/learning` — policy proposals (stub)
- `internal/dashboard` — métricas (stub)
- `wire` — DI
- `migrations` — schema PostgreSQL (para uso futuro; PoC usa in-memory)

## Referencias

- v1 y v2 de Nexus en el mismo repo (referencia de patrones y pkgs).
- [doc/RFC.md](doc/RFC.md) — alcance y diseño del PoC.
- [doc/FEATURE_RANKING.md](doc/FEATURE_RANKING.md) — priorización de features.
