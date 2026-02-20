# INCIDENTS_P1_P2.md

## Severidades

- **P1**: caida total, impacto alto cliente, riesgo de seguridad activo.
- **P2**: degradacion parcial significativa sin caida total.

## Triage inicial (0-10 min)

1. Confirmar alcance:
```bash
curl -i http://localhost:${NEXUS_HTTP_PORT}/healthz
curl -i http://localhost:${NEXUS_HTTP_PORT}/readyz
```
2. Revisar logs:
```bash
docker compose logs --tail=300 nexus-core
```
3. Revisar DB/Redis:
```bash
docker compose ps
```

## Playbook P1

1. Declarar incidente y abrir bridge.
2. Activar mitigacion inmediata:
- fallback rate limit (`memory`) solo si Redis degradado y single-instance controlado.
- bloquear features no criticas si afectan estabilidad.
3. Ejecutar checks:
```bash
curl -sS http://localhost:${NEXUS_HTTP_PORT}/metrics | grep -E "nexus_run_total_prom|nexus_gateway_req_count"
```
4. Si data corruption sospechada: freeze writes + backup + restore plan.
5. Comunicar cada 15 min estado y ETA.

## Playbook P2

1. Identificar componente degradado (egress, upstream, policy errors).
2. Mitigar con config segura y reversible.
3. Abrir ticket de causa raiz (RCA) con owner y deadline.

## Seguridad (aplica P1/P2)

- Si hay sospecha de abuso de credenciales:
  - rotar API keys comprometidas
  - revisar `audit_events` y `admin_activity_events`
  - reforzar scopes temporales

## Cierre

- Postmortem en < 48h
- Acciones correctivas con fecha
- Validacion de runbook actualizado

