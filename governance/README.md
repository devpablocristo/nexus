# 1. Comandos ejecutados

| Comando | Resultado resumido | Falló / OK | Alternativa usada | Impacto en confianza |
|---|---|---|---|---|
| `pwd` | Repo en `/home/pablocristo/Proyectos/pablo/nexus` | OK | No aplica | Alta |
| `git status --short` | Hay 4 archivos modificados en `console/src/*`; no los toqué durante esta auditoría | OK | No aplica | Alta |
| `find . -maxdepth 3 -type f \| sed ... \| head -300` | Inventario inicial; incluyó `.git`, `.env`, `console/dist`, `console/node_modules` | OK | Luego usé búsquedas con exclusiones permitidas | Media |
| `find . -maxdepth 4 -type d \| sed ... \| head -300` | Inventario de directorios; también ruidoso por `.git` y `node_modules` | OK | Luego usé `rg --files` excluyendo dependencias | Media |
| `rg --files -g '!node_modules' -g '!vendor' -g '!dist' -g '!build' -g '!coverage' -g '!__pycache__' -g '!.git'` | 190 archivos relevantes | OK | No aplica | Alta |
| `find . -maxdepth 5 ... go.mod/package.json/Dockerfile/*.sql/*.yml/.env*` | Detectó archivos clave, pero con ruido de `node_modules` | OK | Repetido excluyendo `.git`, `node_modules`, `dist` | Alta |
| `rg -n "TODO|FIXME|...|legacy|..." .` | Deuda explícita: legacy en auth/migraciones; no TODO crítico de runtime | OK | No aplica | Media |
| `rg -n "CREATE TABLE|CREATE INDEX|ALTER TABLE|..." .` | Migraciones y constraints localizadas | OK | Lectura dirigida de migraciones | Alta |
| `rg -n "SELECT |INSERT |UPDATE |DELETE |JOIN |..." .` | Queries principales en repos Go | OK | Lectura dirigida de repositorios | Alta |
| `rg -n "useEffect|fetch|Route|..." .` | Frontend usa Vite/React sin router; API centralizada en `console/src/api.ts` | OK | Lectura de `App.tsx`, `api.ts`, views | Alta |
| `rg -n "func main|http\.|pgx|Begin|Commit|..." .` | Backend Go localizado; salida larga/truncada | OK | Lecturas dirigidas por módulo | Media-Alta |
| `rg -n "FastAPI|Flask|requests|psycopg|..." .` | No hay app Python; Python aparece en scripts Bash con `python3 -c` | OK | `find ... '*.py'/pyproject/requirements` sin resultados | Alta |
| `rg -n "go test|npm run test|pytest|tsc|..." .` | Tests/CI documentados en `Makefile`, workflows y scripts | OK | Lectura de workflows/scripts | Alta |
| `bash scripts/quality/check-migrations.sh` | `Migration version checks passed.` | OK | No aplica | Alta |
| `docker compose -f docker-compose.yml config --services` | Servicios base: `governance-postgres`, `governance`, `console` | OK | No aplica | Alta |
| `docker compose -f docker-compose.yml -f docker-compose.dev.yml config --services` | Agrega servicio `nexus`; no falla en config | OK | Revisé config renderizado y `test -d nexus` | Alta |
| `test -d nexus` | `nexus_dir_exists=1`, o sea no existe | OK | No aplica | Alta |
| `docker compose -f docker-compose.yml -f docker-compose.ponti.yml config --services` | Falla: `ponti-backend depends on undefined service "nexus"` | Falló | Lectura de `docker-compose.ponti.yml` | Alta |
| `nl -ba scripts/smoke-governance.sh` | No existe | Falló | `scripts/smoke/run-policies-crud.sh`, `run-requests-flow.sh` | Alta |
| `nl -ba scripts/e2e-governance.sh` | No existe | Falló | `scripts/e2e/run-full-lifecycle.sh` | Alta |
| `nl -ba governance/internal/server/server.go` | No existe | Falló | `governance/cmd/api/main.go`, `governance/wire/setup.go` | Alta |
| `nl -ba governance/wire/action_type_checker_adapter.go` | Nombre incorrecto | Falló | `governance/wire/actiontype_checker_adapter.go` | Alta |
| `nl -ba governance/internal/requests/usecases_test.go` | No existe | Falló | `governance/internal/requests/handler_test.go` | Media |

# 2. Cobertura real del análisis

- Carpetas revisadas: `governance/`, `console/`, `scripts/`, `.github/`, `doc/`, raíz del repo.
- Archivos clave revisados: `governance/cmd/api/main.go`, `governance/wire/setup.go`, `governance/wire/auth.go`, handlers/repos/usecases de requests, approvals, delegations, policies, action types, evidence, migraciones `0001`, `0003`, `0007`, `0008`, `0010`, `0011`, `0013`, `0014`, `0015`, `console/src/api.ts`, `App.tsx`, `AuthTokenBridge.tsx`, `vite.config.js`, `docker-compose*.yml`, `Makefile`, workflows CI/E2E/deploy.
- Áreas excluidas: `.git`, `console/node_modules`, `console/dist`, caches/dependencias. Son exclusiones permitidas.
- Áreas parcialmente revisadas: módulos `learning`, `rbac`, `config`, `dashboard` solo a nivel wiring/queries/handlers principales.
- Nivel de confianza: Medio-Alto para diagnóstico estático; Bajo para performance real.
- Qué impide aumentar confianza: no ejecuté `go test`, `npm run build`, smoke/e2e con servicios vivos, `EXPLAIN ANALYZE`, logs, métricas ni entorno GCP.
- La cobertura sí alcanza para diagnosticar riesgos estáticos concretos, no para certificar comportamiento runtime completo.

# 3. Mapa del repositorio

- Estructura general: `README.md:3-18` define Nexus como `governance` Go + `console` React + Postgres local; Companion vive en otro repo y consume Governance por HTTP.
- Backend Go: `governance/`, módulo `github.com/devpablocristo/nexus/governance`, Go `1.26.1`, `cmd/api/main.go` como entrypoint.
- Wiring: `governance/wire/setup.go:40-197` abre DB, corre migraciones, crea repos/usecases/handlers y registra auth middleware.
- Frontend React: `console/`, Vite + React 19, scripts `dev`, `build`, `typecheck` en `console/package.json:9-13`.
- PostgreSQL: migraciones embebidas en `governance/migrations`, aplicadas por `sharedpostgres.MigrateUp` en `wire/setup.go:49-54`.
- Infra: `docker-compose.yml` levanta Postgres, governance y console; `Makefile` define `test`, `qa`, `smoke`, `e2e`, `dev`, Ponti local.
- CI/CD: `.github/workflows/ci.yml` corre Go test/vet, migraciones, console typecheck/build y smoke; `.github/workflows/e2e.yml` corre lifecycle programado/manual; deploy Cloud Run en `deploy-governance-dev.yml`.
- Python: no encontré módulos Python propios; se usa `python3 -c` en scripts de smoke/e2e para parsear JSON.

# 4. Comprensión del sistema

- Propósito funcional: un plano de decisión/auditoría para requests de agentes, servicios o humanos, con policies, approvals, replay y evidence. Evidencia: `README.md:3-18`, `governance/README.md:7-10`.
- Arquitectura general: Go modular por dominio con handlers/usecases/repositories, DI manual en `wire/setup.go:56-185`.
- Backend Go: `main.go:29-58` exige `DATABASE_URL` y `GOVERNANCE_API_KEYS`; `main.go:67-80` limita body a 1MB, agrega métricas y security middleware.
- Auth: `wire/auth.go:127-160` borra headers de identidad enviados por cliente y setea headers confiables desde principal/API key/JWT.
- Flujo principal: `requests.Handler.submit` valida entrada y org en `handler.go:138-181`; `Usecases.Submit` valida action type, delegación, evalúa policies y decide en `usecases.go:230-368`.
- Policies: `policies.Repository.List` filtra org + globales cuando recibe `OrgID` en `repository.go:99-118`.
- Action types: `actiontypes.Repository.GetByNameForOrg` prefiere org específica y cae a global en `repository.go:78-85`.
- Delegations: el CRUD setea/valida `OrgID` en `delegations/handler.go:63-65` y `:90-97`, pero el enforcement no pasa org.
- Approvals: las decisiones approve/reject sí tienen transacción y `SELECT FOR UPDATE` en `approvals/decisions_tx.go:17-22`, `:86-99`.
- Evidence: evidence pack se firma con HMAC en `evidence/signer.go:14-52`; puede incluir attestation si existe en `evidence/usecases.go:104-112`.
- Frontend: `console/src/api.ts:38-49` centraliza requests; `App.tsx:18-35` define navegación local por tabs; `AuthTokenBridge.tsx:19-33` registra token Clerk.
- Zonas de riesgo: multi-tenant delegations, creación de pending approval no transaccional, attestations sin verifier inyectado, filtros org aplicados después de límites, overlays Docker stale.

# 5. Evidence ledger

| Tema | Evidencia concreta | Archivo/path | Símbolo/función/componente/query | Qué demuestra | Confianza |
|---|---|---|---|---|---|
| Producto | Nexus = governance + console + Postgres | `README.md:3-18` | README | Alcance funcional | Alta |
| Entrypoint Go | Lee env vars y crea server | `governance/cmd/api/main.go:19-60` | `main` | Startup real | Alta |
| Wiring | Registra repos/usecases/handlers | `governance/wire/setup.go:56-185` | `NewServer` | Dependencias runtime | Alta |
| Auth confiable | Borra y reinyecta headers identidad | `governance/wire/auth.go:127-160` | `withIdentityHeaders` | No confía en headers cliente | Alta |
| Request flow | Submit valida y llama usecase | `requests/handler.go:138-181` | `submit` | Contrato HTTP principal | Alta |
| Policy org-aware | Lista org + globales | `policies/repository.go:104-118` | `List` | Tenant-aware en policies | Alta |
| Action type org-aware | Busca org específica o global | `actiontypes/repository.go:78-85` | `GetByNameForOrg` | Tenant-aware en action types | Alta |
| Delegation org en CRUD | Create setea `OrgID` | `delegations/handler.go:63-65` | `create` | El modelo espera org | Alta |
| Delegation enforcement sin org | Check usa solo agent/action | `requests/usecases.go:281-288`, `wire/delegation_checker_adapter.go:19-21` | `CheckDelegation` | Riesgo cross-org | Alta |
| Repo delegation sin org | Query filtra `agent_id` pero no `org_id` | `delegations/repository.go:75-81` | `ListByAgentID` | Causa técnica del riesgo | Alta |
| Approval decision transaccional | `SELECT FOR UPDATE`, persist final approval+request | `approvals/decisions_tx.go:17-22`, `:86-99` | `DecisionApplier` | Buen patrón existente | Alta |
| Pending approval no transaccional | Request create, approval create, request update separados | `requests/usecases.go:418-456` | `handleRequireApproval` | Inconsistencia posible | Alta |
| Attestation sin verifier | Option existe, wire no la usa | `requests/usecases.go:109-114`, `wire/setup.go:107-122` | `AttestationVerifier` | Claim no verificado | Alta |
| Evidence firmado | HMAC-SHA256 | `evidence/signer.go:31-52` | `SignPack` | Evidence pack sí firmado | Alta |
| Request list post-filtrado | DB limita global, handler filtra org después | `requests/repository.go:136-154`, `requests/handler.go:229-240` | `List` | Resultado incompleto por tenant | Alta |
| Approval list post-filtrado | DB limita 50 global, handler filtra org después | `approvals/repository.go:95-100`, `approvals/handler.go:39-55` | `ListPending` | Inbox incompleto por tenant | Alta |
| Ponti compose roto | Depends on undefined service | `docker-compose.ponti.yml:30-63` | `ponti-backend` | `docker compose config` falla | Alta |
| Dev compose stale | `context: ./nexus`, directorio ausente | `docker-compose.dev.yml:2-9` | service `nexus` | `make dev` probable falla build | Alta |
| Tests disponibles | Go tests y smoke/e2e scripts | `Makefile:12-38`, `ci.yml:48-116` | `make test`, `make smoke` | Validación existente | Alta |
| Python ausente | `find '*.py'/pyproject/requirements` sin resultados | comando ejecutado | Inventario | No hay app Python en este repo | Alta |

# 6. Diagnóstico priorizado

### Hallazgo 1: La validación de delegaciones ignora `org_id`

Severidad: Alta  
Prioridad: P0  
Estado: Confirmado  
Área: Backend Go / Seguridad multi-tenant

Archivos afectados:
- [requests/usecases.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/requests/usecases.go:281)
- [delegation_checker_adapter.go](/home/pablocristo/Proyectos/pablo/nexus/governance/wire/delegation_checker_adapter.go:19)
- [delegations/repository.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/delegations/repository.go:75)

Evidencia:
- `requests.Submit` llama `CheckDelegation(ctx, req.RequesterID, req.ActionType)` sin `req.OrgID`.
- `delegations.Repository.ListByAgentID` filtra por `agent_id`, `enabled`, `expires_at`, pero no por `org_id`.
- El CRUD de delegations sí setea y protege `OrgID`.

Qué comprobé:
- Policies y action types sí tienen resolución org-aware.
- Delegations tienen `OrgID *string`, pero enforcement no lo usa.

Por qué es un problema real:
- Un mismo `agent_id` con delegación válida en una org puede autorizar una request con el mismo action type en otra org.

Impacto:
- Bypass de aislamiento tenant en autorización de acciones.

Riesgo de cambiarlo:
- Medio: hay compatibilidad explícita “sin delegaciones = sin restricciones”; hay que preservar semántica legacy/global.

Recomendación mínima suficiente:
- Pasar `orgID` al port `DelegationChecker`, filtrar delegaciones por `org_id = request org` + globales si aplica, y agregar tests cross-org.

Qué NO conviene hacer:
- Rediseñar todo RBAC/delegations o cambiar el modelo de scopes.

Validación sugerida:
- Test unitario: delegación `org-a` no autoriza request `org-b`.
- `cd governance && go test ./internal/delegations ./internal/requests ./...`.

### Hallazgo 2: Crear request `pending_approval` y approval no es atómico

Severidad: Alta  
Prioridad: P1  
Estado: Confirmado  
Área: Backend Go / Integridad de datos

Archivos afectados:
- [requests/usecases.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/requests/usecases.go:418)
- [approvals/decisions_tx.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/approvals/decisions_tx.go:17)

Evidencia:
- `handleRequireApproval` crea request, luego approval, luego actualiza `approval_id`.
- Si `approvalRepo.Create` falla, queda request `pending_approval` sin approval.
- Si `reqRepo.Update` falla, solo loguea error.
- El propio módulo approvals ya tiene patrón transaccional para approval+request.

Qué comprobé:
- Las decisiones approve/reject sí se resolvieron con transacción y row lock.
- La creación inicial de pending approval no usa patrón equivalente.

Por qué es un problema real:
- Puede dejar requests no accionables o respuestas inconsistentes ante fallo parcial de DB.

Impacto:
- Inbox incompleto, requests colgadas, retries/idempotencia más frágiles.

Riesgo de cambiarlo:
- Medio: toca persistencia cruzada requests/approvals.

Recomendación mínima suficiente:
- Crear un método transaccional específico para request+approval+approval_id, reutilizando el patrón existente de tx.

Qué NO conviene hacer:
- Reescribir repositorios completos o introducir una capa genérica de Unit of Work.

Validación sugerida:
- Test con fallo simulado en create approval/update request.
- Smoke `run-requests-flow.sh`.

### Hallazgo 3: Attestation se persiste sin verificación criptográfica en runtime

Severidad: Alta  
Prioridad: P1  
Estado: Confirmado  
Área: Backend Go / Evidence / Seguridad

Archivos afectados:
- [requests/usecases.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/requests/usecases.go:109)
- [wire/setup.go](/home/pablocristo/Proyectos/pablo/nexus/governance/wire/setup.go:107)
- [requests/handler_test.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/requests/handler_test.go:1438)

Evidencia:
- `AttestationVerifier` existe, pero `wire.NewServer` solo inyecta `WithAttestationStore`.
- Comentario dice que sin verifier queda como claim sin garantía de integridad.
- Test acepta `"signature":"sig-hash-xyz"` sin verifier.

Qué comprobé:
- El evidence pack sí se firma con HMAC, pero eso firma el pack generado por Governance; no verifica que la attestation original sea auténtica.

Por qué es un problema real:
- La API puede almacenar y luego incluir en evidence una “prueba verificable” que no fue verificada.

Impacto:
- Riesgo de compliance/auditoría falsa si consumidores confían en `attestation.signature`.

Riesgo de cambiarlo:
- Medio-Alto: falta definir contrato exacto de firma externa: JWS, HMAC, hash canónico, key lookup.

Recomendación mínima suficiente:
- Definir contrato mínimo de firma y hacer que producción inyecte verifier o falle cerrado cuando se exija verificación.

Qué NO conviene hacer:
- Inventar un formato de firma sin acordarlo con el ejecutor/consumer.

Validación sugerida:
- Test de firma inválida rechazada.
- Test de startup/config sin verifier cuando verification requerida.

### Hallazgo 4: Listados tenant-aware filtran después de aplicar límites globales

Severidad: Media  
Prioridad: P1  
Estado: Confirmado  
Área: Backend Go / Multi-tenant / UX operacional

Archivos afectados:
- [requests/repository.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/requests/repository.go:136)
- [requests/handler.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/requests/handler.go:229)
- [approvals/repository.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/approvals/repository.go:95)
- [approvals/handler.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/approvals/handler.go:39)

Evidencia:
- Requests aplica `LIMIT` en SQL sin org, luego handler filtra por `canAccessRequestOrg`.
- Approvals lista 50 pending globales y luego filtra por org.

Qué comprobé:
- Policies y action types sí filtran por org a nivel query; requests/approvals no.

Por qué es un problema real:
- Una org puede recibir lista vacía o incompleta si los primeros N registros globales pertenecen a otras orgs.

Impacto:
- Console/inbox puede ocultar datos válidos sin error.

Riesgo de cambiarlo:
- Medio: cambiar ports y queries, pero comportamiento externo se preserva.

Recomendación mínima suficiente:
- Agregar filtros org-aware antes del `LIMIT` en repos/usecases para requests y approvals.

Qué NO conviene hacer:
- Agregar paginación compleja ahora si el bug se resuelve moviendo el filtro a SQL.

Validación sugerida:
- Test con datos `org-a`/`org-b` donde `org-a` tenga registros fuera del top global.

### Hallazgo 5: `docker-compose.ponti.yml` no compone

Severidad: Media  
Prioridad: P1  
Estado: Confirmado  
Área: Infra / Integración Ponti

Archivos afectados:
- [docker-compose.ponti.yml](/home/pablocristo/Proyectos/pablo/nexus/docker-compose.ponti.yml:30)
- [Makefile](/home/pablocristo/Proyectos/pablo/nexus/Makefile:56)

Evidencia:
- `docker compose -f docker-compose.yml -f docker-compose.ponti.yml config --services` falla.
- `ponti-backend` depende de `nexus`, pero el servicio base actual se llama `governance`.

Qué comprobé:
- Base compose define `governance`, no `nexus`.
- El comentario aún dice “nexus es el servicio de governance”.

Por qué es un problema real:
- `make up-ponti-local` no puede levantar el stack.

Impacto:
- Integración local Companion/Ponti bloqueada.

Riesgo de cambiarlo:
- Bajo-Medio: depende de rutas externas Ponti no presentes en este repo.

Recomendación mínima suficiente:
- Cambiar dependencia y URL interna a `governance`, y alinear env vars `GOVERNANCE_PONTI_API_KEY`.

Qué NO conviene hacer:
- Reorganizar los stacks o meter Ponti dentro de este repo.

Validación sugerida:
- `docker compose -f docker-compose.yml -f docker-compose.ponti.yml config --services`.

### Hallazgo 6: `docker-compose.dev.yml` apunta a un backend inexistente

Severidad: Media  
Prioridad: P1  
Estado: Confirmado  
Área: Infra / Desarrollo local

Archivos afectados:
- [docker-compose.dev.yml](/home/pablocristo/Proyectos/pablo/nexus/docker-compose.dev.yml:2)
- [console/vite.config.js](/home/pablocristo/Proyectos/pablo/nexus/console/vite.config.js:4)
- [Makefile](/home/pablocristo/Proyectos/pablo/nexus/Makefile:43)

Evidencia:
- Overlay agrega servicio `nexus` con `build.context: ./nexus`.
- `test -d nexus` devolvió no existe.
- Console dev usa `NEXUS_PROXY_TARGET`, pero Vite espera `GOVERNANCE_PROXY_TARGET`.

Qué comprobé:
- `docker compose config` no falla porque puede renderizar el path, pero build/up va contra directorio ausente o creado vacío.

Por qué es un problema real:
- `make dev` está documentado y probablemente falla en hot reload.

Impacto:
- Dev loop roto o confuso.

Riesgo de cambiarlo:
- Bajo.

Recomendación mínima suficiente:
- Hacer que el overlay dev extienda `governance` real o eliminar el servicio `nexus` stale; alinear env vars a `GOVERNANCE_*`.

Qué NO conviene hacer:
- Cambiar puertos o flujo base de `make up`.

Validación sugerida:
- `docker compose -f docker-compose.yml -f docker-compose.dev.yml config`.
- Luego, con aprobación, `make dev`.

### Hallazgo 7: Reportar resultado guarda idempotency report antes de actualizar request

Severidad: Media  
Prioridad: P2  
Estado: Confirmado  
Área: Backend Go / Integridad parcial

Archivos afectados:
- [requests/usecases.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/requests/usecases.go:1224)
- [requests/repository.go](/home/pablocristo/Proyectos/pablo/nexus/governance/internal/requests/repository.go:265)

Evidencia:
- `resultReports.Save` ocurre antes de `reqRepo.Update`.
- Si update falla, queda report guardado pero request no cambia a executed/failed.

Qué comprobé:
- Hay idempotencia por `request_id,result_key`, pero no transacción request+report.

Por qué es un problema real:
- Puede dejar divergencia entre reporte de ejecución y status observable.

Impacto:
- Replay/dashboard/status inconsistentes en fallos parciales.

Riesgo de cambiarlo:
- Medio.

Recomendación mínima suficiente:
- Diferir hasta resolver H2; usar patrón transaccional similar si se ve en producción.

Qué NO conviene hacer:
- Cambiar semántica pública de `/result`.

Validación sugerida:
- Test de fallo simulado en update request después de guardar report.

### Hallazgo 8: Nombres legacy `NEXUS_*` siguen en docs/mensajes

Severidad: Baja  
Prioridad: P3  
Estado: Confirmado  
Área: Documentación / DX

Archivos afectados:
- [doc/DEPLOYMENT.md](/home/pablocristo/Proyectos/pablo/nexus/doc/DEPLOYMENT.md:44)
- [governance/wire/setup.go](/home/pablocristo/Proyectos/pablo/nexus/governance/wire/setup.go:157)
- [governance/README.md](/home/pablocristo/Proyectos/pablo/nexus/governance/README.md:49)

Evidencia:
- `.env.example` usa `GOVERNANCE_*`.
- Docs aún muestran `NEXUS_PORT`, `NEXUS_API_KEYS`, etc.
- Error de startup dice `NEXUS_SIGNING_KEY is required`.

Qué comprobé:
- Runtime principal usa `GOVERNANCE_SIGNING_KEY` y `GOVERNANCE_API_KEYS`.

Por qué es un problema real:
- Puede inducir mala configuración manual.

Impacto:
- DX y soporte, no bug funcional directo.

Riesgo de cambiarlo:
- Bajo.

Recomendación mínima suficiente:
- Corregir docs/mensaje de error, sin tocar contratos runtime.

Qué NO conviene hacer:
- Cambiar env vars runtime por compatibilidad estética.

Validación sugerida:
- `rg -n "NEXUS_" README.md doc governance console docker-compose*.yml`.

# 7. Cambios que sí conviene hacer ahora

- Hallazgo 1: hacer delegations tenant-aware.
  - Archivos a tocar: `requests/usecases.go`, `wire/delegation_checker_adapter.go`, `delegations/usecases.go`, `delegations/repository.go`, tests.
  - Beneficio: elimina bypass cross-org.
  - Riesgo: compatibilidad legacy/global.
  - Cambio mínimo: agregar `orgID *string` al check y filtrar en query.
  - Preservar: “sin delegaciones = sin restricciones” si esa semántica sigue vigente.
  - Validación: tests cross-org + `go test ./...`.

- Hallazgo 2: crear pending approval transaccionalmente.
  - Archivos a tocar: `requests/usecases.go`, repos/adapter transaccional nuevo o método específico.
  - Beneficio: evita requests colgadas.
  - Riesgo: medio por persistencia cruzada.
  - Cambio mínimo: tx local para request+approval+approval_id.
  - Preservar: respuesta HTTP y DTO actual.
  - Validación: test fallo parcial + smoke.

- Hallazgo 3: resolver verification de attestation.
  - Archivos a tocar: `requests/usecases.go`, `wire/setup.go`, config/tests.
  - Beneficio: evidence no incluye claims no verificados como si fueran prueba.
  - Riesgo: falta contrato criptográfico externo.
  - Cambio mínimo: definir verifier y fallar cerrado cuando verification se configure como requerida.
  - Preservar: evidence pack signing actual.
  - Validación: firma válida/ inválida + startup config.

- Hallazgo 4: aplicar filtros org antes del límite.
  - Archivos a tocar: repos/usecases/handlers de requests y approvals.
  - Beneficio: listas completas por tenant.
  - Riesgo: bajo-medio.
  - Cambio mínimo: parámetros `orgID/crossOrg` en query.
  - Preservar: límites y filtros públicos `status`, `action_type`.
  - Validación: tests multi-org.

- Hallazgos 5 y 6: alinear compose dev/Ponti.
  - Archivos a tocar: `docker-compose.dev.yml`, `docker-compose.ponti.yml`.
  - Beneficio: `make dev` y `make up-ponti-local` dejan de apuntar a nombres viejos.
  - Riesgo: bajo para dev; medio para Ponti por repo externo.
  - Cambio mínimo: `nexus` -> `governance`, `NEXUS_*` -> `GOVERNANCE_*` donde aplique.
  - Validación: `docker compose ... config --services`.

# 8. Cambios que NO conviene hacer ahora

- Refactors cosméticos: no hay evidencia de beneficio proporcional.
- Renombres masivos de módulos/carpetas: alto riesgo y bajo valor inmediato.
- Reorganización de carpetas: el wiring actual es entendible y CI lo valida.
- Abstracciones nuevas no justificadas: solo agregar tx/helper donde hay bug demostrado.
- Eliminación de código no confirmado muerto: no encontré código muerto confirmado; no eliminar.
- Optimizaciones SQL sin `EXPLAIN` o frecuencia real: no marcar índices nuevos salvo con evidencia runtime.
- Cambios de API pública: los consumidores usan `/v1/*`; preservar rutas y DTOs.
- Cambios de schema amplios: solo migraciones puntuales si una corrección las exige.
- Cambios con riesgo mayor que beneficio: especialmente attestation sin acordar contrato de firma.

# 9. Plan mínimo de implementación

1. Escribir tests primero para H1 y H4:
   - Cross-org delegation.
   - Request/approval list con registros de varias orgs y límite bajo.
   - Rollback: eliminar tests si revelan supuesto de negocio incorrecto.

2. Implementar H1:
   - Pasar `orgID` desde `requests.Submit`.
   - Agregar repo query org-aware.
   - Preservar global/legacy.
   - Validar con `go test ./internal/delegations ./internal/requests`.

3. Implementar H4:
   - Mover filtros org a SQL antes de `LIMIT`.
   - Mantener soporte cross-org.
   - Validar handlers y smoke.

4. Implementar H2:
   - Método transaccional específico para pending approval.
   - Test de fallo parcial.
   - Validar flujo request→approval.

5. Resolver H5/H6:
   - Arreglar compose overlays.
   - Validar solo `docker compose config` primero.
   - Recién después probar `make dev`/Ponti si el entorno externo existe.

6. H3:
   - Primero acordar contrato de firma.
   - Luego inyectar verifier o flag de verificación requerida.
   - Tests de firma inválida.

# 10. Validaciones recomendadas

- Go:
  - `cd governance && go test ./...`
  - `cd governance && go vet ./...`
  - `cd governance && go test ./... -race`

- React:
  - `cd console && npm ci`
  - `cd console && npm run typecheck`
  - `cd console && npm run build`

- Python:
  - No hay app Python propia.
  - Para scripts: `find scripts -name '*.sh' -print0 | xargs -0 -n1 bash -n`

- PostgreSQL/migraciones:
  - `make check-migrations`
  - `docker compose up -d --build governance-postgres governance`
  - `make smoke`
  - `make e2e`

- Docker/infra:
  - `docker compose -f docker-compose.yml config --services`
  - `docker compose -f docker-compose.yml -f docker-compose.dev.yml config --services`
  - `docker compose -f docker-compose.yml -f docker-compose.ponti.yml config --services`

- CI/CD:
  - GitHub Actions: `CI`, `E2E`, `Deploy Governance DEV`.
  - Para deploy real faltan secretos/vars GCP; no lo ejecuté.

# 11. Riesgos remanentes y preguntas abiertas

- Runtime: no confirmé latencias, errores reales ni comportamiento con DB viva.
- Logs: hacen falta logs de governance y smoke/e2e ante tráfico real.
- Métricas: falta saber volumen por org y distribución de requests/approvals.
- `EXPLAIN ANALYZE`: necesario antes de proponer índices nuevos.
- Credenciales/entorno: deploy Cloud Run depende de GCP secrets, WIF, Cloud SQL e IAM.
- Negocio: confirmar si “sin delegaciones = sin restricciones” debe seguir vigente.
- Attestation: confirmar formato real de firma y quién emite/verifica.
- Ponti/Companion: los repos externos no fueron auditados; solo revisé el compose de integración.
- Worktree: había cambios previos en `console/src/components/RiskBadge.tsx`, `StatusBadge.tsx`, `i18n.ts`, `views/Requests.tsx`; no los modifiqué.

# 12. Decisión requerida

Elegí una opción:

A. Implementar solo P0.  
B. Implementar P0 + P1.  
C. Profundizar investigación en áreas no concluyentes.  
D. No cambiar nada por ahora.