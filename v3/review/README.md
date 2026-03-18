# Nexus Review

Servicio backend de [Nexus Review](../doc/RFC.md): engine de decisión, approval, registro y replay para requests de agentes, servicios y humanos.

## Correr con Docker (recomendado)

```bash
cd v3/
make up        # levanta review + postgres + console
make smoke     # pruebas smoke
make e2e       # pruebas end-to-end
```

Console en `http://localhost:13001`. API en `http://localhost:18084`.

## Correr localmente

```bash
make dev       # requiere postgres en :15434 (docker compose up review-postgres)
```

## API

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `/v1/requests` | POST | Enviar request |
| `/v1/requests` | GET | Listar requests |
| `/v1/requests/{id}` | GET | Consultar estado |
| `/v1/requests/{id}/result` | POST | Reportar resultado |
| `/v1/requests/{id}/replay` | GET | Replay completo |
| `/v1/policies` | CRUD | 7 operaciones (create, read, list, update, delete, archive, restore) |
| `/v1/approvals/pending` | GET | Inbox de aprobaciones |
| `/v1/approvals/{id}/approve` | POST | Aprobar |
| `/v1/approvals/{id}/reject` | POST | Rechazar |
| `/v1/learning/proposals` | GET | Propuestas de learning |
| `/v1/learning/proposals/{id}/accept` | POST | Aceptar propuesta |
| `/v1/learning/proposals/{id}/dismiss` | POST | Descartar propuesta |
| `/v1/learning/analyze` | POST | Trigger análisis de patrones |
| `/v1/metrics/summary` | GET | Dashboard métricas |
| `/healthz` | GET | Liveness |
| `/readyz` | GET | Readiness |

Auth: header `X-API-Key`.

## Estructura

```
review/
├── cmd/api/main.go          # entry point
├── internal/
│   ├── requests/            # módulo principal (CEL, risk, AI, audit sink)
│   ├── policies/            # CRUD políticas CEL
│   ├── approvals/           # inbox + approve/reject
│   ├── audit/               # trail + replay
│   ├── learning/            # pattern detection + proposals
│   ├── dashboard/           # métricas
│   └── shared/              # helpers compartidos
├── wire/setup.go            # DI manual
├── migrations/              # PostgreSQL (siempre)
├── Dockerfile
└── go.mod
```
