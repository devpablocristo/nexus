# Suite de prompts de Nexus

Suite maestra de prompts/documentos guía para implementar y evolucionar `nexus` sin romper arquitectura, boundaries ni contratos.

## Orden oficial

| Prompt | Propósito |
|--------|-----------|
| `00_base_transversal.md` | Base transversal: arquitectura, calidad, límites y estándares |
| `01_user_identity_clerk_aws.md` | Identidad, Clerk, JWT/OIDC, sincronización de usuarios y membresías |
| `02_billing_stripe.md` | Facturación, suscripciones, dunning y límites por plan |
| `03_admin_console_ui.md` | Admin console y UX de gestión/supervisión |
| `04_email_notifications_ses.md` | Notificaciones por email e in-app |
| `05_developer_experience_cicd.md` | OpenAPI, SDKs, portal para desarrolladores y CI/CD |
| `06_prod_infrastructure_terraform.md` | Terraform, AWS, networking, ECS, RDS, DR |
| `07_security_hardening.md` | Hardening de seguridad, auth, headers, limits |
| `08_monitoring_observability.md` | Métricas, dashboards, alerting y SLO/SLI |
| `09_final_polish_launch.md` | Preparación de lanzamiento, tenant lifecycle y smoke/load tests |
| `10_production_hardening_final.md` | Cierre de gaps finales productivos |
| `11_ai_runtime_prompting_eval.md` | Prompting runtime, evaluaciones, guardrails y fallback de IA |
| `12_policy_dsl_mcp_a2a_contracts.md` | Policy DSL, contratos y protocolos MCP/A2A |
| `13_data_model_events_ownership.md` | Ownership de datos, modelo de datos y catálogo narrativo de eventos |
| `14_incident_response_oncall.md` | Respuesta a incidentes, severidades, escalado y postmortems |
| `15_engineering_onboarding_contributing.md` | Onboarding técnico, flujo de trabajo de ingeniería y contributing |
| `16_test_strategy_release_gates.md` | Estrategia de testing y gates de release por servicio |
| `17_architecture_decision_records.md` | ADRs y registro permanente de decisiones arquitectónicas |

## Reglas de lectura

- Empezar siempre por `00_base_transversal.md`.
- Todo prompt posterior hereda los estándares y límites definidos en `00`.
- Ningún prompt debe leerse como opcional salvo indicación explícita.
- Si el código del repo contradice un prompt, hay que resolver el drift y actualizar la suite.
