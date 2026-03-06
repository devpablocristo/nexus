# Prompt 12 — Policy DSL, MCP, A2A & Contratos Operativos

## Contexto del proyecto

`nexus-core` ya expone enforcement runtime, `Policy DSL`, `MCP` y `A2A`, pero hoy la documentación está repartida entre `docs/DOC.md`, `README.md`, código y contratos. Falta una referencia canónica para implementadores e integradores.

**Prerequisito**: aplicar `docs/prompts/00_base_transversal.md`.

## Alcance obligatorio

Este prompt formaliza la capa de contratos y protocolos más sensible del sistema. No es documentación opcional: define cómo se razona, integra y prueba la frontera del producto con agentes y herramientas externas.

---

## Lo que YA existe

- Policy evaluation en `nexus-core/internal/policy/`
- Endpoints `POST /mcp` y `POST /a2a/call`
- Catálogo de errores en `pkgs/contracts/error-codes.json`
- Schema de eventos en `pkgs/contracts/events.schema.json`
- OpenAPI snapshots en `pkgs/contracts/`
- docs parciales en `docs/DOC.md` y `docs/NAMING_AND_BOUNDARIES.md`

---

## Qué implementar

### 1. Referencia dedicada de Policy DSL

Crear `docs/policy/POLICY_DSL_REFERENCE.md` con:
- semántica first-match
- prioridades
- paths permitidos (`input.*`, `context.*`, `tool.*`)
- operadores válidos
- `all` / `any` / `not`
- límites (`rate_limit`, `require_idempotency`, `require_approval`, bytes máximos)
- ejemplos válidos e inválidos
- errores esperados

### 2. Guía práctica de policies

Crear `docs/policy/POLICY_DSL_COOKBOOK.md` con ejemplos reales:
- deny por PII detectada
- require approval para writes
- límite por tool
- deny por actor/rol/contexto
- protección de egress/sensitivity

### 3. Guía de MCP

Crear `docs/protocols/MCP_GUIDE.md` con:
- rutas
- auth/scopes
- estructura de request/response
- errores frecuentes
- límites
- ejemplos end-to-end

### 4. Guía de A2A

Crear `docs/protocols/A2A_GUIDE.md` con:
- casos de uso
- payloads
- auth interna/externa
- qué puede y qué no puede hacer A2A
- relación con approvals, policies e idempotencia

### 5. Contract tests dedicados

Agregar fixtures/tests para:
- parseo válido/ inválido de Policy DSL
- compatibilidad de payloads MCP
- compatibilidad de payloads A2A
- mapping consistente de error codes

### 6. Alineación con OpenAPI y SDKs

Todo cambio contractual debe:
- quedar reflejado en OpenAPI snapshots
- impactar `sdks/python-sdk`, `sdks/typescript-sdk`, `sdks/go-sdk` cuando aplique
- documentar compatibilidad/deprecación

---

## Reglas de implementación

- Policy DSL debe quedar documentado como lenguaje, no solo como feature.
- Los ejemplos de MCP/A2A deben usar headers y rutas reales del repo.
- No inventar protocolos paralelos fuera de `/mcp`, `/a2a/*` y `/v1/*`.
- Los contratos internos deben quedar claramente separados de los públicos.

---

## Archivos a crear o modificar

### Crear
- `docs/policy/POLICY_DSL_REFERENCE.md`
- `docs/policy/POLICY_DSL_COOKBOOK.md`
- `docs/protocols/MCP_GUIDE.md`
- `docs/protocols/A2A_GUIDE.md`

### Modificar
- `docs/DOC.md`
- `README.md`
- `pkgs/contracts/error-codes.json` si faltan códigos contractuales
- tests de `nexus-core/internal/policy/`, `nexus-core/internal/mcp/`, `nexus-core/internal/a2a/`

---

## Criterios de aceptación

- [ ] Existe referencia canónica del Policy DSL
- [ ] Existe cookbook práctico con casos reales
- [ ] MCP y A2A tienen guías dedicadas para integradores
- [ ] Hay tests de compatibilidad para DSL/protocolos
- [ ] OpenAPI/SDKs/contracts quedan alineados con la doc
- [ ] Los errores contractuales mapean al catálogo compartido

---

## Orden de ejecución recomendado

1. Reference del Policy DSL
2. Guía práctica
3. Guía MCP
4. Guía A2A
5. Contract tests
6. Alineación final con OpenAPI/SDKs/docs
