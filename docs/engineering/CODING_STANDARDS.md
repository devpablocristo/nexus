# Coding Standards

## Go

- Mantener separación `handler -> usecases -> repository -> models/adapters`.
- Errores públicos alineados al catálogo `pkgs/contracts/error-codes.json`.
- Tests unitarios para lógica y contract tests para superficies HTTP/protocolos.

## Python (`nexus-ai-operators`)

- Separar API, services, adapters y runtime prompting.
- Tipado explícito en APIs y servicios.
- Fallback determinista y observabilidad obligatorios para flujos LLM.

## TypeScript (`nexus-tower`)

- Consumir APIs existentes; no duplicar clientes ni reglas críticas.
- Permisos y visibilidad derivados del backend.
- Manejar loading/error states como parte del producto, no como afterthought.

## Cross-repo

- No secrets hardcodeados.
- No writes directos a DB desde operators.
- Si cambia contrato o doc maestra, actualizarlo en el mismo cambio.
- Evitar drift entre código, OpenAPI, schemas, SDKs y docs.
