# Nexus v3

Stack activo: **Review** (gobernanza), **Companion** (tareas + integración a Review), **console** (UI), **Postgres**.

## Arranque rápido

Desde este directorio (`v3/`):

```bash
cp -n .env.example .env   # si no tenés .env
docker compose up -d --build
```

Si Postgres ya tenía volumen **sin** la base `nexus_companion`:

```bash
bash scripts/dev/ensure-companion-db.sh
```

Luego reiniciá **companion** si hacía falta la base (`docker compose restart companion`).

## URLs por defecto (host)

| Servicio  | URL |
|-----------|-----|
| Review    | `http://localhost:18084` |
| Companion | `http://localhost:18085` |
| Console   | `http://localhost:13001` |

Variables: ver `.env.example`.

## Tests

```bash
make test          # Go unit (review + companion)
make smoke         # Requiere APIs levantadas (compose)
```

Smoke incluye flujo **Companion → Review** (`scripts/smoke/run-companion-review-flow.sh`): comprueba el vínculo `review_request_id`, que el **estado de la tarea** coincida con el resultado de Review (`allowed` → `done`, `pending_approval` → `waiting_for_approval`, etc.) y que responda `POST /v1/tasks/{id}/sync`.

## Documentación

- [companion/README.md](companion/README.md)
- [review/README.md](review/README.md)
- [doc/NEXUS_COMPLETION_ROADMAP.md](doc/NEXUS_COMPLETION_ROADMAP.md)
- [doc/NEXUS_ECOSYSTEM_DESIGN.md](doc/NEXUS_ECOSYSTEM_DESIGN.md)
