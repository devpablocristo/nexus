# Prompt 10 — Hardening final de producción (97% → 100%)

## Contexto del proyecto

Nexus es una plataforma SaaS multi-tenant con 9 prompts ya implementados:
- 01: User Identity (Clerk + AWS)
- 02: Billing (Stripe)
- 03: Admin Console UI
- 04: Email & Notifications (SES)
- 05: Developer Experience & CI/CD
- 06: Production Infrastructure (Terraform)
- 07: Security Hardening
- 08: Monitoring & Observability
- 09: Final Polish & Launch Readiness

**Score actual: 97% (166/170)**. Este prompt cierra los últimos gaps para producción real.

| Servicio | Stack | Puerto |
|----------|-------|--------|
| nexus-core | Go/Gin | 8080 |
| nexus-saas | Go/Gin | 8082 |
| nexus-control-operators | Go | 8090 |
| nexus-ai-operators | Python/FastAPI | 8000 |
| nexus-tower | React/Vite/Nginx | 5173 |

Monorepo root: `/home/pablo/Projects/Pablo/nexus`

Shared Go packages: `pkgs/go-pkg/`

## Alcance obligatorio

Este prompt hereda los estándares de `docs/prompts/00_base_transversal.md`.

Todo lo definido acá es obligatorio para el cierre productivo final:
- security policy y scanning
- hardening de auth
- DLQ y resiliencia de operators
- documentación operativa adicional
- traceability y polish final de servicios/frontend

La secuencia propuesta es técnica. No convierte ninguna parte en opcional.

## Prerequisito

Leer y respetar `docs/prompts/00_base_transversal.md` antes de ejecutar este prompt.

---

## Objetivo

Cerrar los 10 gaps restantes para pasar de 97% a 100%. Cada área está en 9/10 y necesita un último item concreto.

---

## 1. Security: SECURITY.md + CI fail-on-critical + rate limit SaaS

### 1.1 Crear `SECURITY.md` en la raíz del monorepo

```markdown
# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.9.x   | ✅        |

## Reporting a Vulnerability

**DO NOT** open a public issue.

Email **security@nexus.io** with:
- Description of the vulnerability
- Steps to reproduce
- Impact assessment

We will acknowledge within 48 hours and provide a fix timeline within 5 business days.

## Bug Bounty

We do not currently run a bug bounty program.
```

### 1.2 CI: quitar `continue-on-error` en vulns críticas

Archivo: `.github/workflows/ci.yml`, job `security-scan`.

Cambiar los 3 steps de scanning:
- `govulncheck ./...` → **quitar** `continue-on-error: true`
- `pip-audit` → mantener `continue-on-error: true` (advisories frecuentes en transitive deps)
- `npm audit --audit-level=critical` → **quitar** `continue-on-error: true`

Así el build falla si Go o npm tienen vulnerabilidades críticas.

### 1.3 Rate limit por tenant en nexus-saas

Archivo: `nexus-saas/wire/bootstrap_routes.go`

Agregar un middleware de rate limiting por `org_id` a los endpoints de la API SaaS (`/v1/...`).

Usar el mismo patrón que `nexus-core/internal/gateway/executor/ratelimit/`:
- Implementar en `nexus-saas/internal/shared/ratelimit/middleware.go`
- Leer `org_id` del context (ya lo pone el auth middleware)
- Límite configurable via env: `NEXUS_SAAS_RATE_LIMIT_RPS` (default: 100)
- En-memory con `sync.Map` + token bucket o sliding window
- Retornar `429 Too Many Requests` con header `Retry-After`

```go
package ratelimit

import (
    "net/http"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
    "golang.org/x/time/rate"
)

type TenantLimiter struct {
    mu       sync.RWMutex
    limiters map[string]*rate.Limiter
    rps      float64
    burst    int
}

func NewTenantLimiter(rps float64, burst int) *TenantLimiter {
    return &TenantLimiter{
        limiters: make(map[string]*rate.Limiter),
        rps:      rps,
        burst:    burst,
    }
}

func (tl *TenantLimiter) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        orgID, _ := c.Get("org_id")
        key, _ := orgID.(string)
        if key == "" {
            c.Next()
            return
        }

        tl.mu.RLock()
        lim, ok := tl.limiters[key]
        tl.mu.RUnlock()

        if !ok {
            tl.mu.Lock()
            lim, ok = tl.limiters[key]
            if !ok {
                lim = rate.NewLimiter(rate.Limit(tl.rps), tl.burst)
                tl.limiters[key] = lim
            }
            tl.mu.Unlock()
        }

        if !lim.Allow() {
            c.Header("Retry-After", "1")
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error": "rate limit exceeded",
                "code":  "RATE_LIMIT_EXCEEDED",
            })
            return
        }
        c.Next()
    }
}
```

Registrar en `bootstrap_routes.go` después del auth middleware:

```go
rl := ratelimit.NewTenantLimiter(cfg.SaaSRateLimitRPS, cfg.SaaSRateLimitBurst)
v1.Use(rl.Middleware())
```

Agregar a config: `NEXUS_SAAS_RATE_LIMIT_RPS` (default 100), `NEXUS_SAAS_RATE_LIMIT_BURST` (default 200).

---

## 2. Auth: protección brute-force en login attempts

Archivo: `nexus-saas/internal/shared/handlers/auth_middleware.go`

Agregar un contador de intentos fallidos por IP:
- Implementar en `nexus-saas/internal/shared/ratelimit/auth_limiter.go`
- Tras 10 auth failures en 5 minutos desde la misma IP → bloquear 15 minutos
- Retornar `429 Too Many Requests` con mensaje claro
- In-memory con TTL (no necesita Redis en esta fase)

```go
package ratelimit

import (
    "sync"
    "time"
)

type AuthLimiter struct {
    mu       sync.Mutex
    attempts map[string]*failRecord
    maxFails int
    window   time.Duration
    lockout  time.Duration
}

type failRecord struct {
    count   int
    firstAt time.Time
    lockUntil time.Time
}

func NewAuthLimiter(maxFails int, window, lockout time.Duration) *AuthLimiter {
    al := &AuthLimiter{
        attempts: make(map[string]*failRecord),
        maxFails: maxFails,
        window:   window,
        lockout:  lockout,
    }
    go al.cleanup()
    return al
}

func (al *AuthLimiter) IsBlocked(ip string) bool {
    al.mu.Lock()
    defer al.mu.Unlock()
    rec, ok := al.attempts[ip]
    if !ok {
        return false
    }
    if time.Now().Before(rec.lockUntil) {
        return true
    }
    if time.Since(rec.firstAt) > al.window {
        delete(al.attempts, ip)
        return false
    }
    return false
}

func (al *AuthLimiter) RecordFailure(ip string) {
    al.mu.Lock()
    defer al.mu.Unlock()
    rec, ok := al.attempts[ip]
    if !ok || time.Since(rec.firstAt) > al.window {
        al.attempts[ip] = &failRecord{count: 1, firstAt: time.Now()}
        return
    }
    rec.count++
    if rec.count >= al.maxFails {
        rec.lockUntil = time.Now().Add(al.lockout)
    }
}

func (al *AuthLimiter) RecordSuccess(ip string) {
    al.mu.Lock()
    defer al.mu.Unlock()
    delete(al.attempts, ip)
}

func (al *AuthLimiter) cleanup() {
    for range time.Tick(5 * time.Minute) {
        al.mu.Lock()
        now := time.Now()
        for k, v := range al.attempts {
            if now.After(v.lockUntil) && now.Sub(v.firstAt) > al.window {
                delete(al.attempts, k)
            }
        }
        al.mu.Unlock()
    }
}
```

Integrar en el auth middleware: al inicio, check `IsBlocked(c.ClientIP())`. Si auth falla, `RecordFailure`. Si OK, `RecordSuccess`.

Test unitario en `nexus-saas/internal/shared/ratelimit/auth_limiter_test.go`.

---

## 3. Operators: dead-letter log para eventos fallidos

### 3.1 Control operators: DLQ file

Archivo: `nexus-control-operators/internal/ops/eventstore/consumer.go`

Actualmente, cuando un evento falla 3 veces se loguea y se salta. Agregar persistencia de eventos fallidos:

Crear `nexus-control-operators/internal/ops/eventstore/deadletter.go`:

```go
package eventstore

import (
    "encoding/json"
    "os"
    "sync"
    "time"
)

type DeadLetterEntry struct {
    EventID   string    `json:"event_id"`
    Payload   any       `json:"payload"`
    Error     string    `json:"error"`
    Attempts  int       `json:"attempts"`
    FailedAt  time.Time `json:"failed_at"`
}

type DeadLetterLog struct {
    mu   sync.Mutex
    path string
}

func NewDeadLetterLog(path string) *DeadLetterLog {
    return &DeadLetterLog{path: path}
}

func (d *DeadLetterLog) Append(entry DeadLetterEntry) error {
    d.mu.Lock()
    defer d.mu.Unlock()

    f, err := os.OpenFile(d.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    line, _ := json.Marshal(entry)
    line = append(line, '\n')
    _, err = f.Write(line)
    return err
}
```

En `consumer.go`, cuando el evento falla permanentemente (tras max retries), en vez de solo loguearlo:

```go
dlEntry := eventstore.DeadLetterEntry{
    EventID:  event.ID,
    Payload:  event,
    Error:    lastErr.Error(),
    Attempts: maxRetries,
    FailedAt: time.Now(),
}
dlLog.Append(dlEntry)
```

Ruta del archivo DLQ configurable: `NEXUS_DLQ_PATH` (default: `data/dead_letters.jsonl`).

### 3.2 AI operators: DLQ equivalente

Archivo: `nexus-ai-operators/app/engine/`

Crear `nexus-ai-operators/app/engine/dead_letter.py`:

```python
import json
from datetime import datetime, timezone
from pathlib import Path
from threading import Lock

class DeadLetterLog:
    def __init__(self, path: str = "data/dead_letters.jsonl"):
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._lock = Lock()

    def append(self, event_id: str, payload: dict, error: str, attempts: int) -> None:
        entry = {
            "event_id": event_id,
            "payload": payload,
            "error": error,
            "attempts": attempts,
            "failed_at": datetime.now(timezone.utc).isoformat(),
        }
        with self._lock:
            with open(self.path, "a") as f:
                f.write(json.dumps(entry) + "\n")
```

Integrar en el engine cycle donde se manejan errores de procesamiento.

### 3.3 Métricas para DLQ

Agregar counter Prometheus `nexus_dead_letter_events_total` en ambos operators para alertar sobre eventos muertos.

---

## 4. Billing: documentar grace period + auto-suspend

### 4.1 Crear `docs/billing/GRACE_PERIOD_POLICY.md`

```markdown
# Grace Period & Dunning Policy

## Payment failure flow

1. Stripe fires `invoice.payment_failed` webhook
2. Nexus marks tenant as `past_due` immediately
3. Email `payment_failed` sent to org admins

## Grace period

- **Duration**: 14 days from first failed payment
- **During grace period**: Full API access maintained, banner shown in Tower
- **After grace period**: Tenant auto-suspended via scheduled job

## Auto-suspension

A worker in nexus-saas checks daily for tenants in `past_due` state
longer than 14 days and calls `SuspendTenant` automatically.

## Reactivation

- Update payment method in Stripe Customer Portal
- Stripe retries payment → `invoice.paid` webhook
- Nexus auto-reactivates tenant
- OR: Admin manually reactivates via Admin Console

## Stripe retry schedule

Stripe Smart Retries handles payment retries (up to 4 attempts over ~3 weeks).
No custom retry logic needed on Nexus side.
```

### 4.2 Implementar dunning worker

Archivo: `nexus-saas/internal/billing/dunning_worker.go`

```go
package billing

import (
    "context"
    "time"

    "github.com/rs/zerolog/log"
)

const defaultGracePeriod = 14 * 24 * time.Hour

type DunningWorker struct {
    repo        Repository
    adminUC     AdminUseCase
    gracePeriod time.Duration
}

func NewDunningWorker(repo Repository, adminUC AdminUseCase) *DunningWorker {
    return &DunningWorker{
        repo:        repo,
        adminUC:     adminUC,
        gracePeriod: defaultGracePeriod,
    }
}

func (w *DunningWorker) RunOnce(ctx context.Context) {
    cutoff := time.Now().Add(-w.gracePeriod)
    tenants, err := w.repo.FindPastDueBefore(ctx, cutoff)
    if err != nil {
        log.Error().Err(err).Msg("dunning: failed to query past_due tenants")
        return
    }
    for _, t := range tenants {
        if err := w.adminUC.AutoSuspend(ctx, t.OrgID); err != nil {
            log.Error().Err(err).Str("org_id", t.OrgID.String()).Msg("dunning: auto-suspend failed")
            continue
        }
        log.Info().Str("org_id", t.OrgID.String()).Msg("dunning: tenant auto-suspended after grace period")
    }
}
```

Registrar en el bootstrap con `time.NewTicker(24 * time.Hour)`.

Agregar al repo: `FindPastDueBefore(ctx, cutoff time.Time) ([]TenantBilling, error)`.

---

## 5. Usage metering: notificación approaching-limit

Archivo: `nexus-saas/internal/billing/usage_check.go`

Cuando se ingiere un evento de uso (`POST /internal/usage/events`), verificar:
- Si el uso alcanza **80%** del hard limit → enviar notificación `usage_warning_80`
- Si alcanza **95%** → enviar notificación `usage_warning_95`
- Si alcanza **100%** → notificación `usage_limit_reached` + posible reject

Implementar con un check simple en el handler de ingesta:

```go
func (h *Handler) checkUsageThresholds(ctx context.Context, orgID uuid.UUID, metricName string, current, limit int64) {
    if limit <= 0 {
        return
    }
    pct := float64(current) / float64(limit) * 100
    switch {
    case pct >= 100:
        h.notifySvc.SendAsync(ctx, orgID, "usage_limit_reached", map[string]string{
            "metric": metricName, "current": fmt.Sprintf("%d", current), "limit": fmt.Sprintf("%d", limit),
        })
    case pct >= 95:
        h.notifySvc.SendAsync(ctx, orgID, "usage_warning_95", map[string]string{
            "metric": metricName, "pct": "95",
        })
    case pct >= 80:
        h.notifySvc.SendAsync(ctx, orgID, "usage_warning_80", map[string]string{
            "metric": metricName, "pct": "80",
        })
    }
}
```

Deduplicar: no enviar la misma alerta más de una vez por período de facturación. Usar el mecanismo existente de `reference_id` en notifications.

Agregar templates de email para `usage_warning_80`, `usage_warning_95`, `usage_limit_reached` en `nexus-saas/internal/notifications/templates.go`.

---

## 6. CI/CD: runbook de rollback

Crear `docs/runbooks/DEPLOY_ROLLBACK.md`:

```markdown
# Deploy Rollback Procedure

## When to rollback

- Smoke test fails after deploy
- Error rate > 5% in first 15 minutes
- Critical bug reported in production

## ECS Rollback (preferred)

1. Identify the previous task definition revision:
   ```bash
   aws ecs describe-services --cluster nexus-prod --services nexus-core \
     --query 'services[0].taskDefinition'
   ```

2. Update service to previous revision:
   ```bash
   aws ecs update-service --cluster nexus-prod \
     --service nexus-core \
     --task-definition nexus-core:<PREVIOUS_REVISION> \
     --force-new-deployment
   ```

3. Wait for stable:
   ```bash
   aws ecs wait services-stable --cluster nexus-prod --services nexus-core
   ```

4. Run smoke test:
   ```bash
   ./scripts/smoke/smoke_prod.sh https://api.nexus.io https://saas.nexus.io https://app.nexus.io
   ```

## Database rollback

If the deploy included a migration:

1. Check the current migration version:
   ```bash
   SELECT version, dirty FROM schema_migrations;
   ```

2. Run the down migration:
   ```bash
   migrate -path migrations/ -database "$DATABASE_URL" down 1
   ```

3. **WARNING**: Down migrations may cause data loss. Always review the `.down.sql` first.

## CloudFront rollback (frontend)

1. Re-deploy previous S3 assets from ECR image:
   ```bash
   aws s3 sync s3://nexus-assets-backup/ s3://nexus-assets-prod/
   ```

2. Invalidate CDN cache:
   ```bash
   aws cloudfront create-invalidation --distribution-id $CF_DIST_ID --paths "/*"
   ```

## Post-rollback

- [ ] Verify all smoke tests pass
- [ ] Notify team in #incidents channel
- [ ] Create post-mortem ticket
- [ ] Disable failed deploy branch protection if needed
```

---

## 7. API docs: versioning & deprecation policy

Crear `docs/api/VERSIONING_POLICY.md`:

```markdown
# API Versioning & Deprecation Policy

## Versioning scheme

All API endpoints are prefixed with a version: `/v1/tools`, `/v1/run`, etc.

### Rules

- **Major version** (`/v1/`, `/v2/`): Breaking changes — new version prefix.
- **Minor additions**: New fields, new endpoints — backwards compatible, same version.
- **No removal without deprecation**: Fields and endpoints are never removed without notice.

## Deprecation process

1. **Announce**: Deprecated items get `X-Nexus-Deprecated: true` header + field in OpenAPI spec.
2. **Grace period**: Minimum **6 months** from announcement before removal.
3. **Sunset header**: Responses include `Sunset: <date>` header per RFC 8594.
4. **Communication**: Email to all org admins + changelog entry + developer portal notice.
5. **Removal**: After sunset date, endpoint returns `410 Gone`.

## Current API versions

| Service | Current | Status |
|---------|---------|--------|
| nexus-core | v1 | Stable |
| nexus-saas | v1 | Stable |

## SDK compatibility

SDKs (Go, TypeScript, Python) target the latest stable API version.
Older SDK versions continue to work until the API version is sunset.
```

---

## 8. Monitoring: trace ID en logs estructurados

Archivo: `pkgs/go-pkg/http/middlewares/gin/request_id.go`

Agregar middleware que extrae `trace_id` y `span_id` de OpenTelemetry y los inyecta en el zerolog context:

Crear `pkgs/go-pkg/http/middlewares/gin/trace_context.go`:

```go
package ginmw

import (
    "github.com/gin-gonic/gin"
    "go.opentelemetry.io/otel/trace"
)

func TraceContext() gin.HandlerFunc {
    return func(c *gin.Context) {
        span := trace.SpanFromContext(c.Request.Context())
        sc := span.SpanContext()
        if sc.HasTraceID() {
            c.Set("trace_id", sc.TraceID().String())
            c.Set("span_id", sc.SpanID().String())
        }
        c.Next()
    }
}
```

Registrar este middleware **después** de OTel (otelgin) y **antes** del request logger en `bootstrap_routes.go` de nexus-core y nexus-saas.

En el structured logger, leer `trace_id` y `span_id` del context y agregarlos al log entry. Así cada línea de log tiene correlación con las trazas distribuidas.

Test unitario en `pkgs/go-pkg/http/middlewares/gin/trace_context_test.go`.

---

## 9. Email/notifications: notificaciones in-app (bell icon)

### 9.1 Backend: endpoint de notificaciones no leídas

Archivo: `nexus-saas/internal/notifications/handler.go`

Agregar endpoints:
- `GET /v1/notifications` — lista notificaciones del usuario (paginadas, más recientes primero)
- `GET /v1/notifications/unread-count` — retorna `{ "count": N }`
- `PUT /v1/notifications/:id/read` — marca como leída

Modelo en repo:

```go
type InAppNotification struct {
    ID        uuid.UUID  `json:"id"`
    OrgID     uuid.UUID  `json:"org_id"`
    ActorID   string     `json:"actor_id"`
    Type      string     `json:"type"`
    Title     string     `json:"title"`
    Body      string     `json:"body"`
    ReadAt    *time.Time `json:"read_at"`
    CreatedAt time.Time  `json:"created_at"`
}
```

Migration `nexus-saas/migrations/0007_in_app_notifications.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS in_app_notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL,
    actor_id   text NOT NULL DEFAULT '',
    type       text NOT NULL,
    title      text NOT NULL,
    body       text NOT NULL DEFAULT '',
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_inapp_notif_org_unread ON in_app_notifications (org_id, read_at) WHERE read_at IS NULL;
```

Down migration: `DROP TABLE IF EXISTS in_app_notifications;`

### 9.2 Frontend: notification bell

Archivo: `nexus-tower/src/components/NotificationBell.tsx`

- Ícono de campana en el header/navbar
- Badge con unread count (polling cada 30s o via TanStack Query refetchInterval)
- Dropdown con las últimas 10 notificaciones
- Click en notificación → marca como leída + navega según tipo
- Link "View all" → `/notifications` page

### 9.3 Dual delivery

Cuando se envía una notificación por email, **también** crear el registro en `in_app_notifications`. Modificar `nexus-saas/internal/notifications/usecases.go` para insertar en ambos canales.

---

## 10. DB backup/DR: restore test script

Crear `scripts/dr/test_restore.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Tests RDS snapshot restore to a temporary instance
# Usage: ./test_restore.sh <snapshot-id> <temp-instance-id>

SNAPSHOT_ID="${1:?Usage: $0 <snapshot-id> <temp-instance-id>}"
TEMP_INSTANCE="${2:-nexus-restore-test-$(date +%Y%m%d)}"
DB_CLASS="${DB_CLASS:-db.t3.micro}"

echo "Restoring snapshot $SNAPSHOT_ID to $TEMP_INSTANCE..."

aws rds restore-db-instance-from-db-snapshot \
  --db-instance-identifier "$TEMP_INSTANCE" \
  --db-snapshot-identifier "$SNAPSHOT_ID" \
  --db-instance-class "$DB_CLASS" \
  --no-multi-az \
  --tags Key=Purpose,Value=restore-test Key=AutoDelete,Value=true

echo "Waiting for instance to become available..."
aws rds wait db-instance-available --db-instance-identifier "$TEMP_INSTANCE"

ENDPOINT=$(aws rds describe-db-instances \
  --db-instance-identifier "$TEMP_INSTANCE" \
  --query 'DBInstances[0].Endpoint.Address' --output text)

echo "Restore test instance available at: $ENDPOINT"
echo "Running basic connectivity check..."

PGPASSWORD="$DB_PASSWORD" psql -h "$ENDPOINT" -U "$DB_USER" -d nexus_core -c "SELECT COUNT(*) FROM tools;" && echo "Core DB: OK"
PGPASSWORD="$DB_PASSWORD" psql -h "$ENDPOINT" -U "$DB_USER" -d nexus_saas -c "SELECT COUNT(*) FROM tenant_settings;" && echo "SaaS DB: OK"

echo "Cleaning up test instance..."
aws rds delete-db-instance \
  --db-instance-identifier "$TEMP_INSTANCE" \
  --skip-final-snapshot

echo "Restore test completed successfully."
```

Documentar en `docs/runbooks/DB_BACKUP_DR.md` como sección adicional "Periodic Restore Test".

---

## Criterios de aceptación
Estos criterios se consideran obligatorios para dar por cerrado el hardening final.


| # | Criterio | Verificación |
|---|----------|-------------|
| 1 | `SECURITY.md` existe en raíz | `cat SECURITY.md` |
| 2 | CI falla en vulns críticas Go/npm | `continue-on-error` removido en govulncheck y npm audit |
| 3 | Rate limit por tenant en nexus-saas | Test: 200+ requests rápidas → 429 |
| 4 | Brute-force protection en auth | Test: 10 auth failures → IP bloqueada 15 min |
| 5 | Dead letter log en operators | Archivo JSONL con eventos fallidos |
| 6 | Grace period + dunning documentado | `docs/billing/GRACE_PERIOD_POLICY.md` |
| 7 | Dunning worker auto-suspende | Worker con ticker 24h |
| 8 | Usage threshold notifications | 80%/95%/100% notificaciones |
| 9 | Deploy rollback runbook | `docs/runbooks/DEPLOY_ROLLBACK.md` |
| 10 | API versioning policy | `docs/api/VERSIONING_POLICY.md` |
| 11 | Trace ID en logs | `trace_id` + `span_id` en cada log entry |
| 12 | Notificaciones in-app (bell) | GET /v1/notifications + bell en Tower |
| 13 | Restore test script | `scripts/dr/test_restore.sh` |
| 14 | Todo compila | `go build ./...` en los 3 servicios Go + `npm run build` en Tower |

## Notas importantes

- NO crear archivos fuera del monorepo
- NO modificar la estructura de carpetas existente
- Reusar patterns existentes (middlewares en `pkgs/go-pkg/`, templates en notifications)
- Tests unitarios para toda lógica nueva
- Mantener backwards compatibility total
