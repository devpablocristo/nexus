# Incident Response

Runbook operativo principal para incidentes de Nexus.

## Severidades

| Severidad | Criterio |
|-----------|----------|
| `SEV-1` | caída total, security incident, breach, cross-tenant exposure |
| `SEV-2` | degradación severa, writes bloqueados, deny/error spike sostenido |
| `SEV-3` | degradación parcial con workaround |
| `SEV-4` | issue menor, seguimiento sin impacto alto |

Incidentes de seguridad o exposición cross-tenant escalan siempre a `SEV-1`.

## Flujo operativo

1. Detección: alerta, reporte humano o smoke failure.
2. Triage: confirmar impacto, severidad y alcance.
3. Asignación:
   - incident commander
   - owner técnico primario
   - communications owner si aplica
4. Mitigación: rollback, throttle, suspensión temporal, failover o aislamiento.
5. Comunicación: interna inmediata; externa si hay impacto a tenants.
6. Cierre: servicio estable, impacto acotado, siguientes pasos claros.
7. Postmortem: usar `POSTMORTEM_TEMPLATE.md`.

## Playbooks mínimos

- `nexus-core` degradado:
  - revisar `/readyz`, `/metrics`, error rate y circuit breakers
  - evaluar rollback (`DEPLOY_ROLLBACK.md`)
- `nexus-saas` degradado:
  - revisar `/health`, DB, webhooks y alert evaluator
- Redis caído:
  - validar impacto en rate limits y fallback a inmemory
- Postgres core/saas:
  - revisar backup/restore (`DB_BACKUP_DR.md`)
- Tower inaccesible:
  - confirmar si APIs siguen sanas; incidentar como UX/control plane
- LLM provider caído:
  - confirmar fallback determinista en `nexus-ai-operators`
- webhook storm / rate-limit incident:
  - aislar source, throttlear, validar idempotencia
- key leak / security incident:
  - rotar secretos (`SECRET_ROTATION.md`), revocar access y escalar a `SEV-1`

## Comunicación

- Interna:
  - severidad
  - impacto observado
  - owner/IC
  - mitigación en curso
- Tenant-facing:
  - qué falla
  - qué impacto hay
  - workaround si existe
  - próxima actualización comprometida
