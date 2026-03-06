# Prompt 00 — Base transversal de arquitectura, calidad y límites para Nexus

## Contexto del proyecto

Nexus no es una app única: es un **monorepo poliglota con bounded contexts explícitos** que juntos forman un plano de control de ejecución para agentes y operadores.

Servicios principales:

| Servicio | Rol | Stack | DB |
|----------|-----|-------|----|
| `nexus-core` | data plane determinista: run/simulate, enforcement, audit, MCP, A2A | Go/Gin | PostgreSQL `nexus` |
| `nexus-saas` | business plane multi-tenant: orgs, billing, incidents, actions, alerts, assistant proxy, notifications | Go/Gin | PostgreSQL `nexus_saas` |
| `nexus-control-operators` | plano de control determinista: sentry, coordinator, mitigation, recovery | Go | sin DB, persistencia en archivos |
| `nexus-ai-operators` | operadores asistidos por IA y backend del assistant | Python/FastAPI | sin DB |
| `nexus-tower` | UI de supervisión | React/TypeScript | sin DB |
| `pkgs/contracts` | contratos compartidos | JSON/OpenAPI | n/a |
| `sdks/*` | clientes para integraciones externas | Go/Python/TypeScript | n/a |

Referencias base del repo:
- `README.md`
- `docs/DOC.md`
- `docs/SERVICE_BOUNDARIES.md`
- `docs/AGENT_OPERATED_MODEL.md`
- `docs/NAMING_AND_BOUNDARIES.md`
- `pkgs/contracts/error-codes.json`
- `pkgs/contracts/events.schema.json`

---

## Alcance obligatorio

Todo lo definido en esta suite de prompts forma parte del alcance requerido del proyecto. Nada debe interpretarse como opcional o como "fase 2" salvo que el texto lo diga de forma explícita.

La secuencia de implementación existe solo para:
- respetar dependencias técnicas
- minimizar retrabajo
- evitar drift entre servicios, contratos, SDKs, docs y observabilidad

---

## Invariantes arquitectónicas

Estas reglas no se negocian. Todo prompt posterior debe respetarlas.

1. **No LLM en el pipeline de enforcement**
   `nexus-core` toma decisiones de ejecución de forma determinista. La IA nunca decide allow/deny sobre `/v1/run`.

2. **AI y control operators no escriben directo en DB**
   `nexus-control-operators` y `nexus-ai-operators` operan vía APIs/contratos internos. No hacen writes directos a PostgreSQL.

3. **Separación estricta core vs saas**
   `nexus-core` no posee billing/tenant business state.
   `nexus-saas` no implementa enforcement runtime, audit write ni policy engine del core.

4. **Tower es supervisión, no source of truth**
   `nexus-tower` consume APIs y orquesta UI. No replica lógica de enforcement ni reglas críticas del backend.

5. **Contratos primero**
   Los contratos HTTP, OpenAPI, schemas de eventos y catálogos de errores son parte del producto. Un cambio real no queda completo si rompe contratos sin versionado/deprecación.

6. **Multi-tenant en todo el business plane**
   Todo dato de negocio y configuración tenant-aware vive con `org_id` y con separación clara por servicio/DB.

7. **Headers, rutas y namespaces estables**
   No renombrar headers públicos, namespaces de métricas, rutas `/v1/*`, `/mcp`, `/a2a/*` ni catálogos compartidos sin proceso formal.

---

## Convención de idioma

- La documentación y los prompts se escriben en español.
- Se conservan en inglés los nombres de archivo, símbolos, headers, rutas, variables de entorno, contratos, enums y términos muy asentados como `Policy DSL`, `MCP`, `A2A`, `OpenAPI`, `SDK`, `SLO/SLI`.
- No traducir identificadores de código ni nombres públicos del producto.

---

## Estructura de la suite de prompts

| Prompt | Tema |
|--------|------|
| `00_base_transversal.md` | base de arquitectura, calidad y límites |
| `01_user_identity_clerk_aws.md` | identidad, Clerk, JWT/OIDC, sync de usuarios |
| `02_billing_stripe.md` | billing multi-tenant con Stripe |
| `03_admin_console_ui.md` | admin console y superficies de gestión |
| `04_email_notifications_ses.md` | notificaciones email e in-app |
| `05_developer_experience_cicd.md` | OpenAPI, SDKs, portal, CI/CD |
| `06_prod_infrastructure_terraform.md` | infraestructura AWS/Terraform y DR |
| `07_security_hardening.md` | hardening y controles de seguridad |
| `08_monitoring_observability.md` | métricas, dashboards, alerting, SLO |
| `09_final_polish_launch.md` | launch readiness, lifecycle, polish |
| `10_production_hardening_final.md` | cierre de gaps productivos finales |
| `11_ai_runtime_prompting_eval.md` | prompting runtime, evaluaciones y guardrails de IA |
| `12_policy_dsl_mcp_a2a_contracts.md` | Policy DSL y protocolos MCP/A2A |
| `13_data_model_events_ownership.md` | modelo de datos, ownership y catálogo narrativo de eventos |
| `14_incident_response_oncall.md` | respuesta a incidentes, on-call y postmortems |
| `15_engineering_onboarding_contributing.md` | onboarding de ingeniería y contributing |
| `16_test_strategy_release_gates.md` | estrategia de testing y gates de release |
| `17_architecture_decision_records.md` | ADRs y decisiones arquitectónicas permanentes |

---

## Estándares de ingeniería obligatorios

### E1. Boundaries primero

- Cada feature nueva debe ubicarse en un bounded context concreto.
- Si una capacidad cruza `core`, `saas`, `operators`, `ai` y `tower`, el prompt debe explicitar owner primario, contratos internos y efectos colaterales.
- Nunca mezclar ownership "por conveniencia".

### E2. Determinismo del data plane

- `/v1/run`, `/v1/run/simulate`, `/mcp`, `/a2a/call` siguen pipeline determinista.
- DLP, schemas, policies, approvals, rate limits, SSRF, secrets, timeout budget e idempotencia se resuelven sin intervención LLM.
- La IA solo propone, resume, clasifica o asiste fuera del path crítico.

### E3. Errores tipados y catálogo compartido

- Los errores públicos deben mapear al catálogo de `pkgs/contracts/error-codes.json`.
- Los servicios Go usan errores tipados, no strings ad hoc.
- Las respuestas de error deben ser consistentes entre `core` y `saas`.
- Los prompts deben mencionar explícitamente códigos esperados en flujos críticos: auth, policy deny, rate limit, idempotency, egress, timeout.

### E4. Validación de entrada/salida

- DTOs y request models con validación explícita.
- JSON Schema para tool input/output cuando aplique.
- Policy DSL, eventos y contratos internos deben validarse con schemas o parsers dedicados.
- Nunca confiar en payloads internos "porque vienen del mismo sistema".

### E5. Transacciones, idempotencia y consistencia

- Operaciones multi-tabla usan transacciones explícitas.
- Writes con replay posible usan idempotencia real, no best effort.
- Side-effects externos se desacoplan con outbox/inbox cuando el flujo lo requiera.
- En webhooks/eventos se prefiere persist-first + procesamiento asíncrono/idempotente.

### E6. Eventos y contratos internos

- Todo evento operativo debe tener producer, consumer, schema, semántica e idempotencia documentadas.
- Eventual consistency debe quedar explícita.
- Los workers deterministas y AI-assisted no pueden depender de payloads implícitos ni ambiguos.

### E7. Seguridad por defecto

- JWT/API keys/internal keys con validación estricta.
- SSRF/egress protection obligatoria.
- Secret injection solo en runtime, nunca persistir secretos planos.
- Security headers, body limits, brute-force protection, rate limiting y webhook verification donde corresponda.

### E8. Observabilidad completa

- Logs estructurados con request/trace/correlation IDs.
- Métricas Prometheus por servicio con namespaces estables.
- Trazas o correlación equivalente en flujos cross-service.
- SLO/SLI y alert rules para lo crítico.

### E9. Resiliencia operativa

- Retries con backoff y límites explícitos.
- DLQ o dead-letter equivalente para consumidores/eventos críticos.
- Circuit breakers donde haya upstreams externos o internos frágiles.
- Timeouts por contexto y budgets definidos.

### E10. Testing por capas

- Unit tests para lógica no trivial.
- Integration tests para repositorios, contracts y adapters.
- E2E para flujos cross-service.
- Contract tests para OpenAPI/eventos/SDKs cuando aplique.
- Load/smoke/security tests en la salida a producción.

### E11. Versionado y compatibilidad

- APIs públicas versionadas bajo `/v1`.
- Deprecaciones con headers, OpenAPI y comunicación.
- SDKs y contratos deben seguir la política de versionado del repo.
- No romper dashboards, alertas ni automaciones externas por renames silenciosos.

### E12. Configuración y secrets

- Config validada al startup.
- Diferencias entre ambientes via env/config externa, no forks de lógica.
- Secrets via env/secrets manager, no hardcodeados.

### E13. Prompting runtime gobernado

- Los prompts runtime del subsistema AI deben vivir externalizados o claramente versionados.
- Deben tener fallback determinista, rate limit, observabilidad y evaluación.
- No se aceptan prompts inline invisibles como estrategia final del producto.

### E14. Documentation as code

- Todo cambio relevante debe reflejarse en docs/prompts/runbooks/contracts.
- Si el código ya existe pero la doc maestra no lo refleja, hay drift y debe corregirse.

### E15. Alcance obligatorio explícito

- Cada prompt de esta suite debe dejar explícito que su alcance es requerido.
- El orden propuesto de implementación no reduce el alcance final.

---

## Reglas de implementación

### Arquitectura
- Diseñar por bounded context y ownership claro.
- Preferir adapters/ports en Go y separación clara entre API, usecases, repository, integrations.
- En Python AI, separar API, domain, services, adapters, metrics/logging.

### Contratos
- Si un prompt agrega endpoints o eventos, debe mencionar:
  - servicio owner
  - auth requerida
  - payload/DTO
  - errores esperados
  - impacto en OpenAPI/contracts/SDKs

### Seguridad
- Todo webhook debe verificar firma y ser idempotente.
- Todo endpoint interno debe requerir internal key o auth equivalente.
- No exponer `/metrics` ni endpoints internos sin decisión explícita.

### Observabilidad
- Cada feature crítica debe agregar logs, métricas y criterios de alerta.
- Toda automatización debe dejar evidencia operativa consultable.

### Operación
- Si un cambio afecta incidentes, rollback, dunning, retention, billing o supervisión, debe impactar también docs/runbooks/launch checklist.

### Frontend
- Tower sigue siendo cliente de APIs. No duplicar validaciones críticas del backend.
- Permisos/feature visibility derivados de APIs/auth real, no hardcodeados.

### SDKs
- Si cambia el contrato público, evaluar impacto en `sdks/python-sdk`, `sdks/typescript-sdk` y `sdks/go-sdk`.

---

## Criterios de éxito transversales

- [ ] La suite completa refleja fielmente la arquitectura real de `nexus`
- [ ] Cada prompt respeta los boundaries entre `core`, `saas`, `control-operators`, `ai-operators` y `tower`
- [ ] No hay prompts que pongan LLM en el pipeline de enforcement
- [ ] Los contratos públicos e internos quedan identificados y alineados con `pkgs/contracts`
- [ ] Errores, seguridad, observabilidad, testing y release gates aparecen como requisitos explícitos
- [ ] El alcance obligatorio queda claro en toda la suite
- [ ] La secuencia propuesta de implementación responde a dependencias técnicas, no a recorte de alcance

---

## Orden recomendado de trabajo sobre la suite

**Aclaración importante**: este orden es solo técnico. Todo el contenido de la suite sigue siendo obligatorio.

1. `00_base_transversal.md`
2. `01_user_identity_clerk_aws.md`
3. `02_billing_stripe.md`
4. `03_admin_console_ui.md`
5. `04_email_notifications_ses.md`
6. `05_developer_experience_cicd.md`
7. `06_prod_infrastructure_terraform.md`
8. `07_security_hardening.md`
9. `08_monitoring_observability.md`
10. `09_final_polish_launch.md`
11. `10_production_hardening_final.md`
12. `11_ai_runtime_prompting_eval.md`
13. `12_policy_dsl_mcp_a2a_contracts.md`
14. `13_data_model_events_ownership.md`
15. `14_incident_response_oncall.md`
16. `15_engineering_onboarding_contributing.md`
17. `16_test_strategy_release_gates.md`
18. `17_architecture_decision_records.md`
