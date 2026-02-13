# Nexus Gateway (Phase 2)

Nexus Gateway is a multi-tenant Agent Tool Gateway for Platform/SecOps teams: agents call Nexus (not internal APIs directly), Nexus enforces authn/authz, schema, policies, DLP/egress controls, executes tools, and writes append-only redacted audit events.

## Core architecture

```
Agent / MCP Client
        |
   REST + MCP (Gin)
        |
     Handlers
        |
     Usecases (ports)
   /      |        \
Repos  Executors   Security adapters
(DB)   (HTTP)      (Secrets, Egress, DLP)
```

- `internal/<module>` contains domain modules (`org`, `tool`, `policy`, `audit`, `gateway`, `mcp`, `secrets`, `egress`, `dlp`).
- `pkg/` stays domain-agnostic.
- Usecases do not import Gin/GORM.

## Quickstart

```bash
cp .env.example .env
make dev
make migrate-up
make seed
```

Docs:
- Swagger UI: `http://localhost:8080/docs`
- OpenAPI: `http://localhost:8080/openapi.yaml`
- Architecture notes: `docs/ARCHITECTURE.md`

## Product packaging status

- OpenAPI contract completed in `docs/openapi.yaml` (headers, request/response schemas, error model, MCP envelope, audit export formats).
- CI now runs `unit` + `smoke` + `qa` + `jwt-e2e` on every push/PR.
- Contract tests for audit export are in `internal/audit/export_contract_test.go` with golden snapshots under `internal/audit/testdata/`.
- Idempotency `FAILED` semantics are terminal and replay cached error for same key+fingerprint.

## Authentication & delegation

Required header:
- `X-NEXUS-GATEWAY-KEY`

Optional delegation headers:
- `X-NEXUS-ACTOR`
- `X-NEXUS-ROLE`
- `X-NEXUS-SCOPES` (comma-separated, intersected with key scopes)
- `Authorization: Bearer <JWT>` (when `NEXUS_AUTH_ENABLE_JWT=true`)

API keys are stored hashed (`SHA-256`) and resolved to tenant `org_id`.
JWT/JWKS mode supports strong identity while keeping API key compatibility via flags:
- `NEXUS_AUTH_ENABLE_JWT`
- `NEXUS_AUTH_ALLOW_API_KEY`

## Phase 2 capabilities

- MCP server endpoint: `POST /mcp` (`tools/list`, `tools/get`, `tools/call`)
- Policy simulator/explain endpoint: `POST /v1/run/simulate` (no upstream execution)
- Secrets vault per tool/org (AES-GCM encrypted at rest)
- Secret injection into outbound tool calls (`header` / `bearer`)
- Delegation-aware policy context (`context.actor`, `context.role`, `context.scopes`)
- Egress allowlist rules per tool (`host` allowlist)
- DLP summary in runtime context (`context.dlp.*.count`) + persisted in audit (`dlp_summary`)
- JWT + JWKS authentication mode (issuer/audience/claims configurable)
- Redis distributed rate limiting (`NEXUS_RATE_LIMIT_BACKEND=redis`)
- OpenTelemetry basics (HTTP traces + run counters/latency histograms)
- Audit tamper-evident hash-chain (`prev_event_hash` + `event_hash`)
- Idempotency for WRITE tools via `Idempotency-Key` (replay/conflict/in-progress semantics)
- End-to-end timeout budget via `X-Timeout-Ms` (bounded + audited stage durations)
- SIEM audit export endpoint: `GET /v1/audit/export?format=jsonl|csv`
- Tool sensitivity level (`low|medium|high`) available for policy matching (`tool.sensitivity`)

## Idempotency (final semantics)

- Applies to WRITE tool runs (`POST /v1/run` with `Idempotency-Key`).
- `FAILED` is terminal per key+fingerprint: the same call returns the same cached error response (replay), without re-executing upstream.
- To retry after `FAILED`, clients must use a **new** `Idempotency-Key`.
- `IDEMPOTENCY_IN_PROGRESS` and `IDEMPOTENCY_KEY_CONFLICT` remain `409`.

## Security policy examples

Deny exfiltration to external tools if credit cards are present:

```json
{
  "all": [
    { "path": "context.dlp.credit_card.count", "op": "gt", "value": 0 },
    { "path": "tool.classification", "op": "eq", "value": "external" }
  ]
}
```

Allow only SecOps actor role for sensitive operations:

```json
{
  "path": "context.role",
  "op": "eq",
  "value": "secops"
}
```

## cURL examples

Export key after `make seed`:

```bash
export NEXUS_API_KEY="<printed-by-seed>"
```

Run tool via REST:

```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"echo","input":{"hello":"world"},"context":{"user_id":"u_1"}}' \
  http://localhost:8080/v1/run | jq
```

MCP tools/list:

```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' \
  http://localhost:8080/mcp | jq
```

Create encrypted secret for `echo` tool (requires `X-NEXUS-ROLE: secops` or `admin:secrets` scope):

```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "X-NEXUS-ROLE: secops" \
  -H "Content-Type: application/json" \
  -d '{"secret_type":"header","key_name":"X-Injected-Token","value":"top-secret","enabled":true}' \
  http://localhost:8080/v1/tools/echo/secrets | jq
```

Set egress allowlist:

```bash
curl -sS -o /dev/null -w "%{http_code}\n" \
  -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"host":"mock-tools","enabled":true}' \
  http://localhost:8080/v1/tools/transfer/egress-rules
```

Simulate decision/explain (dry-run):

```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"transfer","input":{"amount":1500,"card_number":"4111111111111111"},"context":{"user_id":"u_1"}}' \
  http://localhost:8080/v1/run/simulate | jq
```

## QA commands

- `make test`: unit/integration tests
- `make e2e`: full API e2e (REST + MCP + secrets + egress + DLP checks)
- `make jwt-e2e`: JWT/JWKS e2e (`Authorization: Bearer`, API key fallback off)
- `make qa`: full pipeline (`down` → `up` → `migrate` → `seed` → `test` → `e2e`)
- `make cleanup-idempotency`: delete expired idempotency records
- `go test ./internal/audit -run TestAuditExport`: contract/snapshot export tests

## Operations Runbook (Pilot)

- **Health checks**: `GET /healthz` (liveness), `GET /readyz` (DB readiness).
- **Idempotency cleanup**:
  - TTL controlled by `NEXUS_IDEMPOTENCY_TTL_HOURS` (default 24h).
  - Run manually: `make cleanup-idempotency` (executes `cmd/cleanup-idempotency` in container).
- **What to monitor**:
  - OTel metrics: `nexus_run_total` (slice by `status`, `decision`, `tool_name`) and `nexus_run_latency_ms`.
  - Alerting slices:
    - blocked: `status=blocked` (policy/rate/egress denies)
    - timeout: `status=error` + logs/audit `TIMEOUT` or `TIMEOUT_BUDGET_EXCEEDED`
    - upstream errors: `status=error` + logs/audit `UPSTREAM_5XX`/`NETWORK_ERROR`
- **Audit retention (pilot recommendation)**: keep at least 30 days online in Postgres; export daily to SIEM (`/v1/audit/export?format=jsonl`).
- **If Redis rate-limit backend degrades**: switch to in-memory backend (`NEXUS_RATE_LIMIT_BACKEND=memory`) as temporary single-instance fallback and monitor `RATE_LIMITED` behavior.
- **SIEM export**:
  - JSONL: `curl -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" "http://localhost:8080/v1/audit/export?format=jsonl&from=2026-01-01T00:00:00Z"`
  - CSV: `curl -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" "http://localhost:8080/v1/audit/export?format=csv&limit=2000"`
- **Egress default-deny**: Tools with no egress rules cannot make outbound calls. Every tool must have at least one allowed host via `POST /v1/tools/{name}/egress-rules`. This is by design to prevent uncontrolled data exfiltration.
- **SSRF protection**: Outbound HTTP calls are protected at the transport layer:
  - Safe `DialContext` validates resolved IPs at connection time (blocks DNS rebinding).
  - Redirects are disabled (blocks redirect-based SSRF).
  - Blocked: loopback, private ranges, link-local, IPv6 ULA (`fc00::/7`), cloud metadata (`169.254.169.254`).
  - `DisableSSRFProtection` config flag exists for test environments only; a WARN log is emitted at startup if set.

## 5-Minute Demo (Copy/Paste)

Also available as `scripts/demo.sh`.

1) Start stack + seed:

```bash
cp .env.example .env
make dev
make migrate-up
make seed
```

2) Export API key from seed output:

```bash
export NEXUS_API_KEY="<seed-output-value>"
```

3) Allow egress to mock-tools host (default-deny blocks everything):

```bash
curl -sS -o /dev/null -w "%{http_code}" \
  -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"host":"mock-tools","enabled":true}' \
  http://localhost:8080/v1/tools/echo/egress-rules
# => 204

curl -sS -o /dev/null -w "%{http_code}" \
  -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"host":"mock-tools","enabled":true}' \
  http://localhost:8080/v1/tools/transfer/egress-rules
# => 204
```

4) Show DLP + external classification deny (credit card detected):

```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"transfer","input":{"amount":500,"card_number":"4111111111111111"},"context":{"user_id":"u_1"}}' \
  http://localhost:8080/v1/run | jq
# => 403, decision=deny, POLICY_DENIED
```

5) Run WRITE with idempotency + timeout:

```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Idempotency-Key: demo-transfer-001" \
  -H "X-Timeout-Ms: 10000" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"transfer","input":{"amount":500},"context":{"user_id":"u_1"}}' \
  http://localhost:8080/v1/run | jq
# => 200, idempotency.outcome=NEW
```

6) Replay with same idempotency key (no upstream re-execution):

```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Idempotency-Key: demo-transfer-001" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"transfer","input":{"amount":500},"context":{"user_id":"u_1"}}' \
  http://localhost:8080/v1/run | jq
# => 200, idempotency.outcome=REPLAY
```

7) Export audit trail with hash-chain:

```bash
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  "http://localhost:8080/v1/audit/export?format=jsonl&tool_name=transfer&limit=20"
# Each line has event_hash, prev_event_hash, hash_algo=sha256
```

8) (Optional) Verify SSRF protection blocks internal targets:

```bash
# Create a tool pointing to the cloud metadata endpoint
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"ssrf-test","kind":"http","method":"GET","url":"http://169.254.169.254/latest/meta-data/","input_schema":{"type":"object"},"action_type":"read","risk_level":5,"enabled":true}' \
  http://localhost:8080/v1/tools | jq

# Try to call it — blocked by SSRF protection before egress check
curl -sS -H "X-NEXUS-GATEWAY-KEY: $NEXUS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"tool_name":"ssrf-test","input":{}}' \
  http://localhost:8080/v1/run | jq
# => 403, reason="ssrf blocked: link-local address 169.254.169.254 not allowed"
```

## Threat model (brief)

Protects against:
- direct agent access to internal APIs bypassing policy
- plaintext secret leakage in DB/audit/logs
- outbound calls to non-approved hosts (egress)
- accidental exfiltration via simple PII/token detection and policy gates

Does not fully protect against:
- compromised tenant-provided API key
- advanced content-transform exfiltration and semantic PII evasions
- side-channel leakage in upstream systems outside Nexus control
