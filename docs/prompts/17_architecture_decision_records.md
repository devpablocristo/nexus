# Prompt 17 — Registro de decisiones arquitectónicas (ADRs)

## Contexto del proyecto

Nexus ya tiene decisiones arquitectónicas fuertes, pero hoy viven dispersas entre README, docs y código. Falta un registro permanente y mantenible de las decisiones más importantes.

**Prerequisito**: aplicar `docs/prompts/00_base_transversal.md`.

## Alcance obligatorio

Este prompt crea el mecanismo para que decisiones futuras no vuelvan a quedar implícitas. No reemplaza la suite de prompts; la complementa con decisiones más estables y de largo plazo.

---

## Qué implementar

### 1. Estructura ADR

Crear:

```text
docs/adr/
├── README.md
├── 0001-bounded-context-separation.md
├── 0002-deterministic-enforcement-no-llm.md
├── 0003-core-vs-saas-dual-database.md
├── 0004-operators-no-direct-db-writes.md
├── 0005-clerk-for-identity.md
├── 0006-stripe-for-billing.md
├── 0007-terraform-aws-deployment-model.md
└── template.md
```

### 2. Template ADR

Cada ADR debe tener:
- status
- date
- context
- decision
- consequences
- alternatives considered

### 3. Captura inicial de decisiones

Las decisiones base a registrar son:
- separación por bounded contexts
- enforcement determinista sin LLM
- dos PostgreSQL separados (`core` y `saas`)
- operators sin writes directos a DB
- Clerk para identity
- Stripe para billing
- Terraform/AWS como modelo productivo

### 4. Regla de uso futuro

Dejar documentado cuándo abrir un ADR nuevo:
- cambio de boundary
- cambio de proveedor crítico
- cambio de protocolo
- cambio de estrategia de despliegue o data ownership

---

## Reglas de implementación

- Un ADR no describe cada detalle de implementación; captura una decisión y sus trade-offs.
- No duplicar prompts completos dentro de un ADR.
- Mantener naming secuencial y statuses claros (`proposed`, `accepted`, `superseded`).

---

## Archivos a crear o modificar

### Crear
- `docs/adr/README.md`
- `docs/adr/template.md`
- ADRs fundacionales `0001` a `0007`

### Modificar
- `README.md`
- `docs/DOC.md`

---

## Criterios de aceptación

- [ ] Existe carpeta `docs/adr/`
- [ ] Existe template de ADR
- [ ] Existen ADRs fundacionales del sistema
- [ ] Queda documentado cuándo crear un ADR nuevo

---

## Orden de ejecución recomendado

1. Template y README de ADRs
2. ADRs fundacionales
3. Enlaces desde README y DOC
