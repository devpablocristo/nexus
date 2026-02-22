# WorldSim Ops Runbook

## Local bootstrap

```bash
cp .env.example .env
make up
make migrate-up
make migrate-worldsim
make seed
bash scripts/seed_worldsim_demo.sh
```

## Health checks

- Core ready: `GET /readyz`
- WorldSim health: `GET /healthz` (internal service)
- World data plane: `GET /v1/world/runs` through Core

## Key env vars

- `NEXUS_EGRESS_ALLOWLIST` (must include `world-sim:8087`)
- `NEXUS_WORLDSIM_BASE_URL` (default `http://world-sim:8087`)
- `NEXUS_WORLDSIM_INTERNAL_KEY` (shared secret header validation)
- `WORLDSIM_DATABASE_URL`

## Expected behavior checks

1. `world.observe` / `world.move` tools exist for demo org.
2. Egress rule `world-sim` exists on both tools.
3. `world.move` has allow policy (write tools are default-deny without allow policy).
4. Core can call world-sim only through allowlisted host:port.
5. Missing/invalid `X-WorldSim-Internal-Key` returns 403 from world-sim.

## Troubleshooting

### `EGRESS_DENIED` for world tools

- Verify `NEXUS_EGRESS_ALLOWLIST` contains `world-sim:8087`.
- Verify tool URL is exactly `http://world-sim:8087/...`.
- Verify egress rule for host `world-sim` exists on tool.

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
  - policy/rate from audit stream
  - collision/loop from world events
- Useful dashboards:
  - tool latency/success
  - policy denied count
  - rate limited count
  - collision/loop rates
