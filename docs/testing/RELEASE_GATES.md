# Release Gates

## Gates por categoría

| Cambio | Gates mínimos |
|--------|---------------|
| solo UI | tests frontend + build + revisión visual |
| contract/API | tests del servicio + contract tests + docs + SDK impact review |
| auth | tests de auth + e2e JWT/API key + security review |
| policy/enforcement | tests gateway/policy + contract tests + smoke |
| billing | tests SaaS + webhooks + docs/runbooks |
| operators | tests worker/runtime + event/schema compatibility |
| AI runtime | pytest + evals + fallback/guardrail verification |
| infra | terraform validate/plan + runbooks + smoke path |

## Contract gating

Si cambian:

- `pkgs/contracts/openapi.*`
- `pkgs/contracts/events.schema.json`
- `pkgs/contracts/error-codes.json`
- SDKs públicos

entonces el cambio no está listo sin actualizar docs y revisar compatibilidad.

## Evidencia mínima antes de release

- build/test del scope afectado
- contratos/snapshots alineados
- smoke relevante
- docs y rollback path actualizados
