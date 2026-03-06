# Prompt 15 — Onboarding técnico, flujo de trabajo y contributing

## Contexto del proyecto

Nexus tiene buen material técnico, pero todavía le falta una guía unificada para que un ingeniero nuevo entienda:
- cómo levantar el monorepo
- cómo no romper boundaries
- cómo correr QA por servicio
- cómo contribuir con cambios seguros

**Prerequisito**: aplicar `docs/prompts/00_base_transversal.md`.

## Alcance obligatorio

Este prompt crea la capa documental para que el repo sea operable por más personas sin depender de conocimiento tribal.

---

## Lo que YA existe

- `README.md`
- `Makefile`
- `docker-compose.yml`
- `scripts/bootstrap/README.md`
- `scripts/db/README.md`
- `scripts/seed/README.md`
- `scripts/e2e/README.md`
- docs técnicas por área

---

## Qué implementar

### 1. CONTRIBUTING.md

Crear `CONTRIBUTING.md` con:
- estructura del monorepo
- boundaries por servicio
- branching/PR workflow
- reglas de testing mínimo antes de merge
- actualización de docs/contracts/prompts
- política de cambios breaking

### 2. Engineering onboarding guide

Crear `docs/engineering/ONBOARDING.md` con:
- quickstart local
- variables de entorno
- orden sugerido para entender el sistema
- cómo correr servicios individualmente
- dónde viven los contratos, prompts, runbooks y SDKs

### 3. Coding standards

Crear `docs/engineering/CODING_STANDARDS.md` con:
- Go: arquitectura por módulos, handlers/usecases/repository, tests
- Python: API/services/adapters, typing, fallback, observabilidad
- TypeScript: data fetching, routing, errores, permisos
- reglas cross-repo: logs, errors, contracts, auth, no lógica en lugares incorrectos

### 4. Regla de mantenimiento de docs

Dejar explícito:
- cambio relevante en código → actualizar doc/prompt/runbook/contracts si aplica
- no aceptar drift entre repo y suite documental

---

## Reglas de implementación

- No duplicar lo que ya está bien explicado en scripts/README; enlazarlo desde la guía principal.
- Explicar primero el mapa mental del sistema, después los comandos.
- Mantener foco práctico: cómo arrancar, dónde tocar, cómo verificar.

---

## Archivos a crear o modificar

### Crear
- `CONTRIBUTING.md`
- `docs/engineering/ONBOARDING.md`
- `docs/engineering/CODING_STANDARDS.md`

### Modificar
- `README.md`
- `docs/DOC.md`

---

## Criterios de aceptación

- [ ] Existe `CONTRIBUTING.md`
- [ ] Existe guía de onboarding técnico
- [ ] Existen standards mínimos por stack
- [ ] Queda documentado que docs/contracts/prompts deben mantenerse sincronizados
- [ ] Un ingeniero nuevo puede entender qué servicio tocar para cada feature

---

## Orden de ejecución recomendado

1. CONTRIBUTING
2. ONBOARDING
3. CODING_STANDARDS
4. Enlaces cruzados desde README y DOC
