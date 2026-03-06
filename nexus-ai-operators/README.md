# nexus-ai-operators

`nexus-ai-operators` is the AI runtime service for Nexus operators and the Tower assistant.

## Responsibilities

- Consume events through the internal operators bridge.
- Expose `POST /v1/assistant/query` with versioned prompts, tenant-aware context snapshots from `nexus-saas`, deterministic fallback, and guardrails.
- Run evals from `tests/evals/`.
- Apply temporary actions, open incidents, and create policy proposals via internal Nexus APIs only.

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
