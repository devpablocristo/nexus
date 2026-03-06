# Prompt 16 — Estrategia de testing y gates de release

## Contexto del proyecto

Nexus ya tiene tests distribuidos por servicio, e2e scripts, load tests, observabilidad y CI. Falta consolidar una estrategia formal de calidad y salida a producción por tipo de cambio.

**Prerequisito**: aplicar `docs/prompts/00_base_transversal.md`.

## Alcance obligatorio

Este prompt formaliza qué significa "listo" en `nexus`. La calidad no queda a criterio del autor del cambio: se explicita por servicio y por tipo de impacto.

---

## Lo que YA existe

- tests Go en `nexus-core`, `nexus-saas`, `nexus-control-operators`
- tests Python en `nexus-ai-operators`
- tests frontend en `nexus-tower`
- `scripts/e2e/README.md`
- `scripts/loadtest/README.md`
- CI en `.github/workflows/ci.yml`

---

## Qué implementar

### 1. Matriz de testing por servicio

Crear `docs/testing/TEST_STRATEGY.md` con matriz:
- `nexus-core`
- `nexus-saas`
- `nexus-control-operators`
- `nexus-ai-operators`
- `nexus-tower`
- SDKs

Tipos:
- unit
- integration
- contract
- e2e
- smoke
- load
- security

### 2. Gates de release por categoría de cambio

Crear `docs/testing/RELEASE_GATES.md` con gates mínimos según cambio:
- cambio solo UI
- cambio en contract/API
- cambio en auth
- cambio en policy/enforcement
- cambio en billing
- cambio en operators
- cambio en AI runtime
- cambio en infra

### 3. Contract gating

Definir que cambios en:
- OpenAPI snapshots
- event schemas
- error codes
- SDK clients

requieren verificación explícita y actualización coordinada.

### 4. Checklist de evidencia de release

Definir evidencia mínima antes de merge/release:
- builds
- tests
- snapshots/contracts
- smoke
- changelog/docs
- rollback path

---

## Reglas de implementación

- No todos los cambios requieren load test, pero los cambios que afectan runtime crítico sí.
- Cualquier cambio en enforcement, auth, billing o contracts requiere gates más estrictos.
- Si se toca un protocolo o schema compartido, los SDKs y docs deben entrar en el mismo review scope.

---

## Archivos a crear o modificar

### Crear
- `docs/testing/TEST_STRATEGY.md`
- `docs/testing/RELEASE_GATES.md`

### Modificar
- `.github/workflows/ci.yml`
- `README.md`
- `docs/runbooks/LAUNCH_CHECKLIST.md`

---

## Criterios de aceptación

- [ ] Existe estrategia de testing consolidada
- [ ] Existen release gates por tipo de cambio
- [ ] Contracts/error-codes/events/SDKs quedan incluidos en la estrategia
- [ ] La salida a producción requiere evidencia mínima documentada

---

## Orden de ejecución recomendado

1. Matriz de testing por servicio
2. Gates de release por categoría
3. Contract gating
4. Evidence checklist y alineación con CI/runbooks
