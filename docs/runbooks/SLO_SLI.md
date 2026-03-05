# SLO/SLI Definitions — Nexus

## Service Level Indicators (SLIs)

| SLI | Definition | Measurement |
| --- | --- | --- |
| Availability | Percentage of HTTP requests that return non-5xx | `1 - (rate(5xx) / rate(total))` |
| Latency (p95) | 95th percentile latency for `/v1/run` | `histogram_quantile(0.95, sum(rate(nexus_run_latency_ms_prom_bucket[5m])) by (le))` |
| Latency (p99) | 99th percentile latency for `/v1/run` | `histogram_quantile(0.99, sum(rate(nexus_run_latency_ms_prom_bucket[5m])) by (le))` |
| Error Rate | Percentage of runs with `status=error` | `sum(rate(nexus_run_total_prom{status="error"}[5m])) / sum(rate(nexus_run_total_prom[5m]))` |
| Webhook Processing Latency | 99th percentile processing time for inbound webhooks | `histogram_quantile(0.99, sum(rate(nexus_saas_request_duration_seconds_bucket{handler=~".*webhooks.*"}[5m])) by (le))` |

## Service Level Objectives (SLOs)

| Service | SLO | Target | Error Budget (30d) |
| --- | --- | --- | --- |
| nexus-core | Availability | 99.9% | 43.2 minutes downtime |
| nexus-core | Latency p95 (`/v1/run`) | < 500ms | n/a |
| nexus-core | Error Rate (`/v1/run`) | < 1% | n/a |
| nexus-saas | Availability | 99.9% | 43.2 minutes downtime |
| nexus-saas | Webhook Processing Latency p99 | < 5s | n/a |

## Error Budget Policy

- More than 50% of budget consumed: reliability review and concrete action plan.
- More than 80% of budget consumed: feature freeze for affected service, reliability work only.
- 100% budget consumed: all engineering focus shifts to reliability until service is back within policy.

## Prometheus Recording Rules (Optional)

For automated SLO tracking, add recording rules such as:

- `nexus_core:availability:ratio_5m`
- `nexus_core:error_rate:ratio_5m`
- `nexus_core:run_latency_p95_ms:5m`
- `nexus_saas:webhook_latency_p99_seconds:5m`

These rules can be added in a dedicated file (for example `monitoring/prometheus/rules/slo.yml`) and referenced from Prometheus `rule_files`.
