# Contributing

## Mapa del monorepo

- `nexus-core`: data plane determinista
- `nexus-saas`: business plane multi-tenant
- `nexus-control-operators`: workers deterministas
- `nexus-ai-operators`: runtime AI asistido
- `nexus-tower`: UI de supervisión
- `pkgs/contracts`: contratos compartidos
- `docs/`: docs canónicas, prompts, runbooks y ADRs

## Reglas de boundaries

- No meter billing, users o tenant lifecycle en `nexus-core`.
- No meter enforcement, audit write ni policy engine en `nexus-saas`.
- Operators nunca escriben directo a DB.
- Tower no replica lógica crítica del backend.

## Flujo de trabajo

1. Identificar bounded context owner.
2. Cambiar código, tests y contratos en el mismo scope.
3. Actualizar docs/runbooks/prompts/ADRs si el cambio lo exige.
4. Ejecutar el set mínimo de verificación antes de merge.

## Verificación mínima antes de merge

- Go: `go test ./...` en el servicio afectado.
- Python: `pytest` en `nexus-ai-operators`.
- Frontend: `npm test` y `npm run build` en `nexus-tower`.
- Si cambian contracts: revisar `pkgs/contracts`, SDKs y docs.

## Breaking changes

Cambios en `/v1/*`, `/mcp`, `/a2a/*`, headers públicos, error codes, event schemas o SDKs requieren:

- actualización coordinada de docs
- nota de compatibilidad/deprecación
- review explícito del scope contractual
