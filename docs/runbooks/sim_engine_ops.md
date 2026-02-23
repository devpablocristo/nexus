# Sim Engine Ops Runbook

## Local bootstrap

```bash
cp .env.example .env
make up
make migrate-up
make migrate-sim-engine
make seed
bash scripts/seed_sim_engine_demo.sh
```

## Health checks

- Core ready: `GET /readyz`
- Sim Engine health: `GET /healthz` (internal service)
- World data plane: `GET /v1/world/runs` through Core

## Key env vars

- `NEXUS_EGRESS_ALLOWLIST` (must include `sim-engine:8087`)
- `NEXUS_SIM_ENGINE_BASE_URL` (default `http://sim-engine:8087`)
- `NEXUS_SIM_ENGINE_INTERNAL_KEY` (shared secret header validation)
- `SIM_ENGINE_DATABASE_URL`

## Expected behavior checks

1. `world.observe` / `world.move` tools exist for demo org.
2. Egress rule `sim-engine` exists on both tools.
3. `world.move` has allow policy (write tools are default-deny without allow policy).
4. Core can call sim-engine only through allowlisted host:port.
5. Missing/invalid `X-Sim-Engine-Internal-Key` returns 403 from sim-engine.

## Troubleshooting

### `EGRESS_DENIED` for world tools

- Verify `NEXUS_EGRESS_ALLOWLIST` contains `sim-engine:8087`.
- Verify tool URL is exactly `http://sim-engine:8087/...`.
- Verify egress rule for host `sim-engine` exists on tool.

### `UNAUTHORIZED` from `/v1/world/*`

- Read endpoints require `admin:console:read`.
- Write endpoints (`run/create`, `replay`) require `admin:console:write`.

### `world.move` always blocked by policy

- Check `world.move` policies and priority order.
- Ensure at least one `allow` rule matches.

### Replay hash mismatch

- Confirm same run_id, seed, config_hash.
- Confirm no out-of-band edits in `world_events`.
- Run `make replay RUN_ID=<id>` and compare with latest state hash.

## Observability hints

- Correlate with `request_id`.
- For overlays:
  - policy/rate/collision/loop from `GET /v1/world/events` (or `/v1/world/events/stream`)
  - no direct dependency on `/v1/audit` for Viewer overlays
- Useful dashboards:
  - tool latency/success
  - policy denied count
  - rate limited count
  - collision/loop rates
