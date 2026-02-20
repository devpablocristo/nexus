# nexus-tower

`nexus-tower` is the supervision UI for the agent-operated model.

## Screens

- Overview: event/action/incident posture.
- Timeline: append-only event feed drill-down.
- Policies: proposal diff + approve/reject/shadow.
- Ask Agent: query through `nexus-core` (`POST /v1/assistant/query`).
- Exports: compliance/audit export entry points.

## Run

```bash
cp .env.example .env
npm install
npm run dev
```

## QA

```bash
npm run qa
```
