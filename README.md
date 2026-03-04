# Nexus Monorepo

Nexus is an **agent-operated execution control plane** — the governance layer between AI agents and real-world tools. It decides what gets executed, with what limits, and keeps an immutable audit trail.

## Architecture

- **nexus-core**: deterministic gateway/data-plane — run/simulate, policies, DLP, egress, idempotency, audit.
- **nexus-control-operators**: deterministic control-plane workers (Go) — sentry, coordinator, mitigation, recovery.
- **nexus-ai-operators**: AI-operated service (Python) — risk scoring, policy proposals, assistant-facing intelligence.
- **nexus-tower**: supervision UI — overview, run explorer, timeline, policies, approvals, alerts, sessions, ask-agent, exports.
- **sdks/**: Python (sync + async) and TypeScript SDKs with LangChain and OpenAI Agents integrations.

## Core Features

| Feature | Description |
|---------|-------------|
| **Policy DSL** | Conditions + effects (allow/deny) with first-match priority evaluation |
| **DLP** | Automatic PII detection (email, phone, credit card, JWT, API key) |
| **Rate Limiting** | Per-tenant and per-tool, in-memory or Redis backend |
| **Circuit Breaker** | Per-host upstream protection (configurable threshold, half-open, reset) |
| **Idempotency** | Replay, conflict, in-progress detection for WRITE tools |
| **Egress/SSRF** | Host allowlist per tool, blocks private IPs, metadata endpoints |
| **Timeout Budgets** | Per-stage time tracking, budget exhaustion before execution |
| **Schema Validation** | JSON Schema for tool input and output |
| **Secret Injection** | Encrypted vault, injected into upstream headers at execution time |
| **Human-in-the-Loop** | Policies can require approval before execution |
| **Alert Rules** | Webhook notifications when metrics breach thresholds |
| **Agent Sessions** | Track call counts, writes, and denials per agent session |
| **Audit Trail** | Hash-chain, DLP summary, export CSV/JSONL |
| **MCP / A2A** | Model Context Protocol and Agent-to-Agent endpoints |
| **OIDC/SSO** | OAuth2 + PKCE authentication flow |
| **Self-service Onboarding** | `POST /v1/orgs` creates org + API key |

## Monorepo Layout

```text
/nexus-core        Go backend (gateway/data plane)
/nexus-control-operators  Dedicated deterministic control-plane service (Go workers image)
/nexus-ai-operators  Python AI operators service
/nexus-tower       React supervision UI
/sdks              Python SDK + TypeScript SDK
/shared            Contracts (OpenAPI, event schemas, error codes)
/docs              Architecture and operations docs
/scripts           Demo, seed, e2e scripts
/Makefile          Root orchestration
/docker-compose.yml
```

## Quickstart

```bash
cp .env.example .env
make up
make migrate-up
make seed
```

`make seed` prints `NEXUS_DEMO_API_KEY` and `NEXUS_OPERATOR_API_KEY`.

## URLs

| Service | URL |
|---------|-----|
| Nexus Core API | http://localhost:8080 |
| API Docs | http://localhost:8080/docs |
| Tower UI | http://localhost:5174 |
| External Operators API | http://localhost:8000 |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3000 |

## SDKs

### Python

```bash
pip install -e sdks/python-sdk
```

```python
from nexus_sdk import NexusClient

client = NexusClient(base_url="http://localhost:8080", api_key="nxk_...")
resp = client.run("echo", input={"hello": "world"})
```

**LangChain integration:**
```python
from nexus_sdk.integrations.langchain import NexusToolkit
tools = NexusToolkit(client).get_tools()
```

**OpenAI Agents integration:**
```python
from nexus_sdk.integrations.openai_agents import nexus_function_tools
tools = nexus_function_tools(client)
```

### TypeScript

```typescript
import { NexusClient } from 'nexus-sdk';
const client = new NexusClient({ baseUrl: 'http://localhost:8080', apiKey: 'nxk_...' });
const resp = await client.run('echo', { hello: 'world' });
```

## Commands

| Command | Description |
|---------|-------------|
| `make up` / `make down` | Start / stop all services |
| `make migrate-up` | Run database migrations |
| `make seed` | Seed demo org, tools, policies |
| `make core-test` | Run Go tests |
| `make qa` | Run all tests (core + operator + tower) |
| `make e2e` | End-to-end integration tests |
| `make jwt-e2e` | JWT authentication e2e tests |

## Contracts

Shared contracts under `shared/`:
- `shared/contracts/openapi.nexus-core.snapshot.yaml`
- `shared/contracts/events.schema.json`
- `shared/contracts/world-events.schema.json`
- `shared/contracts/error-codes.json`

## Agent-Operated Model

- External operators **never** write to DB directly.
- External operators consume `GET /v1/events` and act through Nexus APIs (`/v1/actions/*`, `/v1/incidents`, `/v1/policy-proposals`).
- Control operators run deterministic workers from `nexus-control-operators/cmd/ops-workers` and react via internal event-store workflows.
- HITL: policies with `require_approval` block execution until a human approves/rejects.
- Tower does not call LLM directly — routes through `nexus-core /v1/assistant/query`.

## Docs

- [`docs/DOC.md`](docs/DOC.md) — Full technical reference (pipeline, endpoints, directory structure, SDKs).
- [`docs/AGENT_OPERATED_MODEL.md`](docs/AGENT_OPERATED_MODEL.md) — Agent-operated model and HITL flow.
- [`docs/NAMING_AND_BOUNDARIES.md`](docs/NAMING_AND_BOUNDARIES.md) — Names, headers, compatibility.
