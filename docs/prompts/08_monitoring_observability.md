# Prompt 08 — Monitoring, Observability & Final Polish

## Contexto del proyecto

Nexus es una plataforma SaaS multi-tenant con 5 servicios:

| Servicio | Stack | Puerto | Métricas | Health |
|----------|-------|--------|----------|--------|
| nexus-core | Go/Gin | 8080 | `/metrics` (ginprometheus + custom) | `/readyz` |
| nexus-saas | Go/Gin | 8082 | `/metrics` (ginprometheus, solo HTTP) | `/health` |
| nexus-control-operators | Go | 8090 | `/metrics` (custom) | `/healthz` |
| nexus-ai-operators | Python/FastAPI | 8000 | `/metrics` (custom, requiere X-Operator-Key) | `/readyz` |
| nexus-tower | Nginx/React | 4173 | — | `/` |

---

## Lo que YA existe (NO duplicar)

### Métricas actuales

**nexus-core:**
- `nexus_gateway_requests_total` (Counter) — HTTP por method/handler/status
- `nexus_run_total_prom` (Counter) — runs por tool_name/decision/status
- `nexus_run_latency_ms_prom` (Histogram) — latencia por tool_name/decision/status

**nexus-saas:**
- `nexus_saas_requests_total` (Counter) — HTTP por method/handler/status (go-gin-prometheus)
- NO tiene métricas custom de negocio

**nexus-control-operators:**
- `nexus_operators_events_processed_total` (Counter) — por worker/status
- `nexus_operators_event_processing_duration_seconds` (Histogram)
- `nexus_operators_consumer_offset` (Gauge)
- `nexus_operators_core_requests_total` (Counter) — llamadas al core

**nexus-ai-operators:**
- `nexus_operator_events_consumed_total` (Counter)
- `nexus_operator_actions_applied_total` (Counter)
- `nexus_operator_incidents_opened_total` (Counter)
- `nexus_operator_proposals_created_total` (Counter)
- `nexus_operator_last_cursor` (Gauge)

### Prometheus config actual

Archivo: `nexus-core/monitoring/prometheus/prometheus.yml`

```yaml
scrape_configs:
  - job_name: nexus-core
    static_configs:
      - targets: ["nexus-core:8080"]
  - job_name: nexus-control-operators
    static_configs:
      - targets: ["nexus-control-operators:8090"]
```

**FALTA:** nexus-saas y nexus-ai-operators no están siendo scrapeados.

### Dashboard Grafana actual

Un solo dashboard `nexus-gateway-overview` con 14 paneles, todos basados en métricas de nexus-core (runs, latencia, decisiones, throughput HTTP).

### Sistema de alertas

`nexus-saas/internal/alerts/` tiene:
- `EvaluateAll()` — evalúa reglas vs métricas (deny_rate, error_rate, rate_limited_count)
- `MetricsSource` basado en audit_events
- Modelo de reglas con cooldown, umbrales, webhooks
- Endpoints CRUD para reglas (`/v1/alert-rules`)

**PROBLEMA CRÍTICO:** `EvaluateAll()` existe pero **ningún cron/worker lo invoca**. Las alertas nunca se disparan.

### Logging

Todos los servicios usan JSON estructurado:
- Go: `zerolog` → stdout (JSON)
- Python: `JsonFormatter` → stdout (JSON)

### CloudWatch (producción)

Alarms: ECS CPU/Memory, RDS CPU/Storage, ALB 5xx/unhealthy hosts.
Dashboard: `nexus-overview` con ALB 5xx y ECS CPU.

### Monitoring UI (Tower)

Página `/monitoring` embebe paneles de Grafana con KPIs y gráficos, filtro por tool, rango de tiempo.

---

## Lo que FALTA (implementar)

### 1. Prometheus — Agregar scrape targets faltantes

**Archivo:** `nexus-core/monitoring/prometheus/prometheus.yml`

Agregar nexus-saas y nexus-ai-operators:

```yaml
  - job_name: nexus-saas
    metrics_path: /metrics
    static_configs:
      - targets: ["nexus-saas:8082"]

  - job_name: nexus-ai-operators
    metrics_path: /metrics
    static_configs:
      - targets: ["nexus-ai-operators:8000"]
    # nexus-ai-operators requiere X-Operator-Key para /metrics
    # Opciones: (a) quitar auth de /metrics, (b) usar bearer_token en Prometheus
    # Recomendación: quitar auth de /metrics (endpoint de diagnóstico, no sensible)
```

**Importante sobre nexus-ai-operators:** El endpoint `/metrics` actualmente requiere `X-Operator-Key`. Para que Prometheus pueda scrapearlo, hay que:
- Opción A (recomendada): Excluir `/metrics` y `/healthz` del auth check en `routes.py`
- Opción B: Configurar `authorization` en prometheus.yml

### 2. Métricas custom de negocio en nexus-saas

Agregar métricas Prometheus custom en nexus-saas para visibilidad de negocio:

```go
// nexus-saas/internal/shared/metrics/metrics.go (crear)
var (
    WebhooksReceived = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "nexus_saas_webhooks_received_total", Help: "Webhooks received"},
        []string{"source", "event_type"},
    )
    BillingCheckouts = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "nexus_saas_billing_checkouts_total", Help: "Checkout sessions created"},
        []string{"plan_code"},
    )
    NotificationsSent = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "nexus_saas_notifications_sent_total", Help: "Notifications sent"},
        []string{"notification_type", "channel"},
    )
    AlertsEvaluated = promauto.NewCounter(
        prometheus.CounterOpts{Name: "nexus_saas_alerts_evaluated_total", Help: "Alert evaluation cycles"},
    )
    AlertsFired = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "nexus_saas_alerts_fired_total", Help: "Alerts that fired"},
        []string{"rule_name"},
    )
    UsageMeteringEvents = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "nexus_saas_usage_events_total", Help: "Usage metering events"},
        []string{"org_id", "counter"},
    )
)
```

Instrumentar en:
- `internal/clerkwebhook/handler.go` → `WebhooksReceived.WithLabelValues("clerk", eventType).Inc()`
- `internal/billing/webhook_handler.go` → `WebhooksReceived.WithLabelValues("stripe", eventType).Inc()`
- `internal/billing/usecases.go` (CreateCheckout) → `BillingCheckouts`
- `internal/notifications/usecases.go` (SendNotification) → `NotificationsSent`
- `internal/alerts/usecases.go` (EvaluateAll) → `AlertsEvaluated`, `AlertsFired`

### 3. Alert evaluation worker (CRÍTICO)

Crear un goroutine/ticker en nexus-saas que ejecute `EvaluateAll()` periódicamente.

**Opción A — Ticker en el servicio (recomendada para simplicidad):**

Agregar en `nexus-saas/cmd/api/main.go` o en wire:

```go
// Después de iniciar el HTTP server, lanzar el alert evaluator
go func() {
    ticker := time.NewTicker(alertEvalInterval) // e.g. 1 minute
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            if err := alertUsecases.EvaluateAll(ctx); err != nil {
                logger.Error().Err(err).Msg("alert evaluation failed")
            }
        case <-ctx.Done():
            return
        }
    }
}()
```

**Configuración:**
- Variable: `NEXUS_ALERT_EVAL_INTERVAL` (default: `60s`)
- Agregar en `cmd/config/` como campo del service config

**Opción B — Cron endpoint interno:**

Exponer `POST /internal/alerts/evaluate` y usar un cron externo (ECS scheduled task, CloudWatch Events rule). Más complejo pero más controlable.

### 4. Dashboards Grafana adicionales

Crear dashboards para los servicios que no tienen:

#### Dashboard 2: `nexus-saas-overview`

**Archivo:** `nexus-core/monitoring/grafana/dashboards/nexus-saas-overview.json`

Paneles:
| # | Título | Tipo | Query |
|---|--------|------|-------|
| 1 | HTTP Requests/s | timeseries | `sum(rate(nexus_saas_requests_total[5m])) by (code)` |
| 2 | Webhooks Received | timeseries | `sum(rate(nexus_saas_webhooks_received_total[5m])) by (source)` |
| 3 | Billing Checkouts | stat | `sum(increase(nexus_saas_billing_checkouts_total[24h]))` |
| 4 | Notifications Sent | timeseries | `sum(rate(nexus_saas_notifications_sent_total[5m])) by (notification_type)` |
| 5 | Alert Evaluations | timeseries | `rate(nexus_saas_alerts_evaluated_total[5m])` |
| 6 | Alerts Fired | timeseries | `sum(rate(nexus_saas_alerts_fired_total[5m])) by (rule_name)` |
| 7 | HTTP Latency p95 | stat | `histogram_quantile(0.95, ...)` si se agrega histograma |
| 8 | Error Rate (5xx) | stat | `sum(rate(nexus_saas_requests_total{code=~"5.."}[5m]))` |

#### Dashboard 3: `nexus-operators-overview`

**Archivo:** `nexus-core/monitoring/grafana/dashboards/nexus-operators-overview.json`

Paneles:
| # | Título | Tipo | Query |
|---|--------|------|-------|
| 1 | Events Processed (control) | timeseries | `sum(rate(nexus_operators_events_processed_total[5m])) by (worker, status)` |
| 2 | Processing Duration (control) | timeseries | `histogram_quantile(0.95, ...)` |
| 3 | Consumer Offset | gauge | `nexus_operators_consumer_offset` |
| 4 | Core API Calls (control) | timeseries | `sum(rate(nexus_operators_core_requests_total[5m])) by (status)` |
| 5 | Events Consumed (AI) | timeseries | `rate(nexus_operator_events_consumed_total[5m])` |
| 6 | Actions Applied (AI) | stat | `sum(increase(nexus_operator_actions_applied_total[1h]))` |
| 7 | Incidents Opened (AI) | stat | `sum(increase(nexus_operator_incidents_opened_total[1h]))` |
| 8 | Proposals Created (AI) | stat | `sum(increase(nexus_operator_proposals_created_total[1h]))` |
| 9 | AI Last Cursor | gauge | `nexus_operator_last_cursor` |

Registrar los nuevos dashboards en el provisioning de Grafana. El archivo `nexus-core/monitoring/grafana/provisioning/dashboards/dashboard.yml` ya apunta al directorio de dashboards, así que basta con poner los JSON ahí.

### 5. Monitoring UI — Selector de dashboard en Tower

Actualizar `nexus-tower/src/features/monitoring/MonitoringPage.tsx` para permitir cambiar entre los 3 dashboards:

- Agregar un selector/tabs: "Gateway" | "SaaS" | "Operators"
- Cambiar el `DASHBOARD_UID` según la selección
- Cada dashboard muestra sus propios paneles embebidos
- Mantener el filtro de tool_name solo para el dashboard Gateway

### 6. SLO/SLI definitions

Crear `docs/runbooks/SLO_SLI.md`:

```markdown
# SLO/SLI Definitions — Nexus

## Service Level Indicators (SLIs)

| SLI | Definición | Medición |
|-----|-----------|----------|
| Availability | % de requests HTTP que retornan non-5xx | `1 - (rate(5xx) / rate(total))` |
| Latency (p95) | Percentil 95 de latencia de /v1/run | `histogram_quantile(0.95, nexus_run_latency_ms_prom)` |
| Latency (p99) | Percentil 99 de latencia de /v1/run | `histogram_quantile(0.99, ...)` |
| Error Rate | % de runs con status=error | `rate(error) / rate(total)` |

## Service Level Objectives (SLOs)

| Servicio | SLO | Target | Error Budget (30d) |
|----------|-----|--------|-------------------|
| nexus-core | Availability | 99.9% | 43.2 min downtime |
| nexus-core | Latency p95 | < 500ms | — |
| nexus-core | Error rate | < 1% | — |
| nexus-saas | Availability | 99.9% | 43.2 min downtime |
| nexus-saas | Webhook processing | < 5s p99 | — |

## Error Budget Policy

- > 50% budget consumed → review & action plan
- > 80% budget consumed → feature freeze, focus on reliability
- 100% budget consumed → all hands on reliability until budget recovers

## Prometheus Recording Rules (for SLO tracking)

(Opcional: agregar archivo prometheus/rules/slo.yml si se quiere tracking automático)
```

### 7. Prometheus alerting rules

Crear `nexus-core/monitoring/prometheus/alerts.yml`:

```yaml
groups:
  - name: nexus-core
    rules:
      - alert: HighErrorRate
        expr: sum(rate(nexus_run_total_prom{status="error"}[5m])) / sum(rate(nexus_run_total_prom[5m])) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate on nexus-core (> 5%)"

      - alert: HighLatency
        expr: histogram_quantile(0.95, sum(rate(nexus_run_latency_ms_prom_bucket[5m])) by (le)) > 1000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "p95 latency > 1s on nexus-core"

      - alert: ServiceDown
        expr: up{job=~"nexus-.*"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "{{ $labels.job }} is down"

  - name: nexus-saas
    rules:
      - alert: SaaSHighErrorRate
        expr: sum(rate(nexus_saas_requests_total{code=~"5.."}[5m])) / sum(rate(nexus_saas_requests_total[5m])) > 0.05
        for: 5m
        labels:
          severity: warning

  - name: nexus-operators
    rules:
      - alert: OperatorStalled
        expr: changes(nexus_operator_last_cursor[10m]) == 0 and nexus_operator_last_cursor > 0
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "AI operator cursor hasn't advanced in 10m"

      - alert: ControlOperatorStalled
        expr: changes(nexus_operators_consumer_offset[10m]) == 0 and nexus_operators_consumer_offset > 0
        for: 10m
        labels:
          severity: warning
```

Actualizar `prometheus.yml` para cargar las reglas:

```yaml
rule_files:
  - "/etc/prometheus/alerts.yml"
```

Y montar el archivo en docker-compose.yml (volumen al contenedor de Prometheus).

### 8. Operators polish

#### 8a. Retry con backoff en nexus-control-operators

Si las llamadas al core fallan, implementar retry con exponential backoff:

```go
// En el client HTTP del control-operators
// Retry hasta 3 veces con backoff: 1s, 2s, 4s
```

Verificar en `nexus-control-operators/internal/shared/coreclient/` o similar.

#### 8b. Circuit breaker (opcional, nice-to-have)

Si el core está caído, el operator no debería seguir intentando. Un circuit breaker simple:
- Después de N fallos consecutivos, abrir el circuito
- Reintentar cada M segundos
- Si vuelve a funcionar, cerrar

### 9. nexus-ai-operators — Quitar auth de /metrics

Para que Prometheus pueda scrapearlo sin auth:

**Archivo:** `nexus-ai-operators/app/api/routes.py`

El endpoint `/metrics` y `/healthz` deben ser accesibles sin `X-Operator-Key`. Verificar que no pasan por `verify_operator_key`. Actualmente `/healthz` y `/readyz` no tienen `Depends(verify_operator_key)`, pero `/metrics` puede tenerlo — verificar y corregir si es necesario.

---

## Archivos a crear

| Archivo | Descripción |
|---------|-------------|
| `nexus-saas/internal/shared/metrics/metrics.go` | Métricas Prometheus custom de nexus-saas |
| `nexus-core/monitoring/grafana/dashboards/nexus-saas-overview.json` | Dashboard Grafana para nexus-saas |
| `nexus-core/monitoring/grafana/dashboards/nexus-operators-overview.json` | Dashboard Grafana para operators |
| `nexus-core/monitoring/prometheus/alerts.yml` | Reglas de alerting Prometheus |
| `docs/runbooks/SLO_SLI.md` | Definiciones de SLO/SLI |

## Archivos a modificar

| Archivo | Cambio |
|---------|--------|
| `nexus-core/monitoring/prometheus/prometheus.yml` | Agregar scrape targets para saas y ai-operators + rule_files |
| `nexus-saas/cmd/api/main.go` o wire | Agregar alert evaluation ticker goroutine |
| `nexus-saas/cmd/config/` | Agregar `NEXUS_ALERT_EVAL_INTERVAL` |
| `nexus-saas/internal/clerkwebhook/handler.go` | Instrumentar WebhooksReceived |
| `nexus-saas/internal/billing/webhook_handler.go` | Instrumentar WebhooksReceived |
| `nexus-saas/internal/billing/usecases.go` | Instrumentar BillingCheckouts |
| `nexus-saas/internal/notifications/usecases.go` | Instrumentar NotificationsSent |
| `nexus-saas/internal/alerts/usecases.go` | Instrumentar AlertsEvaluated, AlertsFired |
| `nexus-ai-operators/app/api/routes.py` | Quitar auth de /metrics si la tiene |
| `nexus-tower/src/features/monitoring/MonitoringPage.tsx` | Selector de dashboard (Gateway/SaaS/Operators) |
| `docker-compose.yml` | Montar alerts.yml en Prometheus |

---

## Criterios de aceptación

### Prometheus & scraping
1. [ ] `curl http://localhost:9090/api/v1/targets` muestra 4 targets UP: nexus-core, nexus-saas, nexus-control-operators, nexus-ai-operators
2. [ ] `curl http://localhost:9090/api/v1/query?query=nexus_saas_requests_total` retorna datos
3. [ ] `curl http://localhost:9090/api/v1/query?query=nexus_operator_events_consumed_total` retorna datos

### Métricas custom saas
4. [ ] `curl http://localhost:8082/metrics | grep nexus_saas_webhooks` muestra la métrica (aunque esté en 0)
5. [ ] `curl http://localhost:8082/metrics | grep nexus_saas_alerts_evaluated` muestra la métrica

### Alert evaluation worker
6. [ ] Logs de nexus-saas muestran evaluación periódica de alertas (cada ~60s)
7. [ ] Crear una alert rule via API → esperar 2 minutos → verificar que se evaluó (logs o métricas)

### Dashboards Grafana
8. [ ] `http://localhost:3001/d/nexus-gateway-overview` existe y muestra datos
9. [ ] `http://localhost:3001/d/nexus-saas-overview` existe y muestra datos
10. [ ] `http://localhost:3001/d/nexus-operators-overview` existe y muestra datos

### Prometheus alerts
11. [ ] `curl http://localhost:9090/api/v1/rules` muestra las reglas de alerting
12. [ ] Regla `ServiceDown` aparece como inactive (todos los servicios están up)

### Tower UI
13. [ ] Página `/monitoring` tiene selector para cambiar entre dashboards Gateway/SaaS/Operators
14. [ ] Cada dashboard embebe los paneles correctos

### Documentación
15. [ ] `docs/runbooks/SLO_SLI.md` existe con SLIs, SLOs y error budget policy

### Operators polish
16. [ ] nexus-control-operators tiene retry con backoff en llamadas al core

### Build & tests
17. [ ] `cd nexus-core && go build ./...` ✓
18. [ ] `cd nexus-saas && go build ./...` ✓
19. [ ] `cd nexus-control-operators && go build ./...` ✓
20. [ ] `cd nexus-tower && npm run build` ✓
21. [ ] `make e2e` pasa sin regresiones

---

## Orden sugerido de implementación

1. Prometheus config (agregar scrape targets) — efecto inmediato
2. Quitar auth de /metrics en ai-operators
3. Métricas custom en nexus-saas
4. Alert evaluation worker
5. Prometheus alerting rules + montar en docker-compose
6. Dashboard nexus-saas-overview
7. Dashboard nexus-operators-overview
8. Tower UI — selector de dashboard
9. Operators retry con backoff
10. SLO/SLI doc
11. Verificar todo compila y e2e pasan
