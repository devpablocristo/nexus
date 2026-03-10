# Prompt 11 — Prompting runtime de IA, guardrails y evaluación

## Contexto del proyecto

`nexus-ai-operators` hoy existe y funciona, pero su prompting runtime todavía está demasiado implícito en código. Hay prompting inline en `app/api/routes.py`, backends LLM en `app/services/llm_client.py`, fallback determinista y DLQ, pero falta formalizar este subsistema como parte del producto.

Este prompt cubre:
- prompting runtime del assistant
- versionado de prompts
- evaluaciones
- guardrails
- fallback
- observabilidad del comportamiento LLM

**Prerequisito**: aplicar `docs/prompts/00_base_transversal.md`.

## Alcance obligatorio

Todo lo definido en este prompt es parte del alcance requerido del subsistema AI. El objetivo no es "mejorar un prompt", sino convertir el prompting runtime en un componente gobernado, observable y testeable.

---

## Lo que YA existe (no duplicar)

- `ai-runtime/app/api/routes.py` expone `/v1/assistant/query`.
- `ai-runtime/app/services/llm_client.py` soporta `anthropic`, `ollama` y `fallback`.
- Existe rate limiting de assistant y fallback determinista.
- Existen métricas, logging y dead-letter básicos en el servicio AI.
- Tower consume assistant vía `nexus-saas`, no habla directo con el LLM.

---

## Qué implementar

### 1. Prompt registry versionado

Crear un registry explícito de prompts runtime:

```text
ai-runtime/app/prompts/
├── assistant_system_v1.md
├── diagnosis_system_v1.md
├── comms_system_v1.md
├── executive_qa_system_v1.md
└── registry.py
```

Reglas:
- Los prompts viven fuera del handler.
- Cada prompt tiene `id`, `version`, `owner`, `purpose`.
- El código carga prompts por nombre/version, no por string inline.

### 2. Builder de contexto seguro

Crear un `prompt_context_builder.py` que:
- arme contexto mínimo necesario
- redacciones datos sensibles
- no exponga secretos, raw payloads ni PII innecesaria
- limite tamaño del contexto

### 3. Guardrails de salida

Definir y aplicar reglas de salida:
- respuestas estructuradas para Tower (`summary`, `tables`, `actions`)
- no prometer ejecuciones que no ocurrieron
- no inventar estados/acciones
- no sugerir bypass de enforcement
- no emitir acciones fuera de catálogos permitidos

### 4. Política de fallback

Formalizar fallback por backend:
- `anthropic` falla → fallback determinista
- `ollama` falla → fallback determinista
- backend no configurado → fallback determinista
- timeouts/5xx/invalid payload → fallback determinista

Debe quedar documentado qué se pierde y qué se preserva cuando entra el fallback.

### 5. Evaluación (evals)

Agregar carpeta de evals:

```text
ai-runtime/tests/evals/
├── assistant_cases.yaml
├── diagnosis_cases.yaml
└── test_prompt_evals.py
```

Casos mínimos:
- estado normal sin incidentes
- incidente abierto con resumen correcto
- operator state vacío o degradado
- backend LLM caído
- pregunta ambigua
- intento de pedir bypass o ejecución no permitida

### 6. Observabilidad específica de AI

Agregar métricas específicas:
- `nexus_ai_prompt_requests_total`
- `nexus_ai_prompt_fallback_total`
- `nexus_ai_prompt_latency_seconds`
- `nexus_ai_prompt_tokens_total` si el backend lo permite
- `nexus_ai_prompt_guardrail_violations_total`

Y logs con:
- `request_id`
- `org_id`
- `backend`
- `prompt_id`
- `prompt_version`
- `fallback_used`

### 7. Configuración

Agregar env/config para:
- prompt version default por flujo
- max context chars/tokens
- max output tokens
- eval mode
- sampling toggles de observabilidad

---

## Reglas de implementación

- No dejar prompts críticos inline en handlers.
- No filtrar secretos del runtime ni payloads sensibles al prompt.
- No permitir que el LLM produzca decisiones de enforcement.
- Tower sigue hablando con `nexus-saas`; no saltear el proxy existente.
- El assistant debe seguir funcionando aunque no haya backend LLM externo.

---

## Archivos a crear o modificar

### Crear
- `ai-runtime/app/prompts/*`
- `ai-runtime/app/services/prompt_registry.py`
- `ai-runtime/app/services/prompt_context_builder.py`
- `ai-runtime/tests/evals/*`

### Modificar
- `ai-runtime/app/api/routes.py`
- `ai-runtime/app/services/llm_client.py`
- `ai-runtime/app/core/config.py`
- `ai-runtime/app/core/metrics.py`
- `ai-runtime/README.md`

---

## Criterios de aceptación

- [ ] No quedan prompts runtime importantes hardcodeados inline en handlers
- [ ] Existe registry/versionado de prompts por flujo
- [ ] Existe builder de contexto con redacción y límites
- [ ] Hay fallback determinista explícito y probado
- [ ] Hay evals mínimas automatizadas
- [ ] Hay métricas y logs específicos del subsistema AI
- [ ] El assistant no puede sugerir bypass del pipeline determinista

---

## Orden de ejecución recomendado

1. Prompt registry + archivos de prompts
2. Context builder seguro
3. Refactor del assistant actual a prompts versionados
4. Guardrails de salida
5. Métricas y logs específicos
6. Evals automatizadas
7. README y documentación final
