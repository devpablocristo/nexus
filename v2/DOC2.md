# V2

## Endpoints

### `POST /v1/run`

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
- `writeJSON`
- `writeError`

### `POST /v1/policies`

- `policy.Handler.Register`
- `policy.Handler.create`
- `policy.decodeJSON`
- `policy.Usecases.Create`
- `policy.Usecases.ensureToolExists`
- `policy.Evaluator.Validate`
- `InMemoryRepository.Create`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `policy.writeJSON`

### `GET /v1/policies`

- `policy.Handler.Register`
- `policy.Handler.list`
- `policy.parseOptionalBool`
- `policy.Usecases.List`
- `InMemoryRepository.List`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `policy.writeJSON`

### `GET /v1/policies/{id}`

- `policy.Handler.Register`
- `policy.Handler.getByID`
- `policy.parsePolicyID`
- `policy.Usecases.GetByID`
- `InMemoryRepository.GetByID`
- `policy.mapRepoErr`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `policy.writeJSON`

### `PATCH /v1/policies/{id}`

- `policy.Handler.Register`
- `policy.Handler.patchByID`
- `policy.parsePolicyID`
- `policy.decodeJSON`
- `policy.Usecases.UpdateByID`
- `policy.Usecases.ensureToolExists`
- `policy.Evaluator.Validate`
- `InMemoryRepository.GetByID`
- `InMemoryRepository.Save`
- `policy.mapRepoErr`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `policy.writeJSON`

### `DELETE /v1/policies/{id}`

- `policy.Handler.Register`
- `policy.Handler.deleteByID`
- `policy.parsePolicyID`
- `policy.Usecases.DeleteByID`
- `InMemoryRepository.DeleteByID`
- `policy.mapRepoErr`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`

### `POST /v1/policies/{id}/archive`

- `policy.Handler.Register`
- `policy.Handler.archiveByID`
- `policy.parsePolicyID`
- `policy.Usecases.ArchiveByID`
- `InMemoryRepository.ArchiveByID`
- `policy.mapRepoErr`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `policy.writeJSON`

### `POST /v1/policies/{id}/restore`

- `policy.Handler.Register`
- `policy.Handler.restoreByID`
- `policy.parsePolicyID`
- `policy.Usecases.RestoreByID`
- `InMemoryRepository.RestoreByID`
- `policy.mapRepoErr`
- `policy.toPolicyResponse`
- `policy.writePolicyUsecaseError`
- `policy.writePolicyError`
- `policy.writeJSON`

### `GET /v1/approvals`

- `approval.Handler.Register`
- `approval.Handler.listPending`
- `approval.Usecases.ListPending`
- `approval.InMemoryRepository.ListPending`
- `approval.toApprovalDTO`
- `approval.writeApprovalUsecaseError`
- `approval.writeApprovalError`
- `approval.writeApprovalJSON`

### `GET /v1/approvals/{id}`

- `approval.Handler.Register`
- `approval.Handler.getByID`
- `approval.parseApprovalID`
- `approval.Usecases.GetByID`
- `approval.InMemoryRepository.GetByID`
- `approval.toApprovalDTO`
- `approval.writeApprovalUsecaseError`
- `approval.writeApprovalError`
- `approval.writeApprovalJSON`

### `GET /v1/run/intents`

- `Handler.Register`
- `Handler.listIntents`
- `Usecases.ListIntents`
- `InMemoryIntentRepository.ListRecent`
- `toIntentDTO`
- `writeGatewayError`
- `writeJSON`

### `GET /v1/run/intents/{id}`

- `Handler.Register`
- `Handler.getIntent`
- `Usecases.GetIntent`
- `InMemoryIntentRepository.GetByID`
- `toIntentDTO`
- `writeGatewayError`
- `writeJSON`

### `POST /v1/run/intents/{id}/execute`

- `Handler.Register`
- `Handler.executeIntent`
- `parseTimeoutMS`
- `Usecases.ExecuteIntent`
- `InMemoryIntentRepository.GetByID`
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
- `writeJSON`
- `writeError`

### `POST /v1/approvals/{id}/approve`

- `approval.Handler.Register`
- `approval.Handler.approve`
- `approval.Handler.decide`
- `approval.parseApprovalID`
- `approval.decodeApprovalJSON`
- `approval.Usecases.Approve`
- `approval.InMemoryRepository.Decide`
- `IntentStatusPort.MarkApproved`
- `approval.writeApprovalUsecaseError`
- `approval.writeApprovalError`
- `approval.writeApprovalJSON`

### `POST /v1/approvals/{id}/reject`

- `approval.Handler.Register`
- `approval.Handler.reject`
- `approval.Handler.decide`
- `approval.parseApprovalID`
- `approval.decodeApprovalJSON`
- `approval.Usecases.Reject`
- `approval.InMemoryRepository.Decide`
- `IntentStatusPort.MarkRejected`
- `approval.writeApprovalUsecaseError`
- `approval.writeApprovalError`
- `approval.writeApprovalJSON`

Detalles en `DOC.md`.
