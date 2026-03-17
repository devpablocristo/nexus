# Referencias externas para estudiar

Proyectos open source y recursos relevantes para el desarrollo de Nexus. No son dependencias — son fuentes de ideas, patterns, y componentes potenciales.

## Para construir el agente IA de Nexus (ai-runtime)

| Proyecto | Que es | Por que mirarlo | Link |
|----------|--------|----------------|------|
| Claude Agent SDK | SDK oficial de Anthropic para construir agentes con Claude | Loop de agente, tool use, contexto, guardrails built-in. Candidato principal para el ai-runtime | https://github.com/anthropics/claude-agent-sdk-python |
| OpenAI Agents SDK (Python) | SDK oficial de OpenAI para agentes | Guardrails, handoffs entre agentes, tracing | https://openai.github.io/openai-agents-python/ |
| OpenAI Agents Go | Port Go del SDK de agentes de OpenAI | Si preferimos Go para el ai-runtime en lugar de Python | https://github.com/nlpodyssey/openai-agents-go |

## Para governance y guardrails de agentes

| Proyecto | Que es | Por que mirarlo | Link |
|----------|--------|----------------|------|
| Galileo Agent Control | Control plane open source para gobernar agentes IA a escala (Apache 2.0) | Arquitectura de evaluators como plugins, runtime policy updates sin downtime, integracion con frameworks de agentes. Lanzado 11 marzo 2026 | https://github.com/agentcontrol/agent-control |
| Guardrails AI | Framework Python para validacion de inputs/outputs de LLMs | Patterns de validacion, risk detection, hub de guardrails reutilizables | https://github.com/guardrails-ai/guardrails |
| OpenAI Guardrails Python | Wrapper para agregar safety y compliance a apps con OpenAI | Input/output validation, moderation automatica | https://github.com/openai/openai-guardrails-python |
| Superagent | Framework para definir agentes con roles, permisos y guardrails | Constraints en config, enforcement en runtime, restriccion de API calls y data access | https://github.com/superagent-ai |

## Para billing y metering (ya evaluados, descartados para uso directo)

| Proyecto | Que es | Por que lo descartamos | Link |
|----------|--------|----------------------|------|
| LastSaaS | SaaS boilerplate Go+React con Stripe, multi-tenant, RBAC | Usa MongoDB (nosotros PostgreSQL). Arquitectura propia incompatible con hexagonal | https://github.com/jonradoff/lastsaas |
| GO-SAAS-KIT | SaaS starter kit Go con Stripe y multi-tenancy | Menos maduro que lo que ya tenemos en saas-core | https://go-saas.github.io/kit/ |
| Lago | Metering y usage-based billing open source | Util si necesitamos metering mas sofisticado que lo de saas-core | https://github.com/getlago/lago |

## Reportes y datos de mercado

| Recurso | Dato clave | Link |
|---------|-----------|------|
| Zenity 2026 Threat Landscape | 80% de orgs reportaron comportamientos riesgosos de agentes IA | https://zenity.io/resources/white-papers/2026-threat-landscape-report |
| CyberArk AI Agent Security | Solo 21% de ejecutivos tienen visibilidad completa de permisos de agentes | https://www.cyberark.com/resources/blog/whats-shaping-the-ai-agent-security-market-in-2026 |
| Help Net Security | Solo 47% de agentes IA estan monitoreados; mas de la mitad operan sin oversight | https://www.helpnetsecurity.com/2026/03/03/enterprise-ai-agent-security-2026/ |
| TRM Labs | Agentes autonomos y crimen financiero: trazar autoridad delegada de vuelta a humanos responsables | https://www.trmlabs.com/resources/blog/autonomous-ai-agents-and-financial-crime-risk-responsibility-and-accountability |
| PYMNTS Treasury AI Guardrails | US Treasury definiendo guardrails para AI en operaciones financieras | https://www.pymnts.com/artificial-intelligence-2/2026/regulators-aim-for-clearer-ai-guardrails-for-innovation-in-financial-operations/ |
| Crypto.com Autonomous Wallet | Wallets autonomas con autoridad delegada temporal para agentes | https://crypto.com/us/research/rise-of-autonomous-wallet-feb-2026 |
| Fintech Futures Banking 2026 | Banca entrando en produccion a escala de agentes IA | https://www.fintechfutures.com/ai-in-fintech/banking-in-2026-production-scale-ai-agents |
| Federal Register AI Agents RFI | Gobierno US pidiendo input sobre seguridad de agentes IA | https://www.federalregister.gov/documents/2026/01/08/2026-00206/request-for-information-regarding-security-considerations-for-artificial-intelligence-agents |

## Que estudiar primero

1. **Galileo Agent Control** — el mas relevante arquitectonicamente. Ver como definen evaluators, como hacen runtime updates, como integran con frameworks de agentes.
2. **Claude Agent SDK** — candidato para el ai-runtime. Ver loop de agente, tool use, como se definen tools, como se mantiene contexto.
3. **TRM Labs report** — el argumento regulatorio y de compliance para Nexus. "Trazar autoridad delegada de vuelta a humanos responsables" es exactamente lo que Nexus hace.

## Regla

Construir lo que nos diferencia (cascada, doble nexo, engine determinista, contexto financiero). Importar lo que no (loop de agente, tool use, guardrails genericos).
