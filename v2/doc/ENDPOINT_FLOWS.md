# Nexus v2 Endpoint Flows

Relacionado:

- [README.md](README.md)
- [DEFINITION.md](DEFINITION.md)
- [MVP.md](MVP.md)
- [TECHNICAL_REFERENCE.md](TECHNICAL_REFERENCE.md)

Agrupado por servicio.

Nota:

- `audit` no sigue CRUD: write interno en `control-plane /internal/audit` y lectura admin en `control-plane /v1/audit`
- los CRUD handlers comparten `v2/pkgs/go-pkg/handlers` y `qa` valida ese patron
- el bootstrap HTTP transversal usa `v2/pkgs/go-pkg/httpserver`
- `qa` valida tambien `v2/pkgs/go-pkg`
- `make milestone = qa + acceptance`
- los endpoints de negocio requieren API key; `/healthz` y `/readyz` quedan libres
- `/metrics` tambien requiere API key
- `control-plane`, `data-plane` y `control-workers` validan auth inbound con `NEXUS_API_KEYS`
- `data-plane` autentica salidas hacia `control-plane` y `control-workers` con API keys de servicio
- `control-workers` autentica salidas hacia `control-plane` con API key de servicio
- `resources`, `policies`, `audit`, `actions`, `incidents` y `alerts` pueden correr en memoria o con PostgreSQL segun config
- `docker compose` y el ALB de `v2/infra` usan `/readyz` como probe de readiness

## Endpoints

### `control-workers POST /v1/incidents`

- `incidents.Handler.Register`
- `incidents.Handler.create`
- `handlers.DecodeJSON`
- `incidents.Usecases.Create`
- `incidents.normalizeCreate`
- `incidents.deriveSeverity`
- `incidents.deriveSummary`
- `incidents.InMemoryRepository.Create` o `incidents.PostgresRepository.Create`
- `incidents.Usecases.emitAudit`
- `[si NEXUS_CONTROL_PLANE_URL] audit.Client.Create`
- `incidents.Usecases.emitAlert`
- `[si severity=high|critical] alerts.Usecases.Create`
- `incidents.toIncidentResponse`
- `incidents.writeIncidentUsecaseError`
- `incidents.writeIncidentError`
- `handlers.WriteJSON`

### `control-workers POST /v1/alerts`

- `alerts.Handler.Register`
- `alerts.Handler.create`
- `handlers.DecodeJSON`
- `alerts.Usecases.Create`
- `alerts.normalizeCreate`
- `alerts.InMemoryRepository.Create` o `alerts.PostgresRepository.Create`
- `alerts.Usecases.emitAudit`
- `[si NEXUS_CONTROL_PLANE_URL] audit.Client.Create`
- `alerts.toAlertResponse`
- `alerts.writeAlertUsecaseError`
- `alerts.writeAlertError`
- `handlers.WriteJSON`

### `control-workers GET /v1/alerts`

- `alerts.Handler.Register`
- `alerts.Handler.list`
- `handlers.ParseLimit`
- `handlers.ParseArchived`
- `alerts.Usecases.List`
- `alerts.InMemoryRepository.List` o `alerts.PostgresRepository.List`
- `alerts.toAlertResponse`
- `alerts.writeAlertUsecaseError`
- `alerts.writeAlertError`
- `handlers.WriteJSON`

### `control-workers GET /v1/alerts/{id}`

- `alerts.Handler.Register`
- `alerts.Handler.getByID`
- `alerts.parseAlertID`
- `alerts.Usecases.GetByID`
- `alerts.InMemoryRepository.GetByID` o `alerts.PostgresRepository.GetByID`
- `alerts.mapRepoErr`
- `alerts.toAlertResponse`
- `alerts.writeAlertUsecaseError`
- `alerts.writeAlertError`
- `handlers.WriteJSON`

### `control-workers PATCH /v1/alerts/{id}`

- `alerts.Handler.Register`
- `alerts.Handler.updateByID`
- `alerts.parseAlertID`
- `handlers.DecodeJSON`
- `alerts.Usecases.UpdateByID`
- `alerts.InMemoryRepository.GetByID` o `alerts.PostgresRepository.GetByID`
- `alerts.InMemoryRepository.Update` o `alerts.PostgresRepository.Update`
- `alerts.mapRepoErr`
- `alerts.toAlertResponse`
- `alerts.writeAlertUsecaseError`
- `alerts.writeAlertError`
- `handlers.WriteJSON`

### `control-workers DELETE /v1/alerts/{id}`

- `alerts.Handler.Register`
- `alerts.Handler.deleteByID`
- `alerts.parseAlertID`
- `alerts.Usecases.DeleteByID`
- `alerts.InMemoryRepository.Delete` o `alerts.PostgresRepository.Delete`
- `alerts.mapRepoErr`
- `alerts.writeAlertUsecaseError`
- `alerts.writeAlertError`

### `control-workers POST /v1/alerts/{id}/archive`

- `alerts.Handler.Register`
- `alerts.Handler.archiveByID`
- `alerts.parseAlertID`
- `alerts.Usecases.ArchiveByID`
- `alerts.InMemoryRepository.Archive` o `alerts.PostgresRepository.Archive`
- `alerts.mapRepoErr`
- `alerts.toAlertResponse`
- `alerts.writeAlertUsecaseError`
- `alerts.writeAlertError`
- `handlers.WriteJSON`

### `control-workers POST /v1/alerts/{id}/restore`

- `alerts.Handler.Register`
- `alerts.Handler.restoreByID`
- `alerts.parseAlertID`
- `alerts.Usecases.RestoreByID`
- `alerts.InMemoryRepository.Restore` o `alerts.PostgresRepository.Restore`
- `alerts.mapRepoErr`
- `alerts.toAlertResponse`
- `alerts.writeAlertUsecaseError`
- `alerts.writeAlertError`
- `handlers.WriteJSON`

### `control-workers GET /v1/incidents`

- `incidents.Handler.Register`
- `incidents.Handler.list`
- `handlers.ParseLimit`
- `handlers.ParseArchived`
- `incidents.Usecases.List`
- `incidents.InMemoryRepository.List` o `incidents.PostgresRepository.List`
- `incidents.toIncidentResponse`
- `incidents.writeIncidentUsecaseError`
- `incidents.writeIncidentError`
- `handlers.WriteJSON`

### `control-workers GET /v1/incidents/{id}`

- `incidents.Handler.Register`
- `incidents.Handler.getByID`
- `incidents.parseIncidentID`
- `incidents.Usecases.GetByID`
- `incidents.InMemoryRepository.GetByID` o `incidents.PostgresRepository.GetByID`
- `incidents.mapRepoErr`
- `incidents.toIncidentResponse`
- `incidents.writeIncidentUsecaseError`
- `incidents.writeIncidentError`
- `handlers.WriteJSON`

### `control-workers PATCH /v1/incidents/{id}`

- `incidents.Handler.Register`
- `incidents.Handler.updateByID`
- `incidents.parseIncidentID`
- `handlers.DecodeJSON`
- `incidents.Usecases.UpdateByID`
- `incidents.InMemoryRepository.GetByID` o `incidents.PostgresRepository.GetByID`
- `incidents.InMemoryRepository.Update` o `incidents.PostgresRepository.Update`
- `incidents.mapRepoErr`
- `incidents.toIncidentResponse`
- `incidents.writeIncidentUsecaseError`
- `incidents.writeIncidentError`
- `handlers.WriteJSON`

### `control-workers DELETE /v1/incidents/{id}`

- `incidents.Handler.Register`
- `incidents.Handler.deleteByID`
- `incidents.parseIncidentID`
- `incidents.Usecases.DeleteByID`
- `incidents.InMemoryRepository.Delete` o `incidents.PostgresRepository.Delete`
- `incidents.mapRepoErr`
- `incidents.writeIncidentUsecaseError`
- `incidents.writeIncidentError`

### `control-workers POST /v1/incidents/{id}/archive`

- `incidents.Handler.Register`
- `incidents.Handler.archiveByID`
- `incidents.parseIncidentID`
- `incidents.Usecases.ArchiveByID`
- `incidents.InMemoryRepository.Archive` o `incidents.PostgresRepository.Archive`
- `incidents.mapRepoErr`
- `incidents.toIncidentResponse`
- `incidents.writeIncidentUsecaseError`
- `incidents.writeIncidentError`
- `handlers.WriteJSON`

### `control-workers POST /v1/incidents/{id}/restore`

- `incidents.Handler.Register`
- `incidents.Handler.restoreByID`
- `incidents.parseIncidentID`
- `incidents.Usecases.RestoreByID`
- `incidents.InMemoryRepository.Restore` o `incidents.PostgresRepository.Restore`
- `incidents.mapRepoErr`
- `incidents.toIncidentResponse`
- `incidents.writeIncidentUsecaseError`
- `incidents.writeIncidentError`
- `handlers.WriteJSON`

### `control-plane POST /internal/audit`

- `audit.Handler.Register`
- `audit.Handler.createInternal`
- `handlers.DecodeJSON`
- `audit.Usecases.Create`
- `audit.InMemoryRepository.Create` o `audit.PostgresRepository.Create`
- `handlers.WriteJSON`

### `control-plane GET /v1/audit`

- `audit.Handler.Register`
- `audit.Handler.list`
- `handlers.ParseLimit`
- `audit.parseRFC3339`
- `audit.Usecases.List`
- `audit.InMemoryRepository.List` o `audit.PostgresRepository.List`
- `handlers.WriteJSON`

### `control-plane GET /v1/audit/{id}`

- `audit.Handler.Register`
- `audit.Handler.getByID`
- `audit.parseAuditID`
- `audit.Usecases.GetByID`
- `audit.InMemoryRepository.GetByID` o `audit.PostgresRepository.GetByID`
- `handlers.WriteJSON`

### `control-plane POST /v1/resources`

- `resources.Handler.Register`
- `resources.Handler.create`
- `actors.FromRequest`
- `handlers.DecodeJSON`
- `resources.Usecases.Create`
- `resources.normalizeCreate`
- `resources.validateType`
- `resources.validateCriticality`
- `resources.InMemoryRepository.Create` o `resources.PostgresRepository.Create`
- `resources.Handler.emitAudit`
- `audit.SinkAdapter.Write`
- `resources.toResourceResponse`
- `resources.writeResourceUsecaseError`
- `resources.writeResourceError`
- `handlers.WriteJSON`

Notas:

- si `is_canary=true`, `control-plane` agrega la label interna `_nexus_trap=true`
- esa marca se persiste en `resources` y no requiere una policy por recurso

### `control-plane GET /v1/resources`

- `resources.Handler.Register`
- `resources.Handler.list`
- `handlers.ParseLimit`
- `handlers.ParseArchived`
- `resources.Usecases.List`
- `resources.validateType`
- `resources.InMemoryRepository.List` o `resources.PostgresRepository.List`
- `resources.toResourceResponse`
- `resources.writeResourceUsecaseError`
- `resources.writeResourceError`
- `handlers.WriteJSON`

### `control-plane GET /v1/resources/{id}`

- `resources.Handler.Register`
- `resources.Handler.getByID`
- `resources.parseResourceID`
- `resources.Usecases.GetByID`
- `resources.InMemoryRepository.GetByID` o `resources.PostgresRepository.GetByID`
- `resources.mapRepoErr`
- `resources.toResourceResponse`
- `resources.writeResourceUsecaseError`
- `resources.writeResourceError`
- `handlers.WriteJSON`

### `control-plane PATCH /v1/resources/{id}`

- `resources.Handler.Register`
- `resources.Handler.updateByID`
- `resources.parseResourceID`
- `handlers.DecodeJSON`
- `resources.Usecases.UpdateByID`
- `resources.validateType`
- `resources.validateCriticality`
- `resources.Usecases.GetByID`
- `resources.InMemoryRepository.GetByID` o `resources.PostgresRepository.GetByID`
- `resources.InMemoryRepository.Update` o `resources.PostgresRepository.Update`
- `resources.mapRepoErr`
- `resources.toResourceResponse`
- `resources.writeResourceUsecaseError`
- `resources.writeResourceError`
- `handlers.WriteJSON`

### `control-plane DELETE /v1/resources/{id}`

- `resources.Handler.Register`
- `resources.Handler.deleteByID`
- `resources.parseResourceID`
- `resources.Usecases.DeleteByID`
- `resources.InMemoryRepository.Delete` o `resources.PostgresRepository.Delete`
- `resources.mapRepoErr`

### `control-plane POST /v1/resources/{id}/archive`

- `resources.Handler.Register`
- `resources.Handler.archiveByID`
- `resources.parseResourceID`
- `resources.Usecases.ArchiveByID`
- `resources.InMemoryRepository.Archive` o `resources.PostgresRepository.Archive`
- `resources.mapRepoErr`
- `resources.toResourceResponse`
- `resources.writeResourceUsecaseError`
- `resources.writeResourceError`
- `handlers.WriteJSON`

### `control-plane POST /v1/resources/{id}/restore`

- `resources.Handler.Register`
- `resources.Handler.restoreByID`
- `resources.parseResourceID`
- `resources.Usecases.RestoreByID`
- `resources.InMemoryRepository.Restore` o `resources.PostgresRepository.Restore`
- `resources.mapRepoErr`
- `resources.toResourceResponse`
- `resources.writeResourceUsecaseError`
- `resources.writeResourceError`
- `handlers.WriteJSON`

### `control-plane POST /v1/policies`

- `policies.Handler.Register`
- `policies.Handler.create`
- `actors.FromRequest`
- `handlers.DecodeJSON`
- `policies.Usecases.Create`
- `policies.Evaluator.Validate`
- `policies.InMemoryRepository.Create` o `policies.PostgresRepository.Create`
- `policies.Handler.emitAudit`
- `audit.SinkAdapter.Write`
- `policies.toPolicyResponse`
- `policies.writePolicyUsecaseError`
- `policies.writePolicyError`
- `handlers.WriteJSON`

Notas:

- `control-plane` garantiza una trap policy builtin `is_trap=true`
- el listado por `action_type` y `resource_type` tambien devuelve wildcard policies (`*`)

### `control-plane GET /v1/policies`

- `policies.Handler.Register`
- `policies.Handler.list`
- `handlers.ParseArchived`
- `policies.Usecases.List`
- `policies.InMemoryRepository.List` o `policies.PostgresRepository.List`
- `policies.toPolicyResponse`
- `policies.writePolicyUsecaseError`
- `policies.writePolicyError`
- `handlers.WriteJSON`

### `control-plane GET /v1/policies/{id}`

- `policies.Handler.Register`
- `policies.Handler.getByID`
- `policies.parsePolicyID`
- `policies.Usecases.GetByID`
- `policies.InMemoryRepository.GetByID` o `policies.PostgresRepository.GetByID`
- `policies.mapRepoErr`
- `policies.toPolicyResponse`
- `policies.writePolicyUsecaseError`
- `policies.writePolicyError`
- `handlers.WriteJSON`

### `control-plane PATCH /v1/policies/{id}`

- `policies.Handler.Register`
- `policies.Handler.patchByID`
- `policies.parsePolicyID`
- `handlers.DecodeJSON`
- `policies.Usecases.UpdateByID`
- `policies.Evaluator.Validate`
- `policies.InMemoryRepository.GetByID` o `policies.PostgresRepository.GetByID`
- `policies.InMemoryRepository.Save` o `policies.PostgresRepository.Save`
- `policies.mapRepoErr`
- `policies.toPolicyResponse`
- `policies.writePolicyUsecaseError`
- `policies.writePolicyError`
- `handlers.WriteJSON`

### `control-plane DELETE /v1/policies/{id}`

- `policies.Handler.Register`
- `policies.Handler.deleteByID`
- `policies.parsePolicyID`
- `policies.Usecases.DeleteByID`
- `policies.InMemoryRepository.DeleteByID` o `policies.PostgresRepository.DeleteByID`
- `policies.mapRepoErr`
- `policies.writePolicyUsecaseError`
- `policies.writePolicyError`

### `control-plane POST /v1/policies/{id}/archive`

- `policies.Handler.Register`
- `policies.Handler.archiveByID`
- `policies.parsePolicyID`
- `policies.Usecases.ArchiveByID`
- `policies.InMemoryRepository.ArchiveByID` o `policies.PostgresRepository.ArchiveByID`
- `policies.mapRepoErr`
- `policies.toPolicyResponse`
- `policies.writePolicyUsecaseError`
- `policies.writePolicyError`
- `handlers.WriteJSON`

### `control-plane POST /v1/policies/{id}/restore`

- `policies.Handler.Register`
- `policies.Handler.restoreByID`
- `policies.parsePolicyID`
- `policies.Usecases.RestoreByID`
- `policies.InMemoryRepository.RestoreByID` o `policies.PostgresRepository.RestoreByID`
- `policies.mapRepoErr`
- `policies.toPolicyResponse`
- `policies.writePolicyUsecaseError`
- `policies.writePolicyError`
- `handlers.WriteJSON`

### `data-plane POST /v1/actions`

- `action.Handler.Register`
- `action.Handler.create`
- `[si Idempotency-Key header] action.IdempotencyStore.Get`
- `[si key existe y valido] retorna respuesta cacheada con X-Idempotency-Replay: true`
- `handlers.DecodeJSON`
- `action.Usecases.Create`
- `action.WithDegradationCollector(ctx)` — crea collector per-request en context
- `action.validateActionType`
- `action.validateResourceType`
- `action.validateActor`
- `action.Usecases.resolveResource`
- `[si hay control-plane] action.CachingResourceResolver.GetByID (cache + fallback)`
- `[si upstream fallo y cache valido] DegradationFromContext(ctx).resourceDegraded = true`
- `action.Usecases.listPolicies`
- `[si hay control-plane] action.CachingPolicySource.List (cache + fallback)`
- `[si upstream fallo y cache valido] DegradationFromContext(ctx).policiesDegraded = true`
- `action.evaluateAction`
- `action.normalizePayload`
- `action.riskFor`
- `action.HistoricalRiskContextProvider.ContextFor` — provee baselines, known_destinations, incident context
- `action.buildEvidence`
- `action.evaluatePolicyDecision`
- `[si hay policy CEL] action.ActionPolicyEvaluator.Matches`
- `action.InMemoryRepository.Create` o `action.PostgresRepository.Create`
- `[si DegradationFromContext(ctx).IsDegraded()] auditData["degraded_context"] = true`
- `[si hay control-plane] action.Usecases.emitAudit`
- `[si hay control-plane] audit.Client.Create`
- `[si hay control-workers y la accion queda blocked] action.Usecases.emitIncident`
- `[si hay control-workers] action.ControlWorkersClient.Create`
- `[si Idempotency-Key header] action.IdempotencyStore.Set`
- `action.toActionResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

Notas:

- el evaluator de riesgo usa baselines por `resource` y `actor`, destinos conocidos e incidentes abiertos
- si matchea una policy con `is_trap=true`, el incidente emitido usa `trigger=canary_triggered`
- `recommended_decision` sigue siendo informativa; el lifecycle real lo decide policy evaluation
- si la decision se tomo con datos de cache (control-plane no disponible), el audit record incluye `degraded_context: true`

### `data-plane GET /v1/actions`

- `action.Handler.Register`
- `action.Handler.list`
- `handlers.ParseLimit`
- `action.Usecases.List`
- `action.validateActionType`
- `action.validateStatus`
- `action.InMemoryRepository.List` o `action.PostgresRepository.List`
- `action.toActionResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

### `data-plane GET /v1/actions/{id}`

- `action.Handler.Register`
- `action.Handler.getByID`
- `action.parseActionID`
- `action.Usecases.GetByID`
- `action.InMemoryRepository.GetByID` o `action.PostgresRepository.GetByID`
- `action.mapRepoErr`
- `action.toActionResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

### `data-plane GET /v1/actions/{id}/risk`

- `action.Handler.Register`
- `action.Handler.getRisk`
- `action.parseActionID`
- `action.Usecases.GetRisk`
- `action.Usecases.GetByID`
- `action.InMemoryRepository.GetByID` o `action.PostgresRepository.GetByID`
- `actionrisk.Evaluator`
- `action.toRiskResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

Respuesta actual:

- `level`
- `score`
- `summary`
- `profile`
- `risk_pressure`
- `safety_pressure`
- `raw_score`
- `decision_score`
- `recommended_decision`
- `factors`
- `amplifications`
- `attenuations`

Notas:

- `profile` hoy es builtin `balanced/v1`
- `factors` devuelve los 10 factores de `1A`, activos o inactivos
- `evidence_quality` sale por factor y explica si la evidencia fue observada, inferida, missing o stale

### `data-plane GET /v1/actions/{id}/evidence`

- `action.Handler.Register`
- `action.Handler.getEvidence`
- `action.parseActionID`
- `action.Usecases.GetEvidence`
- `action.Usecases.GetByID`
- `action.InMemoryRepository.GetByID` o `action.PostgresRepository.GetByID`
- `action.toEvidenceRecordResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

### `data-plane POST /v1/actions/{id}/approve`

- `action.Handler.Register`
- `action.Handler.approve`
- `action.Handler.decide`
- `action.parseActionID`
- `handlers.DecodeJSON`
- `action.Usecases.Approve`
- `action.validateActor`
- `action.Usecases.GetByID`
- `action.InMemoryRepository.GetByID` o `action.PostgresRepository.GetByID`
- `action.InMemoryRepository.Decide` o `action.PostgresRepository.Decide`
- `[si hay control-plane] action.Usecases.emitAudit`
- `[si hay control-plane] audit.Client.Create`
- `action.mapRepoErr`
- `action.toActionResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

### `data-plane POST /v1/actions/{id}/reject`

- `action.Handler.Register`
- `action.Handler.reject`
- `action.Handler.decide`
- `action.parseActionID`
- `handlers.DecodeJSON`
- `action.Usecases.Reject`
- `action.validateActor`
- `action.Usecases.GetByID`
- `action.InMemoryRepository.GetByID` o `action.PostgresRepository.GetByID`
- `action.InMemoryRepository.Decide` o `action.PostgresRepository.Decide`
- `[si hay control-plane] action.Usecases.emitAudit`
- `[si hay control-plane] audit.Client.Create`
- `[si hay control-workers] action.Usecases.emitIncident`
- `[si hay control-workers] action.ControlWorkersClient.Create`
- `action.mapRepoErr`
- `action.toActionResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

### `data-plane POST /v1/actions/{id}/lease`

- `action.Handler.Register`
- `action.Handler.issueLease`
- `action.parseActionID`
- `action.Usecases.IssueLease`
- `action.Usecases.GetByID`
- `action.InMemoryRepository.GetByID` o `action.PostgresRepository.GetByID`
- `action.allEvidencePassed`
- `action.leaseTTL`
- `action.InMemoryRepository.IssueLease` o `action.PostgresRepository.IssueLease`
- `[si hay control-plane] action.Usecases.emitAudit`
- `[si hay control-plane] audit.Client.Create`
- `action.mapRepoErr`
- `action.toActionResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

### `data-plane POST /v1/actions/{id}/execute`

- `action.Handler.Register`
- `action.Handler.execute`
- `action.parseActionID`
- `handlers.DecodeJSON`
- `action.Usecases.Execute`
- `action.validateActor`
- `action.Usecases.GetByID`
- `action.InMemoryRepository.GetByID` o `action.PostgresRepository.GetByID`
- `action.DeterministicExecutor.Execute`
- `[si execute falla y hay control-plane] action.Usecases.emitAudit`
- `[si execute falla y hay control-plane] audit.Client.Create`
- `[si execute falla y hay control-workers] action.Usecases.emitIncident`
- `[si execute falla y hay control-workers] action.ControlWorkersClient.Create`
- `action.InMemoryRepository.ConsumeLeaseAndMarkExecuted` o `action.PostgresRepository.ConsumeLeaseAndMarkExecuted`
- `[si execute success y hay control-plane] action.Usecases.emitAudit`
- `[si execute success y hay control-plane] audit.Client.Create`
- `action.mapRepoErr`
- `action.toActionResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

Detalles en `TECHNICAL_REFERENCE.md`.
