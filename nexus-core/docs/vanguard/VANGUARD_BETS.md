# VANGUARD_BETS.md

Nexus toma exactamente 2 apuestas vanguardistas para 2026-2030:

1. **B1 — Cryptographic audit receipts verificables externamente**
2. **B4 — Interoperabilidad MCP + A2A sin bypass**

---

## BET 1: Cryptographic Audit Receipts (B1)

### A) Why now / why us

- Mercado: procurement enterprise pide evidencia verificable, no solo logs internos.
- Técnica: Nexus ya tiene hash-chain en auditoría; el salto a receipts firmados es natural.
- Diferencial: verificabilidad por terceros (auditor/SIEM) con endpoint y CLI dedicados.
- Verificable: sí (firma + verificación offline/online).

### B) Arquitectura concreta

- Nuevos componentes:
  - `internal/receipts/` (issuer/verifier)
  - `cmd/verify-receipt/` (CLI)
- Storage:
  - `audit_events`: agregar `receipt_sig`, `receipt_key_id`, `receipt_payload_hash`
- Endpoints:
  - `GET /v1/audit/receipts/:event_hash`
  - `POST /v1/audit/receipts/verify`
- Consola:
  - panel “Receipt Verify” en `/admin`
- Threat impact:
  - reduce repudiation y tampering disputes
- Failure/rollback:
  - modo dual-write (hash-chain + receipt opcional)
  - fallback a hash-chain local si firma falla

### C) MVP 4-6 semanas

- Semana 1: esquema + issuer (firma por evento)
- Semana 2: endpoint fetch + verify API
- Semana 3: CLI verify
- Semana 4: dashboard panel + export con receipt
- Semana 5-6: hardening + performance tests

DoD:
- tests unit + integration + e2e receipt verify
- métrica `nexus_receipt_issued_total`, `nexus_receipt_verify_fail_total`
- demo reproducible en 20 min

### D) Demo 20 min

1. Ejecutar run de tool.
2. Obtener `event_hash` desde `/v1/audit`.
3. Consultar receipt firmado.
4. Verificar receipt con CLI y con endpoint `/verify`.
5. Alterar payload localmente y demostrar verificación fallida.

### E) Operación

- Alertas: ratio de fallos de firma/verificación.
- Runbook: rotación de claves de firma + revocación.
- Coste/performance: overhead bajo por firma por evento (medir p95).

---

## BET 2: MCP + A2A Bridge sin bypass (B4)

### A) Why now / why us

- Mercado: multi-agent orchestration convergiendo en protocolos interoperables.
- Técnica: Nexus ya centraliza control en `gateway.Run`; bridge puede reutilizar pipeline exacto.
- Diferencial: misma política/egress/limits/audit para MCP y A2A.
- Verificable: sí (mismo input => mismo enforcement/result).

### B) Arquitectura concreta

- Nuevos componentes:
  - `internal/a2a/handler.go`
  - `internal/a2a/usecases.go`
- Endpoint:
  - `POST /a2a` (mapping A2A request -> `gwdomain.RunRequest`)
- Reuso obligatorio:
  - `a2a.CallTool` delega en `gateway.Run` (sin executor alterno)
- Consola:
  - panel “A2A demo call”
- Threat impact:
  - evita bypass por canal nuevo
- Failure/rollback:
  - feature flag `NEXUS_ENABLE_A2A=false` para rollout canary

### C) MVP 4-6 semanas

- Semana 1: envelope DTO + mapping básico A2A->Run
- Semana 2: endpoint + tests parity MCP/REST/A2A
- Semana 3: authz scopes `a2a:read`, `a2a:call`
- Semana 4: docs + demo + observabilidad
- Semana 5-6: pruebas de interoperabilidad con SDKs/clientes

DoD:
- tests de paridad de enforcement (MCP vs A2A)
- métrica `nexus_protocol_requests_total{protocol=...}`
- demo reproducible 20 min

### D) Demo 20 min

1. Crear request equivalente por REST y por MCP.
2. Ejecutar mismo request por A2A bridge.
3. Mostrar misma decisión policy + mismo bloqueo egress/rate-limit.
4. Comparar eventos de auditoría por protocolo.

### E) Operación

- Alertas por error rate A2A.
- Runbook de versionado de envelope A2A.
- Coste/performance: parse/mapping overhead mínimo; medir p95.

