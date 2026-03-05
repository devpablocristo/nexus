# nexus-control-operators

Deterministic control-plane service for Nexus. Runs four event-driven workers that consume operational events from `nexus-core` and react autonomously: detecting anomalies, coordinating incident state, applying mitigations, and verifying recovery.

## Architecture

```
nexus-core  ◄──HTTP──►  nexus-control-operators
  (event store API)         ├── sentry          (anomaly detection)
                            ├── coordinator     (incident state machine)
                            ├── mitigation      (auto-apply actions)
                            └── recovery        (post-mitigation monitoring)
```

All workers share one binary (`cmd/ops-workers`) and run as goroutines with independent consumer groups. Communication with `nexus-core` is via HTTP using the internal operator key.

## Workers

| Worker | Consumer Group | Responsibility |
|--------|---------------|----------------|
| **Sentry** | `agents.sentry.v1` | EWMA-based anomaly detection on error rate, latency, policy denials, quota exceeded. Creates incidents and emits `anomaly.detected` / `incident.opened`. |
| **Coordinator** | `agents.coordinator.v1` | State machine: OPEN → DIAGNOSING → MITIGATING → MONITORING → RESOLVED/ESCALATED. Cleans up on external RESOLVED (e.g. from recovery). |
| **Mitigation** | `agents.mitigation.v1` | Receives `recommended_actions.created`, executes DryRun + Apply per action. Respects `ApprovalRequired`. |
| **Recovery** | `agents.recovery.v1` | Monitors post-mitigation. Counts successes within a monitoring window. Emits RESOLVED if stable, rollback + OPEN if regression or TTL expiry. |

## Endpoints

The health server runs on `OPERATOR_HEALTH_PORT` (default `8090`):

| Path | Purpose |
|------|---------|
| `GET /healthz` | Liveness — always 200 if process is alive |
| `GET /readyz` | Readiness — pings `nexus-core /readyz`, returns 503 if unreachable |
| `GET /metrics` | Prometheus metrics (`events_processed_total`, `event_processing_duration_seconds`, `consumer_offset`, `core_requests_total`) |

## Configuration

All via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `NEXUS_CORE_URL` | `http://nexus-core:8080` | nexus-core base URL |
| `OPERATOR_INTERNAL_KEY` | *(required)* | Auth key for nexus-core internal API |
| `NEXUS_DEFAULT_ORG_ID` | *(empty)* | Default org UUID for events without explicit org |
| `OPERATOR_BATCH_SIZE` | `100` | Events per poll batch |
| `OPERATOR_POLL_INTERVAL_MS` | `700` | Poll interval in ms |
| `OPERATOR_IDLE_INTERVAL_MS` | `15000` | Recovery idle check interval in ms |
| `OPERATOR_HEALTH_PORT` | `8090` | Health/metrics server port |
| `OPERATOR_DATA_DIR` | `/app/data` | Persistence directory for offsets and state |
| `NEXUS_LOG_LEVEL` | `info` | Log level (trace/debug/info/warn/error) |

## Persistence

State is persisted to `OPERATOR_DATA_DIR` as JSON files with atomic writes (tmp + rename):

| File | Content |
|------|---------|
| `offsets.json` | Consumer group offsets — survives restarts |
| `sentry_state.json` | EWMA baselines and anomaly fingerprints |
| `recovery_tracks.json` | Active post-mitigation monitoring tracks |
| `proposals.json` | Pending action engine proposals |

In Docker, this directory is backed by a named volume (`control_operators_data`).

## Build & Run

```bash
# Build container (from repo root)
docker compose build nexus-control-operators

# Run with the full stack
docker compose up -d

# Run tests
cd nexus-control-operators && go test ./...

# Build binary locally
cd nexus-control-operators && go build -o bin/nexus-control-operators ./cmd/ops-workers
```

## Development

```bash
# With hot-reload (requires air)
docker compose -f docker-compose.dev.yml up nexus-control-operators

# Run vet + tests
cd nexus-control-operators && go vet ./... && go test ./...
```

## Shutdown

On `SIGINT`/`SIGTERM`:
1. Context cancelled → all worker consumers drain gracefully
2. `sync.WaitGroup` waits for all workers to finish
3. Health server shuts down with 5s grace period
4. Process exits
