# V2

Agrupado por servicio.

Nota:

- hoy existen dos superficies distintas con path `/v1/policies`
- `control-plane /v1/policies`
- `data-plane /v1/policies` legacy para `/run`
- los CRUD handlers comparten `v2/pkgs/go-pkg/handlers` y `qa` valida ese patron

## Endpoints

### `control-workers POST /v1/incidents`

- `incidents.Handler.Register`
- `incidents.Handler.create`
- `handlers.DecodeJSON`
- `incidents.Usecases.Create`
- `incidents.normalizeCreate`
- `incidents.deriveSeverity`
- `incidents.deriveSummary`
- `incidents.InMemoryRepository.Create`
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
- `alerts.InMemoryRepository.Create`
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
- `alerts.InMemoryRepository.List`
- `alerts.toAlertResponse`
- `alerts.writeAlertUsecaseError`
- `alerts.writeAlertError`
- `handlers.WriteJSON`

### `control-workers GET /v1/alerts/{id}`

- `alerts.Handler.Register`
- `alerts.Handler.getByID`
- `alerts.parseAlertID`
- `alerts.Usecases.GetByID`
- `alerts.InMemoryRepository.GetByID`
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
- `alerts.InMemoryRepository.GetByID`
- `alerts.InMemoryRepository.Update`
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
- `alerts.InMemoryRepository.Delete`
- `alerts.mapRepoErr`
- `alerts.writeAlertUsecaseError`
- `alerts.writeAlertError`

### `control-workers POST /v1/alerts/{id}/archive`

- `alerts.Handler.Register`
- `alerts.Handler.archiveByID`
- `alerts.parseAlertID`
- `alerts.Usecases.ArchiveByID`
- `alerts.InMemoryRepository.Archive`
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
- `alerts.InMemoryRepository.Restore`
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
- `incidents.InMemoryRepository.List`
- `incidents.toIncidentResponse`
- `incidents.writeIncidentUsecaseError`
- `incidents.writeIncidentError`
- `handlers.WriteJSON`

### `control-workers GET /v1/incidents/{id}`

- `incidents.Handler.Register`
- `incidents.Handler.getByID`
- `incidents.parseIncidentID`
- `incidents.Usecases.GetByID`
- `incidents.InMemoryRepository.GetByID`
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
- `incidents.InMemoryRepository.GetByID`
- `incidents.InMemoryRepository.Update`
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
- `incidents.InMemoryRepository.Delete`
- `incidents.mapRepoErr`
- `incidents.writeIncidentUsecaseError`
- `incidents.writeIncidentError`

### `control-workers POST /v1/incidents/{id}/archive`

- `incidents.Handler.Register`
- `incidents.Handler.archiveByID`
- `incidents.parseIncidentID`
- `incidents.Usecases.ArchiveByID`
- `incidents.InMemoryRepository.Archive`
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
- `incidents.InMemoryRepository.Restore`
- `incidents.mapRepoErr`
- `incidents.toIncidentResponse`
- `incidents.writeIncidentUsecaseError`
- `incidents.writeIncidentError`
- `handlers.WriteJSON`

### `control-plane POST /v1/resources`

- `resources.Handler.Register`
- `resources.Handler.create`
- `handlers.DecodeJSON`
- `resources.Usecases.Create`
- `resources.normalizeCreate`
- `resources.validateType`
- `resources.validateCriticality`
- `resources.InMemoryRepository.Create`
- `resources.toResourceResponse`
- `resources.writeResourceUsecaseError`
- `resources.writeResourceError`
- `handlers.WriteJSON`

### `control-plane GET /v1/resources`

- `resources.Handler.Register`
- `resources.Handler.list`
- `handlers.ParseLimit`
- `handlers.ParseArchived`
- `resources.Usecases.List`
- `resources.validateType`
- `resources.InMemoryRepository.List`
- `resources.toResourceResponse`
- `resources.writeResourceUsecaseError`
- `resources.writeResourceError`
- `handlers.WriteJSON`

### `control-plane GET /v1/resources/{id}`

- `resources.Handler.Register`
- `resources.Handler.getByID`
- `resources.parseResourceID`
- `resources.Usecases.GetByID`
- `resources.InMemoryRepository.GetByID`
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
- `resources.InMemoryRepository.GetByID`
- `resources.InMemoryRepository.Update`
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
- `resources.InMemoryRepository.Delete`
- `resources.mapRepoErr`

### `control-plane POST /v1/resources/{id}/archive`

- `resources.Handler.Register`
- `resources.Handler.archiveByID`
- `resources.parseResourceID`
- `resources.Usecases.ArchiveByID`
- `resources.InMemoryRepository.Archive`
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
- `resources.InMemoryRepository.Restore`
- `resources.mapRepoErr`
- `resources.toResourceResponse`
- `resources.writeResourceUsecaseError`
- `resources.writeResourceError`
- `handlers.WriteJSON`

### `control-plane POST /v1/policies`

- `policies.Handler.Register`
- `policies.Handler.create`
- `handlers.DecodeJSON`
- `policies.Usecases.Create`
- `policies.Evaluator.Validate`
- `policies.InMemoryRepository.Create`
- `policies.toPolicyResponse`
- `policies.writePolicyUsecaseError`
- `policies.writePolicyError`
- `handlers.WriteJSON`

### `control-plane GET /v1/policies`

- `policies.Handler.Register`
- `policies.Handler.list`
- `handlers.ParseArchived`
- `policies.Usecases.List`
- `policies.InMemoryRepository.List`
- `policies.toPolicyResponse`
- `policies.writePolicyUsecaseError`
- `policies.writePolicyError`
- `handlers.WriteJSON`

### `control-plane GET /v1/policies/{id}`

- `policies.Handler.Register`
- `policies.Handler.getByID`
- `policies.parsePolicyID`
- `policies.Usecases.GetByID`
- `policies.InMemoryRepository.GetByID`
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
- `policies.InMemoryRepository.GetByID`
- `policies.InMemoryRepository.Save`
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
- `policies.InMemoryRepository.DeleteByID`
- `policies.mapRepoErr`
- `policies.writePolicyUsecaseError`
- `policies.writePolicyError`

### `control-plane POST /v1/policies/{id}/archive`

- `policies.Handler.Register`
- `policies.Handler.archiveByID`
- `policies.parsePolicyID`
- `policies.Usecases.ArchiveByID`
- `policies.InMemoryRepository.ArchiveByID`
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
- `policies.InMemoryRepository.RestoreByID`
- `policies.mapRepoErr`
- `policies.toPolicyResponse`
- `policies.writePolicyUsecaseError`
- `policies.writePolicyError`
- `handlers.WriteJSON`

### `data-plane POST /v1/actions`

- `action.Handler.Register`
- `action.Handler.create`
- `handlers.DecodeJSON`
- `action.Usecases.Create`
- `action.validateActionType`
- `action.validateResourceType`
- `action.validateActor`
- `action.Usecases.resolveResource`
- `[si hay control-plane] action.ControlPlaneClient.GetByID`
- `action.Usecases.listPolicies`
- `[si hay control-plane] action.ControlPlaneClient.List`
- `action.evaluateAction`
- `action.normalizePayload`
- `action.riskFor`
- `action.buildEvidence`
- `action.evaluatePolicyDecision`
- `[si hay policy CEL] action.ActionPolicyEvaluator.Matches`
- `action.InMemoryRepository.Create`
- `[si hay control-workers y la accion queda blocked] action.Usecases.emitIncident`
- `[si hay control-workers] action.ControlWorkersClient.Create`
- `action.toActionResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

### `data-plane GET /v1/actions`

- `action.Handler.Register`
- `action.Handler.list`
- `handlers.ParseLimit`
- `action.Usecases.List`
- `action.validateActionType`
- `action.validateStatus`
- `action.InMemoryRepository.List`
- `action.toActionResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

### `data-plane GET /v1/actions/{id}`

- `action.Handler.Register`
- `action.Handler.getByID`
- `action.parseActionID`
- `action.Usecases.GetByID`
- `action.InMemoryRepository.GetByID`
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
- `action.InMemoryRepository.GetByID`
- `action.toRiskResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

### `data-plane GET /v1/actions/{id}/evidence`

- `action.Handler.Register`
- `action.Handler.getEvidence`
- `action.parseActionID`
- `action.Usecases.GetEvidence`
- `action.Usecases.GetByID`
- `action.InMemoryRepository.GetByID`
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
- `action.InMemoryRepository.GetByID`
- `action.InMemoryRepository.Decide`
- `[si hay control-workers] action.Usecases.emitIncident`
- `[si hay control-workers] action.ControlWorkersClient.Create`
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
- `action.InMemoryRepository.GetByID`
- `action.InMemoryRepository.Decide`
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
- `action.InMemoryRepository.GetByID`
- `action.allEvidencePassed`
- `action.leaseTTL`
- `action.InMemoryRepository.IssueLease`
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
- `action.InMemoryRepository.GetByID`
- `action.DeterministicExecutor.Execute`
- `[si execute falla y hay control-workers] action.Usecases.emitIncident`
- `[si execute falla y hay control-workers] action.ControlWorkersClient.Create`
- `action.InMemoryRepository.ConsumeLeaseAndMarkExecuted`
- `action.mapRepoErr`
- `action.toActionResponse`
- `action.writeActionUsecaseError`
- `action.writeActionError`
- `handlers.WriteJSON`

### `data-plane POST /v1/run`

- `Handler.Register`
- `Handler.runTool`
- `parseIdempotencyKey`
- `Usecases.Run`
- `clampTimeoutMS`
- `resolveTool`
- `resolveIdempotency`
- `buildRequestFingerprint`
- `mapRunError`
- `toRunHTTPError`
- `validateAndPrepare`
- `decide`
- `classifyRiskClass`
- `evaluateDeterministicPreflight`
- `IntentRepository.Create`
- `ApprovalPort.RequestApproval`
- `IntentRepository.LinkApproval`
- `prepareExecution`
- `RateLimiter.Allow`
- `egress.Usecases.IsHostAllowed`
- `SecretRepository.ListForTool`
- `executeAndFinish`
- `markCompletedIdempotency`
- `markFailedIdempotency`
- `Executor.Execute`
- `writeRunResponse`
- `writeIdempotencyHeader`
- `handlers.WriteJSON`
- `writeError`

### `data-plane POST /v1/policies`

- `policy.Handler.Register`
- `policy.Handler.create`
- `handlers.DecodeJSON`
- `policy.Usecases.Create`
- `policy.Usecases.ensureToolExists`
- `policy.Evaluator.Validate`
- `InMemoryRepository.Create`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `handlers.WriteJSON`

### `data-plane GET /v1/policies`

- `policy.Handler.Register`
- `policy.Handler.list`
- `handlers.ParseArchived`
- `policy.Usecases.List`
- `InMemoryRepository.List`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `handlers.WriteJSON`

### `data-plane GET /v1/policies/{id}`

- `policy.Handler.Register`
- `policy.Handler.getByID`
- `policy.parsePolicyID`
- `policy.Usecases.GetByID`
- `InMemoryRepository.GetByID`
- `policy.mapRepoErr`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `handlers.WriteJSON`

### `data-plane PATCH /v1/policies/{id}`

- `policy.Handler.Register`
- `policy.Handler.patchByID`
- `policy.parsePolicyID`
- `handlers.DecodeJSON`
- `policy.Usecases.UpdateByID`
- `policy.Usecases.ensureToolExists`
- `policy.Evaluator.Validate`
- `InMemoryRepository.GetByID`
- `InMemoryRepository.Save`
- `policy.mapRepoErr`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `handlers.WriteJSON`

### `data-plane DELETE /v1/policies/{id}`

- `policy.Handler.Register`
- `policy.Handler.deleteByID`
- `policy.parsePolicyID`
- `policy.Usecases.DeleteByID`
- `InMemoryRepository.DeleteByID`
- `policy.mapRepoErr`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`

### `data-plane POST /v1/policies/{id}/archive`

- `policy.Handler.Register`
- `policy.Handler.archiveByID`
- `policy.parsePolicyID`
- `policy.Usecases.ArchiveByID`
- `InMemoryRepository.ArchiveByID`
- `policy.mapRepoErr`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `handlers.WriteJSON`

### `data-plane POST /v1/policies/{id}/restore`

- `policy.Handler.Register`
- `policy.Handler.restoreByID`
- `policy.parsePolicyID`
- `policy.Usecases.RestoreByID`
- `InMemoryRepository.RestoreByID`
- `policy.mapRepoErr`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `handlers.WriteJSON`

### `data-plane GET /v1/approvals`

- `approval.Handler.Register`
- `approval.Handler.listPending`
- `approval.Usecases.ListPending`
- `approval.InMemoryRepository.ListPending`
- `approval.toApprovalDTO`
- `approval.writeApprovalUsecaseError`
- `approval.writeApprovalError`
- `handlers.WriteJSON`

### `data-plane GET /v1/approvals/{id}`

- `approval.Handler.Register`
- `approval.Handler.getByID`
- `approval.parseApprovalID`
- `approval.Usecases.GetByID`
- `approval.InMemoryRepository.GetByID`
- `approval.toApprovalDTO`
- `approval.writeApprovalUsecaseError`
- `approval.writeApprovalError`
- `handlers.WriteJSON`

### `data-plane GET /v1/run/intents`

- `Handler.Register`
- `Handler.listIntents`
- `Usecases.ListIntents`
- `InMemoryIntentRepository.ListRecent`
- `toIntentDTO`
- `writeGatewayError`
- `handlers.WriteJSON`

### `data-plane GET /v1/run/intents/{id}`

- `Handler.Register`
- `Handler.getIntent`
- `Usecases.GetIntent`
- `InMemoryIntentRepository.GetByID`
- `toIntentDTO`
- `writeGatewayError`
- `handlers.WriteJSON`

### `data-plane GET /v1/run/intents/{id}/preflight`

- `Handler.Register`
- `Handler.getIntentPreflight`
- `Usecases.GetIntentPreflight`
- `Usecases.GetIntent`
- `InMemoryIntentRepository.GetByID`
- `toPreflightReviewDTO`
- `writeGatewayError`
- `handlers.WriteJSON`

### `data-plane POST /v1/run/intents/{id}/lease`

- `Handler.Register`
- `Handler.issueExecutionLease`
- `Usecases.IssueExecutionLease`
- `InMemoryIntentRepository.GetByID`
- `InMemoryLeaseRepository.Create`
- `toExecutionLeaseDTO`
- `writeGatewayError`
- `handlers.WriteJSON`

### `data-plane POST /v1/run/intents/{id}/execute`

- `Handler.Register`
- `Handler.executeIntent`
- `parseTimeoutMS`
- `Usecases.ExecuteIntentWithLease`
- `InMemoryIntentRepository.GetByID`
- `InMemoryLeaseRepository.Consume`
- `Run`
- `clampTimeoutMS`
- `resolveTool`
- `resolveIdempotency`
- `buildRequestFingerprint`
- `mapRunError`
- `toRunHTTPError`
- `validateAndPrepare`
- `decide`
- `prepareExecution`
- `executeAndFinish`
- `markCompletedIdempotency`
- `markFailedIdempotency`
- `Executor.Execute`
- `InMemoryIntentRepository.MarkExecuted`
- `writeGatewayError`
- `writeRunResponse`
- `writeIdempotencyHeader`
- `handlers.WriteJSON`
- `writeError`

### `data-plane POST /v1/approvals/{id}/approve`

- `approval.Handler.Register`
- `approval.Handler.approve`
- `approval.Handler.decide`
- `approval.parseApprovalID`
- `handlers.DecodeJSON`
- `approval.Usecases.Approve`
- `approval.InMemoryRepository.Decide`
- `IntentStatusPort.MarkApproved`
- `approval.writeApprovalUsecaseError`
- `approval.writeApprovalError`
- `handlers.WriteJSON`

### `data-plane POST /v1/approvals/{id}/reject`

- `approval.Handler.Register`
- `approval.Handler.reject`
- `approval.Handler.decide`
- `approval.parseApprovalID`
- `handlers.DecodeJSON`
- `approval.Usecases.Reject`
- `approval.InMemoryRepository.Decide`
- `IntentStatusPort.MarkRejected`
- `approval.writeApprovalUsecaseError`
- `approval.writeApprovalError`
- `handlers.WriteJSON`

Detalles en `DOC.md`.
