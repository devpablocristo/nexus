# Nexus — One-pager

**El nexo entre agentes financieros y lo protegido, y entre los equipos que supervisan.**

---

## Problema

Bots y agentes ya mueven fondos en entornos automatizados. Los equipos humanos que supervisan estan fragmentados: treasury no sabe que hace security, ops no sabe que aprobo compliance.

Sin una capa de gobierno explicita:

- Un agente puede operar fuera de los limites que el humano cree haber delegado.
- Una accion tecnicamente valida puede ser operacionalmente catastrofica.
- Approvals, evidencia y auditoria quedan dispersos.
- La coordinacion entre equipos depende de Slack, emails y reuniones.
- El equipo reacciona tarde a excepciones en lugar de gobernar la autonomia a tiempo.

**El problema no es solo ejecutar; es hacer util la autonomia sin perder control, y coordinar a los humanos sin canales informales.**

---

## Solucion

Nexus es el nexo unico en dos sentidos:

**Con los clientes**: los agentes (bots, sistemas) envian requests. Un engine determinista gobierna la autoridad delegada antes de que toquen infraestructura que mueve fondos. No custodia ni ejecuta.

**Con los equipos**: un solo agente IA — el mismo para todos — notifica anomalias, contextualiza, y ofrece acciones rapidas (botones, forms) en un unico chat (web y app movil). Los equipos no se coordinan entre si: se coordinan a traves de Nexus.

El bucle: **anomalia en el engine → notifica al agente IA → agente presenta contexto y opciones al equipo responsable → humano decide.**

---

## Diferenciacion

| | Policy engine clasico | Nexus |
|---|------------------------|--------|
| Unidad de evaluacion | Transaccion aislada | Patron (contexto, historial, riesgo) |
| Risk scoring | Reglas estaticas | Cascada multi-factor, baselines, canaries |
| Deteccion de abuso | Umbrales manuales | Ventanas temporales, recursos trampa |
| Interaccion con humanos | Alertas por email | Agente IA con contexto y acciones rapidas en un chat |
| Coordinacion entre equipos | No existe | Nexus es el puente: una voz para todos |
| Deployment | Acoplado a custodia | Standalone; se conecta a tu infra |
| Auditoria | Variable | Inmutable, con descomposicion de factores |

**Frase:** Nexus no es la puerta que mira una transaccion. Es el nexo que gobierna agentes y coordina humanos en operaciones financieras criticas.

---

## Nicho inicial

- **Vertical:** operaciones criticas en infraestructuras cripto automatizadas.
- **Casos:** withdrawals, treasury transfers, movimientos hot-to-cold.
- **Buyers:** Head of Security, COO, Treasury Lead, responsables de plataforma.

Crypto es el wedge. Las primitivas se disenan agnosticas para cualquier servicio financiero AI-native.

---

## Estado actual

- **MVP:** cerrado (resources, policies, actions, approvals, leases, execute, incidents, alerts).
- **Fase 0 (hardening):** cerrada en local; 4 blockers de deploy pendientes.
- **Fase 1A (risk scoring):** implementada (cascada multi-factor, 5 niveles de decision, baselines, canaries, hysteresis).
- **saas-core:** integrado (billing Stripe, auth JWT/Clerk, tenancy, metering, admin). Pendiente: migrations y tests.

---

## Proximo paso

1. Migrations y tests de saas-core.
2. Bucle anomalia → agente IA → contexto y opciones a responsables.
3. Interfaz: chat unico (web + app movil) con forms, botones, diagramas.
4. Features diferenciales: CapabilityLease, AutonomyBudget, intervenciones proporcionales.
5. Go-to-market: demo, doc de integracion, prospects — meta segunda semana de junio 2026.

---

## Contacto y documentacion

- Documentacion tecnica: [README.md](README.md), [DEFINITION.md](DEFINITION.md), [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md).
- Roadmap: [ROADMAP.md](ROADMAP.md).
- RFC: [RFC.md](RFC.md).
