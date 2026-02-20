# DEMO_TECH_PLAYBOOK.md

## Objetivo

Demo tecnica de 20 min mostrando control plane para agentes con REST + MCP + admin console.

## Setup (clean install)

```bash
cp .env.example .env
bash scripts/quickstart_admin.sh
```

## Guion (20 min)

1. **Arquitectura (2 min)**
- mostrar `/docs` y `docs/ARCHITECTURE.md`

2. **Admin bootstrap (3 min)**
- abrir `http://localhost:8080/admin`
- cargar API key del output de quickstart
- ejecutar `Load Bootstrap`

3. **Tool execution REST (4 min)**
- panel REST Run Demo -> ejecutar `echo`
- mostrar decision/status/latency

4. **Tool execution MCP (4 min)**
- panel MCP -> `tools/list`
- explicar que no hay bypass: usa el mismo pipeline

5. **Security controls (4 min)**
- egress default deny + DLP/policy + secrets
- mostrar bloqueos en audit

6. **Observabilidad (3 min)**
- `http://localhost:3000` dashboard `Nexus Starter Overview`
- métricas de throughput/latencia/blocked

## Mensajes clave

- onboarding < 1h
- gobierno de seguridad sin frenar agentes
- evidencia auditable para compliance

