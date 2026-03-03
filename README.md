# Nexus Monorepo

Nexus is an **agent-operated execution control plane** — the governance layer between AI agents and real-world tools. It decides what gets executed, with what limits, and keeps an immutable audit trail.

## Architecture

- **nexus-core**: deterministic backend — gateway, policies, DLP, rate-limits, audit, approvals, alerts, sessions.
- **nexus-operator**: AI-operated service — signals, risk scoring, reversible actions, policy proposals.
- **nexus-tower**: supervision UI — overview, run explorer, timeline, policies, approvals, alerts, sessions, ask-agent, exports.
- **sdks/**: Python and TypeScript SDKs with LangChain and OpenAI Agents integrations.

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
/nexus-core        Go backend (gateway + control plane + audit)
/nexus-operator    Python AI-operated service
/nexus-tower       React supervision UI
/sim-engine        Go simulation engine (Door Jam)
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
make migrate-sim-engine
make seed
```

`make seed` prints `NEXUS_DEMO_API_KEY` and `NEXUS_OPERATOR_API_KEY`.

## URLs

| Service | URL |
|---------|-----|
| Nexus Core API | http://localhost:8080 |
| API Docs | http://localhost:8080/docs |
| Tower UI | http://localhost:5174 |
| Operator API | http://localhost:8000 |
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
| `make demo-doorjam` | Run Door Jam simulation demo |

## Contracts

Shared contracts under `shared/`:
- `shared/contracts/openapi.nexus-core.snapshot.yaml`
- `shared/contracts/events.schema.json`
- `shared/contracts/error-codes.json`

## Agent-Operated Model

- Operator **never** writes to DB directly.
- Operator consumes `GET /v1/events` and acts through: `POST /v1/actions/apply`, `POST /v1/incidents`, `POST /v1/policy-proposals`.
- Tower does not call LLM directly — routes through `nexus-core /v1/assistant/query`.
