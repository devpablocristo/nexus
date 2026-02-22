# WorldSim Regression and Determinism

## Test layers

## Unit

- `world-sim/internal/sim/engine_test.go`
  - deterministic hash with same seed + same moves
- `world-sim/internal/http/auth_test.go`
  - internal key auth middleware
- `nexus-core/pkg/utils/ssrf_test.go`
  - SSRF allowlist behavior
- `nexus-core/internal/gateway/*worldsim*test.go`
  - request-id + internal key propagation
  - allowlisted host pass / private host blocked
- `nexus-core/internal/world/*test.go`
  - `/v1/world/*` scope checks + upstream mapping

## Integration (expected in compose env)

- Core -> WorldSim allowlisted call succeeds.
- Core blocks non-allowlisted private destination.
- Missing `X-WorldSim-Internal-Key` rejected by world-sim.
- Policy denied and rate-limited paths through `/v1/run`.

## Determinism workflow

1. Seed and run demo:

```bash
bash scripts/seed_worldsim_demo.sh
make demo-doorjam
```

2. Generated artifacts:

- `golden_runs/door_jam/baseline.jsonl`
- `golden_runs/door_jam/checksums.json`

3. Replay check:

```bash
make replay RUN_ID=<run_id>
```

4. Validate:

- replay state hash equals latest state hash for same run
- baseline/coordinated key step hashes are stable for same seed/events

## CI

`world-sim` has dedicated CI job in `.github/workflows/ci.yml`:

- `go vet ./...`
- `go test ./...`
- docker build verification
