# Roadmap: Go-To-Market + Vanguard IA (Nexus Gateway)

## Objetivo

Llevar `nexus-gateway` a dos metas simultaneas:

1. **Listo para vender (0-90 dias)**: onboarding rapido, controles empresariales, operacion estable y narrativa comercial cerrable.
2. **Diferenciacion vanguardista (3-12 meses)**: capacidades de seguridad e interoperabilidad que seran demanda fuerte en arquitecturas agentic.

## Documentos relacionados (repo)

- Pricing/planes (draft): `docs/sales/PLANS_PRICING_DRAFT.md`
- Playbooks de demo: `docs/sales/DEMO_EXEC_PLAYBOOK.md`, `docs/sales/DEMO_TECH_PLAYBOOK.md`
- Runbooks: `docs/runbooks/DEPLOY_PROD.md`, `docs/runbooks/INCIDENTS_P1_P2.md`, `docs/runbooks/RELEASE_GATES.md`
- Seguridad/compliance: `docs/security/THREAT_MODEL.md`, `docs/security/DATA_HANDLING_RETENTION.md`, `docs/security/RBAC_PERMISSIONS.md`

Nota: este archivo es un roadmap (que/por que/cuando). El "estado actual" y el "como probar" estan en `README.md` y `docs/DESCRITION-SPA.md`.

## Norte de producto (12 meses)

- Ser la capa de control estandar para ejecucion de tools en ecosistemas multi-agente.
- Ofrecer evidencia verificable de seguridad y trazabilidad (no solo logs internos).
- Integrar de forma nativa con protocolos de agentes y governance enterprise.

## Criterios de exito (definicion de "vendible")

- Un nuevo cliente integra su primer flujo en menos de 1 dia.
- Demo en vivo de 20 minutos, sin cambios manuales de codigo.
- 99.9% de disponibilidad mensual en entorno productivo.
- Security questionnaire estandar enterprise sin bloqueadores criticos.
- Primer caso pagado con KPI medible (riesgo/costo/tiempo reducido).

## Fase 0 (Semana 1-2): Base comercial minima

### Entregables

- Definir packaging comercial: `Starter`, `Growth`, `Enterprise`.
- Definir limites por plan: tools, rpm, retencion, export, soporte.
- Definir pricing inicial y politica de POC.
- Crear 2 playbooks comerciales:
  - demo tecnica (Platform/SecOps)
  - demo ejecutiva (riesgo/compliance/costo)

### KPI salida de fase

- 3 propuestas comerciales listas para enviar.
- Documento de pricing/planes validado internamente.

## Fase 1 (Semana 3-6): Productizacion para venta inmediata

### Entregables tecnicos

- **Consola Admin MVP**:
  - CRUD de tools
  - CRUD de policies
  - gestion de secrets
  - gestion de egress rules
  - vista de auditoria basica
- **Onboarding 1 hora**:
  - quickstart para REST y MCP
  - entorno demo reproducible
- **Observabilidad util para cliente**:
  - dashboard base (bloqueos, errores, latencia p95, rate-limits)
  - alertas iniciales

### Entregables operativos/comerciales

- Runbook de despliegue productivo.
- Runbook de incidentes (P1/P2).
- Matriz de responsabilidades de soporte (SLA interno).

### KPI salida de fase

- Time-to-first-run < 60 minutos en una instalacion limpia.
- Demo end-to-end ejecutable por preventa sin soporte de ingenieria.

## Fase 2 (Semana 7-10): Cierre enterprise basico

### Entregables tecnicos

- **SSO/OIDC para consola**.
- **RBAC empresarial** por permisos (no solo rol textual).
- **Auditoria administrativa** (quien cambio policy/tool/secret).
- **Cuotas por tenant** (hard limits por plan).

### Entregables de cumplimiento

- Paquete de seguridad:
  - arquitectura y modelo de amenazas
  - manejo de datos y retencion
  - controles alineados a NIST AI RMF / OWASP GenAI
- Plantillas legales base:
  - MSA
  - DPA
  - politica de seguridad

### KPI salida de fase

- Completar security questionnaire estandar con gaps menores.
- Habilitar primer cliente pago de riesgo medio.

## Fase 3 (Semana 11-13): "Listo para vender" formal

### Entregables

- SLO/SLA publicados (disponibilidad, soporte, tiempos de respuesta).
- Procedimientos de backup/restore probados.
- Proceso de release con gates:
  - unit + integration + e2e
  - smoke en entorno similar a produccion
- Material de venta final:
  - one-pager
  - deck comercial
  - ROI calculator simple

### KPI salida de fase

- 1-2 logos en POC activo.
- Al menos 1 conversion a contrato pago.

## Fase 4 (Mes 4-6): Diferenciacion vanguardista I

### Features prioritarias

1. **Interoperabilidad multi-protocolo**
- Ampliar capa de entrada para coexistencia `MCP + A2A`.
- Reutilizar pipeline de control actual para evitar bypass.

2. **Delegacion verificable (zero-trust agent identity)**
- Credenciales cortas por ejecucion.
- Cadena de delegacion auditable extremo a extremo.

3. **Red Team agentic en CI/CD**
- Suite de ataques (prompt injection, exfiltration attempts, tool misuse).
- Bloqueo de release por umbral de riesgo.

### KPI salida de fase

- 100% de flujos criticos de herramientas cubiertos por pruebas de abuso.
- Reduccion sostenida de incidentes de policy bypass.

## Fase 5 (Mes 7-12): Diferenciacion vanguardista II

### Features prioritarias

1. **Receipts criptograficos de auditoria**
- Firma de eventos exportables y verificables externamente.
- Endpoint de verificacion para terceros/auditores.

2. **Governance de riesgo y costo en tiempo real**
- Budget por tenant/equipo/agente.
- Kill-switch y approval humano para acciones high-risk.

3. **Marketplace de policy packs**
- Plantillas por vertical (finanzas, salud, soporte, retail).
- Versionado y rollout seguro de politicas.

### KPI salida de fase

- 3 vertical packs listos para venta.
- "Security proof" verificable usado en procesos de procurement.

## Backlog priorizado (resumen ejecutable)

1. Consola Admin MVP.
2. SSO/OIDC + RBAC por permisos.
3. Onboarding 1 hora + demo guiada.
4. Dashboards/alertas listos para cliente.
5. Paquete compliance + security docs.
6. Cuotas por plan/tenant.
7. A2A bridge sobre pipeline existente.
8. Delegacion verificable.
9. Red teaming en CI.
10. Auditoria con receipts firmados.

## Riesgos y mitigaciones

- **Riesgo**: crecer features sin cerrar venta.
  - **Mitigacion**: no iniciar Fase 4 sin 1-2 clientes pagos.
- **Riesgo**: deuda operativa bloquea enterprise.
  - **Mitigacion**: SLO/SLA + runbooks + backups antes de escalar.
- **Riesgo**: narrativa difusa (security vs agent platform).
  - **Mitigacion**: posicionamiento unico: "control plane para ejecucion de tools de agentes".

## Cadencia de seguimiento

- Review semanal de roadmap (producto + ventas + seguridad).
- Repriorizacion quincenal por feedback de POCs.
- Cierre mensual con:
  - avance por fase
  - KPIs
  - riesgos abiertos

