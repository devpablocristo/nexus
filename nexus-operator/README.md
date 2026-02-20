# nexus-operator

`nexus-operator` is the AI-operated service that automates L1 operational responses using Nexus Core APIs only.

## Responsibilities

- Consume `GET /v1/events` by cursor.
- Produce deterministic signals (deny ratio spikes).
- Apply temporary actions with TTL (`POST /v1/actions/apply`).
- Open incidents (`POST /v1/incidents`).
- Create policy proposals for human review (`POST /v1/policy-proposals`).
- Expose `POST /v1/assistant/query` for Nexus Core assistant proxy.

No direct DB access is allowed.

## Run

```bash
cp .env.example .env
pip install -e .[dev]
make run
```

## QA

```bash
make qa
```
