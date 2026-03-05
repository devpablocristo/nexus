# Gateway Load Test (k6)

Install:

```bash
brew install k6
```

Run:

```bash
k6 run \
  --env API_URL=http://localhost:8080 \
  --env API_KEY=dev-api-key \
  scripts/loadtest/k6_gateway.js
```

The script targets `POST /v1/run` and enforces these thresholds:

- p95 latency < 500ms
- error rate < 1%
