# Nexus v3 — Testing

## Niveles de testing

| Nivel | Qué prueba | Cómo correr |
|-------|-----------|-------------|
| Unit tests | Lógica de negocio, handlers, mappers | `make test` |
| Quality | Migraciones duplicadas + build + vet + test -race + console si está instalada | `make qa` |
| Smoke | Endpoints individuales contra API corriendo | `make smoke` |
| E2E | Flujo completo de punta a punta | `make e2e` |
| Acceptance | Smoke + E2E | `make acceptance` |

## Unit tests (Go)

Convenciones:
- **Table-driven** — `[]struct{ name, input, expected }`
- **`t.Parallel()`** en todos los tests que no compartan estado
- **`httptest.NewRequest` + `httptest.NewRecorder`** para handlers
- **Fakes inline** en `_test.go` — nunca archivos separados, nunca mocks sintéticos
- **Mismo package** — tests en el mismo package que el código (acceso a unexported)

Cobertura por módulo:

```bash
cd review/
go test -cover ./...
```

Qué cubrir por módulo:
- Happy path
- Not found (404)
- Validation error (400)
- Forbidden (403) — ej: action type desconocido, agente sin delegación
- Conflict (409) — ej: approval ya decidida
- Archive/Restore lifecycle (policies)

## Smoke tests

Scripts en `scripts/smoke/`. Requieren API corriendo (`make up` primero).

### run-policies-crud.sh

Prueba las 7 operaciones CRUD de policies:
1. Create → 201
2. Read → 200
3. List → 200 (≥1 resultado)
4. Update (PATCH) → 200
5. Archive → 204
6. Restore → 204
7. Delete (hard) → 204
8. Verify deleted → 404

### run-requests-flow.sh

Prueba el flujo completo de requests:
1. Create policy (require_approval)
2. Submit request (allow — sin match)
3. Submit request (require_approval — match)
4. List pending approvals
5. Approve
6. Report result
7. Replay (≥3 events)
8. Dashboard (≥2 requests)
9. Cleanup

## Migraciones

El historial de migraciones debe ser lineal por servicio. Antes de abrir PR o correr smoke:

```bash
make check-migrations
docker compose config --services
```

`make check-migrations` falla si `review/migrations` o `companion/migrations` tienen dos archivos `.up.sql` con el mismo prefijo numérico. En bases persistidas que hayan aplicado migraciones renumeradas, validar primero la versión registrada antes de correr `up`; si ya existe una versión aplicada con número viejo, usar una migración correctiva específica para ese ambiente en lugar de reescribir estado manualmente.

## E2E test

Script: `scripts/e2e/run-full-lifecycle.sh`

Prueba el ciclo completo:
1. Setup: crear policy
2. Submit → require_approval
3. Verify en pending approvals
4. Verify request status = pending_approval
5. Approve
6. Verify status = approved
7. Report result
8. Verify status = executed
9. Replay (≥4 events, final_status = executed)
10. Dashboard (≥1 request)
11. Learning analyze (0 proposals — pocas muestras)
12. Idempotency (misma key → mismo ID)
13. Cleanup

## Helpers compartidos

`scripts/lib/common.sh` provee:
- `wait_for_http` — espera que endpoint responda 200
- `api_get` / `api_post` / `api_delete` — con API key
- `json_get` — extrae campos JSON (soporta nested keys y `len()`)
- `assert_status` — verifica HTTP status code
- `pass` / `fail` — output coloreado
