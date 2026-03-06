# nexus-ai-operators

`nexus-ai-operators` is the AI runtime service for Nexus operators and the Tower assistant.

## Responsibilities

- Consume events through the internal operators bridge.
- Expose `POST /v1/assistant/query` with versioned prompts, safe context building, deterministic fallback, and guardrails.
- Run evals from `tests/evals/`.
- Apply temporary actions with TTL (`POST /v1/actions/apply`).
- Open incidents (`POST /v1/incidents`).
- Create policy proposals for human review (`POST /v1/policy-proposals`).

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
