# Prompt 14 — Respuesta a incidentes, on-call y postmortems

## Contexto del proyecto

Nexus ya tiene piezas operativas importantes: rollback, backup/DR, launch checklist, alerting, incidents y operators. Lo que falta es una guía unificada de respuesta a incidentes y operación humana.

**Prerequisito**: aplicar `docs/prompts/00_base_transversal.md`.

## Alcance obligatorio

Este prompt formaliza cómo responde el equipo cuando el sistema falla, se degrada o compromete seguridad/tenancy. No es solo runbook: es parte del readiness productivo.

---

## Lo que YA existe

- `docs/runbooks/DEPLOY_ROLLBACK.md`
- `docs/runbooks/DB_BACKUP_DR.md`
- `docs/runbooks/SECRET_ROTATION.md`
- `docs/runbooks/LAUNCH_CHECKLIST.md`
- `docs/runbooks/SLO_SLI.md`
- incidentes y alert rules en `nexus-saas`
- control operators y AI operators con acciones/mitigaciones

---

## Qué implementar

### 1. Severidades y clasificación

Crear `docs/runbooks/INCIDENT_RESPONSE.md` con matriz:
- `SEV-1` caída total / breach / cross-tenant risk
- `SEV-2` degradación severa / writes bloqueados / alta tasa de denials o failures
- `SEV-3` degradación parcial o workaround disponible
- `SEV-4` issue menor / seguimiento

### 2. Flujo operativo

Definir:
- detección
- triage
- asignación de incident commander
- mitigación
- comunicación
- cierre
- postmortem

### 3. Playbooks mínimos

Incluir procedimientos para:
- `nexus-core` degradado
- `nexus-saas` degradado
- Redis caído
- Postgres core/saas con problemas
- Tower inaccesible
- LLM provider caído
- webhook storm / rate limit incident
- security incident / key leak

### 4. Comunicación

Definir plantillas para:
- comunicación interna
- comunicación a stakeholders
- comunicación a tenants/admins si aplica

### 5. Postmortem

Crear template en `docs/runbooks/POSTMORTEM_TEMPLATE.md`:
- timeline
- impacto
- causa raíz
- qué funcionó / qué no
- acciones correctivas
- owner y fechas

---

## Reglas de implementación

- Incidentes de seguridad y cross-tenant exposure siempre escalan a máxima severidad.
- No depender solo de memoria individual; todo paso crítico debe quedar escrito.
- Los playbooks deben referenciar runbooks ya existentes, no duplicarlos innecesariamente.
- Postmortem sin blame, con acciones concretas.

---

## Archivos a crear o modificar

### Crear
- `docs/runbooks/INCIDENT_RESPONSE.md`
- `docs/runbooks/POSTMORTEM_TEMPLATE.md`

### Modificar
- `docs/runbooks/LAUNCH_CHECKLIST.md`
- `docs/runbooks/SLO_SLI.md`
- `README.md` o `docs/DOC.md` para enlazar operación

---

## Criterios de aceptación

- [ ] Existe matriz de severidades
- [ ] Existe flujo operativo end-to-end
- [ ] Hay playbooks mínimos por falla crítica
- [ ] Hay plantillas de comunicación
- [ ] Existe template de postmortem
- [ ] La documentación enlaza con rollback/DR/secret rotation ya existentes

---

## Orden de ejecución recomendado

1. Severidades y flujo operativo
2. Playbooks por escenario
3. Comunicación
4. Template de postmortem
5. Enlaces con runbooks existentes
