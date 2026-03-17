# Nexus — Reglas del proyecto

## Contexto
Monorepo con tres proyectos activos:
- `v2/control-plane` — servicio Go en producción
- `review-v1/` — PoC: engine de aprobación/review para requests de agentes, servicios y humanos
- `v1/` — legacy, no modificar sin pedido explícito

Todos siguen el mismo patrón hexagonal. La referencia canónica de arquitectura es `v2/control-plane`.

---

## Idioma
- **Código**: inglés (variables, funciones, tipos, constantes, archivos)
- **Comentarios** (`//`, `/* */`, `--` SQL, `#` Python): español
- **TODOs**: inglés
- **Respuestas**: español siempre

---

## Principios de diseño

- **DRY** — no duplicar lógica. Si algo se repite dos veces, abstraer.
- **YAGNI** — no agregar lo que no se pidió. Sin features especulativas.
- **SOLID** — SRP (un motivo de cambio por struct/clase), OCP (extensible sin modificar), LSP (subtipos sustituibles), ISP (interfaces segregadas), DIP (depender de abstracciones).
- **KISS** — tres líneas similares son mejores que una abstracción prematura.
- **Fail fast** — validar inputs al inicio, retornar error inmediato. No acumular errores.
- **Cambios quirúrgicos** — solo modificar lo que se pide. No "limpiar" código cercano.
- **No sobre-ingenierizar** — no error handling para casos imposibles, no feature flags, no backwards-compat shims.

---

## Flujo de trabajo

1. **TLDR primero** — en la primera línea, resumir en 1-2 oraciones qué se va a hacer.
2. **Ideal primero** — mostrar la forma ideal aunque sea costosa, luego la práctica si difieren.
3. **Esperar aprobación** — antes de implementar algo no trivial, proponer y esperar confirmación.
4. **Verificar antes de decir "listo"**:
   - Go: `go build ./...` + `go vet ./...` + `go test ./...`
   - Python: `python -m pytest` + `ruff check .` + `mypy .`
   - Si el entorno no permite correr tests, decirlo explícitamente.
5. **Nunca decir "listo", "funciona" o "debería andar"** sin evidencia de ejecución exitosa.

---

## Patrón CRUD canónico (universal — Go y Python)

**Todo módulo con entidades persistidas implementa exactamente estas 7 operaciones.** No se omite ninguna. Siempre en el mismo orden, con los mismos status codes.

| Operación | Método HTTP | Path | Status | Notas |
|-----------|-------------|------|--------|-------|
| Create | `POST` | `/v1/{entities}` | 201 | Retorna `{"id": "uuid"}` o entidad completa |
| Read | `GET` | `/v1/{entities}/{id}` | 200 | Incluye archivados |
| List | `GET` | `/v1/{entities}` | 200 | Excluye archivados por default |
| Update | `PATCH` | `/v1/{entities}/{id}` | 200 | Retorna entidad actualizada |
| Delete | `DELETE` | `/v1/{entities}/{id}` | 204 | **Hard delete** — físico, irreversible |
| Archive | `POST` | `/v1/{entities}/{id}/archive` | 204 | **Soft delete** — setea `archived_at` |
| Restore | `POST` | `/v1/{entities}/{id}/restore` | 204 | Limpia `archived_at` |

### Reglas inamovibles
- `DELETE` = **hard delete** siempre. Borra el registro de la base de datos. Irreversible.
- `Archive` = **soft delete**. Setea `archived_at = now()`. El registro sigue existiendo.
- `Restore` = limpia `archived_at`. Devuelve el registro al estado activo.
- Archive y Restore son **idempotentes** (204 aunque ya esté en ese estado).
- `GET /{id}` retorna archivados (con `archived_at` en DTO).
- `GET /` excluye archivados por default; `?archived=true` para incluirlos.
- **No existe** "soft delete con DELETE". DELETE siempre es hard.
- Cada módulo **debe** implementar las 7 operaciones. Si una entidad no tiene sentido archivar, no tiene `archived_at` en el schema — pero sí tiene DELETE (hard).

### Repository interface (Go)
```go
type Repository interface {
    Create(ctx context.Context, item domain.Entity) (*domain.Entity, error)
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Entity, error)
    List(ctx context.Context, filters ListFilters) ([]domain.Entity, error)
    Update(ctx context.Context, item domain.Entity) (*domain.Entity, error)
    DeleteByID(ctx context.Context, id uuid.UUID) error
    ArchiveByID(ctx context.Context, id uuid.UUID) error
    RestoreByID(ctx context.Context, id uuid.UUID) error
}
```

### Repository interface (Python)
```python
class Repository(Protocol):
    async def create(self, item: Entity) -> Entity: ...
    async def get_by_id(self, id: UUID) -> Entity | None: ...
    async def list(self, filters: ListFilters) -> list[Entity]: ...
    async def update(self, item: Entity) -> Entity: ...
    async def delete_by_id(self, id: UUID) -> None: ...
    async def archive_by_id(self, id: UUID) -> None: ...
    async def restore_by_id(self, id: UUID) -> None: ...
```

### Errores HTTP estructurados

| Caso | Status | Code |
|------|--------|------|
| No encontrado | 404 | `NOT_FOUND` |
| Ya existe / conflicto | 409 | `CONFLICT` |
| Input inválido | 400 | `VALIDATION` |
| No autorizado | 401 | `UNAUTHORIZED` |
| Forbidden | 403 | `FORBIDDEN` |
| Server error | 500 | `INTERNAL` |

### Sentinel errors (Go)
```go
var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrArchived      = errors.New("entity is archived")
)
```

---

## Go — Arquitectura hexagonal

**Siempre hexagonal para Go.** Sin excepciones.

Un solo binario por servicio. Módulos con ports & adapters. DI manual en `wire/setup.go`.

### Estructura de directorios (canónica)

```
cmd/api/
    main.go

internal/
    <modulo>/
        usecases/domain/
            entities.go          # entidades puras (sin deps HTTP ni DB)
        usecases.go              # lógica de negocio + ports (interfaces)
        handler.go               # adapter HTTP (net/http)
        handler/dto/
            dto.go               # DTOs entrada/salida HTTP
        repository.go            # port: interface de persistencia + sentinel errors
        repository_inmemory.go   # adapter: in-memory (siempre presente)
        repository_postgres.go   # adapter: pgx (cuando aplica)
        *_test.go                # tests del módulo

wire/
    setup.go                     # DI manual, wiring completo

migrations/
    0001_initial.up.sql
    0001_initial.down.sql
```

### Naming obligatorio

| Archivo | Struct principal | Constructor |
|---------|-----------------|-------------|
| `usecases.go` | `type Usecases struct` | `func NewUsecases(...)` |
| `handler.go` | `type Handler struct` | `func NewHandler(...)` |
| `repository.go` | `type Repository interface` | — |
| `repository_inmemory.go` | `type InMemoryRepository struct` | `func NewInMemoryRepository(...)` |
| `repository_postgres.go` | `type PostgresRepository struct` | `func NewPostgresRepository(...)` |

### Accept interfaces, return structs

- Los constructores reciben **interfaces**, nunca tipos concretos.
- Los constructores devuelven `*Struct`, **nunca** una interfaz.
- Las interfaces se definen en el **consumidor** (quien las necesita), no en el proveedor.
- Cada adapter define su propio port con **solo los métodos que necesita** (ISP).

```go
// handler.go — port mínimo
type requestUsecase interface {
    Submit(ctx context.Context, req SubmitRequest) (*domain.Request, error)
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Request, error)
}

type Handler struct { uc requestUsecase }

func NewHandler(uc requestUsecase) *Handler {
    return &Handler{uc: uc}
}
```

### Wire builder pattern (dependencias opcionales)
```go
func NewHandler(uc usecasePort) *Handler {
    return &Handler{uc: uc}
}

func (h *Handler) WithAuditSink(sink auditSink) *Handler {
    h.audit = sink
    return h
}
```

### Register pattern (rutas HTTP)
```go
func (h *Handler) Register(mux *http.ServeMux) {
    mux.HandleFunc("POST /v1/policies", h.create)
    mux.HandleFunc("GET /v1/policies", h.list)
    mux.HandleFunc("GET /v1/policies/{id}", h.getByID)
    mux.HandleFunc("PATCH /v1/policies/{id}", h.update)
    mux.HandleFunc("DELETE /v1/policies/{id}", h.deleteByID)
    mux.HandleFunc("POST /v1/policies/{id}/archive", h.archiveByID)
    mux.HandleFunc("POST /v1/policies/{id}/restore", h.restoreByID)
}
```

### Convenciones Go

- **`context.Context`** siempre como primer parámetro
- **No `init()`** en código de aplicación
- **No `panic()`** en producción — retornar error siempre
- **No `_`** para ignorar errores — si no se maneja, al menos loguear con `slog.Error`
- **Slices como valores** `[]Type`, no `*[]Type`
- **Punteros para structs** que retorna el repositorio: `*domain.Entity`
- **Enums como typed string**: `type Effect string; const EffectAllow Effect = "allow"`
- **No global state** — todo por DI
- **IDs como `uuid.UUID`** (no string, no int64)
- **Error wrapping**: `fmt.Errorf("create policy: %w", err)` — siempre agregar contexto
- **Early return** — evitar nesting excesivo; validar y retornar temprano
- **Errors son valores** — nunca usar strings para comparar errores; usar `errors.Is()` o `errors.As()`
- **Structs literales nombrados** — `Policy{Name: "x"}`, nunca posicionales `Policy{"x"}`
- **Tabla de decisiones** para mapeos complejos, no cadenas de if/else
- Todo config desde **variables de entorno**, nunca hardcodeado

### Logging Go
```go
// Siempre slog, nunca fmt.Printf ni log.Println
slog.Info("request evaluated", "request_id", id, "decision", decision)
slog.Error("policy evaluation failed", "error", err, "request_id", id)
```

### Health endpoints
- `GET /healthz` — liveness (siempre 200)
- `GET /readyz` — readiness (verifica DB, etc.)
- **Fuera del middleware de auth** — nunca protegidos por API key

### Graceful shutdown
- Siempre `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`
- Server con timeout de shutdown (30s default)
- Cerrar conexiones de DB en cleanup

### AI adapter pattern (timeout + fallback)
```go
type AIContextualizer interface {
    Summarize(ctx context.Context, input SummarizeInput) (string, error)
}

// El adapter real: timeout 5s, fallback a datos crudos si falla
// Si falla: retornar datos crudos + marcar ai_degraded=true
// NUNCA fallar la request por un error de AI
```

### Audit emission
- Best-effort: emitir desde el handler, después de la lógica de negocio
- **Nunca fallar la request** si el audit falla — loguear el error con `slog.Error`
- Append-only: los eventos de audit nunca se modifican ni eliminan

---

## Python — FastAPI

**Para Python siempre FastAPI.** Arquitectura clean/layered adaptada a FastAPI.

### Estructura de directorios

```
app/
    main.py                  # FastAPI app factory + startup/shutdown
    config.py                # pydantic-settings BaseSettings

    <modulo>/
        domain.py            # entidades puras (Pydantic BaseModel, dataclass)
        service.py           # lógica de negocio (equivale a usecases.go)
        router.py            # adapter HTTP (APIRouter)
        schemas.py           # DTOs request/response (Pydantic)
        repository.py        # interface (Protocol) + implementación
        dependencies.py      # FastAPI Depends() factories

    shared/
        errors.py            # HTTPException helpers, error codes
        database.py          # async engine, session factory
        auth.py              # API key dependency

tests/
    conftest.py
    <modulo>/
        test_router.py
        test_service.py

alembic/
    versions/
```

### Convenciones Python

- **Type hints siempre** — `def create(self, item: Entity) -> Entity`
- **Pydantic para DTOs** — `class CreatePolicyRequest(BaseModel)` con validadores
- **Pydantic Settings para config** — `class Settings(BaseSettings)`, nunca hardcoded
- **async/await** para I/O — `async def get_by_id(self, id: UUID) -> Entity | None`
- **Protocol para interfaces** — `class Repository(Protocol):` (equivale a interface Go)
- **Depends() para DI** — inyección vía FastAPI, no global state
- **HTTPException para errores** — con `status_code` y `detail` estructurado
- **Alembic para migraciones** — nunca modificar una existente, crear nueva
- **pytest + httpx.AsyncClient** para tests
- **Ruff para lint**, **mypy para tipos**
- **No `print()`** — usar `logging` con structlog o loguru
- **Early return** — mismo principio que Go
- **Enums**: `class Effect(str, Enum): ALLOW = "allow"`

### CRUD pattern FastAPI
```python
router = APIRouter(prefix="/v1/policies", tags=["policies"])

@router.post("/", status_code=201)
async def create(req: CreatePolicyRequest, svc: PolicyService = Depends(get_service)) -> PolicyResponse: ...

@router.get("/{id}")
async def get_by_id(id: UUID, svc: PolicyService = Depends(get_service)) -> PolicyResponse: ...

@router.get("/")
async def list(archived: bool = False, svc: PolicyService = Depends(get_service)) -> list[PolicyResponse]: ...

@router.patch("/{id}")
async def update(id: UUID, req: UpdatePolicyRequest, svc: PolicyService = Depends(get_service)) -> PolicyResponse: ...

@router.delete("/{id}", status_code=204)
async def delete(id: UUID, svc: PolicyService = Depends(get_service)) -> None: ...

@router.post("/{id}/archive", status_code=204)
async def archive(id: UUID, svc: PolicyService = Depends(get_service)) -> None: ...

@router.post("/{id}/restore", status_code=204)
async def restore(id: UUID, svc: PolicyService = Depends(get_service)) -> None: ...
```

---

## Tests

### Go
- **Table-driven tests** — `[]struct{ name, input, expected }`
- **`t.Parallel()`** en todos los tests que no compartan estado
- **`httptest.NewRequest` + `httptest.NewRecorder`** para handlers
- **In-memory repositories** para tests (sin DB real, sin mocks sintéticos)
- Cubrir: happy path, not found, validation error, conflict, archive/restore lifecycle

### Python
- **pytest** con `@pytest.mark.asyncio`
- **httpx.AsyncClient** para tests de endpoints
- **In-memory repositories** o SQLite para tests (no mocks de DB)
- Misma cobertura: happy path, not found, validation, conflict

---

## Migraciones

- **Go**: archivos `NNNN_descripcion.up.sql` / `NNNN_descripcion.down.sql`
- **Python**: Alembic con `alembic revision --autogenerate -m "descripcion"`
- Secuenciales, descripción en inglés
- **Nunca modificar** una migración existente — crear nueva
- `IF NOT EXISTS` para idempotencia
- **Sin `ROUND()`** en migraciones — mantener precisión
- Comentarios SQL en español

---

## Configuración y entorno

- Un solo codebase para todos los ambientes (local, staging, prod)
- **Sin forks de lógica por ambiente**
- Todo config desde env vars
- `.env.example` como referencia canónica — mantenerlo actualizado
- `docker-compose.yml` para desarrollo local

---

## Paquetes compartidos Go (v2/pkgs/go-pkg)

| Paquete | Uso |
|---------|-----|
| `pkgs/go-pkg/handlers` | `DecodeJSON`, `WriteJSON`, health endpoints |
| `pkgs/go-pkg/postgres` | `pgxpool`, migrations |
| `pkgs/go-pkg/apikey` | Auth SHA256 middleware |
| `pkgs/go-pkg/httpserver` | Security headers, graceful shutdown |
| `pkgs/go-pkg/observability` | `slog` JSON, Prometheus, RED middleware |

---

## Documentación

Antes de crear o actualizar documentación:
1. **Escanear el código real**
2. **Verificar** que cada afirmación tiene respaldo en código
3. **No inventar** features, endpoints o flujos que no existan
4. **Eliminar** referencias a componentes que ya no existen

---

## Reglas críticas (nunca violar)

- **NUNCA** valores hardcodeados (URLs, keys, timeouts fijos sin config)
- **NUNCA** exponer structs/modelos de dominio por HTTP — siempre DTOs
- **NUNCA** modificar migraciones existentes — crear nuevas
- **NUNCA** usar `panic()` (Go) o excepciones genéricas sin tipo (Python) en producción
- **NUNCA** ignorar errores con `_` (Go) o `except: pass` (Python)
- **NUNCA** decir "listo" sin haber buildado/testeado
- **NUNCA** modificar código que no fue explícitamente pedido
- **NUNCA** crear archivos `.md` sin pedido explícito
- **NUNCA** usar DELETE como soft delete — DELETE es hard delete, Archive es soft delete
- **NUNCA** omitir Archive/Restore en un módulo CRUD que tiene `archived_at`
- **NUNCA** usar `fmt.Printf`/`log.Println` (Go) o `print()` (Python) para logging — usar slog/structlog
