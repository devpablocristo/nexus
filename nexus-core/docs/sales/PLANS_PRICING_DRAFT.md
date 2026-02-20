# PLANS_PRICING_DRAFT.md

## Principios

- cobrar por valor de control (riesgo reducido + auditabilidad), no solo por llamadas.
- limites claros por plan (tools, rpm, retencion, soporte).

## Draft de planes

| Plan | Target | Tools max | Run RPM | Retencion audit | Soporte |
|---|---|---:|---:|---:|---|
| Starter | equipos iniciales | 20 | 300 | 30 dias | best effort |
| Growth | equipos productivos | 75 | 1200 | 90 dias | horario laboral |
| Enterprise | orgs reguladas | 250+ | 5000+ | 365 dias | SLA 24/7 |

## Features por plan

- Starter:
  - REST + MCP
  - policy/egress/secrets/audit
  - dashboard starter
- Growth:
  - todo Starter + SSO/RBAC
  - export SIEM extendido
  - hard limits por tenant
- Enterprise:
  - todo Growth + controles avanzados
  - soporte de compliance/procurement
  - roadmap features vanguardistas (segun contrato)

## Modelo comercial sugerido

- Fee base mensual por tenant + tramos de volumen.
- Add-ons:
  - retencion extendida
  - soporte premium
  - paquetes de politicas por vertical

## Oferta POC

- Duracion: 2-4 semanas
- Alcance: 1-2 casos de uso criticos
- Exito: KPI de riesgo/costo/tiempo pactados al inicio

