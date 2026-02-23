# Sim Engine Regression and Determinism

## Test layers

## Unit

- `sim-engine/internal/sim/engine_test.go`
  - deterministic hash with same seed + same moves
- `sim-engine/internal/http/auth_test.go`
  - internal key auth middleware
- `nexus-core/pkg/utils/ssrf_test.go`
  - SSRF allowlist behavior
- `nexus-core/internal/gateway/*sim_engine*test.go`
  - request-id + internal key propagation
  - allowlisted host pass / private host blocked
- `nexus-core/internal/gateway/world_enforcement_events_test.go`
  - Core emits `tool.denied` / `tool.rate_limited` to world feed via sim-engine internal endpoint
- `nexus-core/internal/world/*test.go`
  - `/v1/world/*` scope checks + upstream mapping

## Integration (expected in compose env)

- Core -> Sim Engine allowlisted call succeeds.
- Core blocks non-allowlisted private destination.
- Missing `X-Sim-Engine-Internal-Key` rejected by sim-engine.
- Policy denied and rate-limited paths through `/v1/run`.

## Determinism workflow

1. Seed and run demo:

```bash
bash scripts/seed_sim_engine_demo.sh
make demo-doorjam
```

2. Generated artifacts:

- `golden_runs/door_jam/baseline.jsonl`
- `golden_runs/door_jam/checksums.json`

Notes:

- `scripts/seed_sim_engine_demo.sh` resets Redis buckets before seeding so rate-limit state from prior runs does not leak into regression runs.
- `baseline.jsonl` is generated from deterministic scenario steps (`step_id <= demo steps`) and excludes post-step observe burst events used only to force overlay/rate-limit signals.

3. Replay check:

```bash
make replay RUN_ID=<run_id>
```

4. Validate:

- replay state hash equals latest state hash for same run
- baseline/coordinated key step hashes are stable for same seed/events

## CI

`sim-engine` has dedicated CI job in `.github/workflows/ci.yml`:

- `go vet ./...`
- `go test ./...`
- docker build verification

And e2e job runs:

- `make demo-doorjam` (Door Jam + replay determinism regression)
