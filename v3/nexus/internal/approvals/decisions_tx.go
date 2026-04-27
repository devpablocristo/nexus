package approvals

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	sharedpostgres "github.com/devpablocristo/core/databases/postgres/go"
	approvaldomain "github.com/devpablocristo/nexus/v3/nexus/internal/approvals/usecases/domain"
	requestdomain "github.com/devpablocristo/nexus/v3/nexus/internal/requests/usecases/domain"
)

// DecisionApplier persiste de forma atómica la decisión sobre un approval y
// el cambio de status de la request asociada. Ambos UPDATEs viven en una
// sola transacción: o suben los dos, o no sube ninguno.
//
// Antes esto se hacía con dos repo.Update() separados; si el segundo fallaba
// quedaba el approval en "approved" con la request todavía "pending"
// (estado imposible de drift). Esta es la mitigación de C10 del audit.
type DecisionApplier struct {
	db *sharedpostgres.DB
}

func NewDecisionApplier(db *sharedpostgres.DB) *DecisionApplier {
	return &DecisionApplier{db: db}
}

// ApplyDecision corre los UPDATEs de approval + request en una sola tx.
// Espera que el caller ya haya mutado ambos structs en memoria con el estado
// final deseado.
func (d *DecisionApplier) ApplyDecision(ctx context.Context, a approvaldomain.Approval, req requestdomain.Request) error {
	tx, err := d.db.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin decision tx: %w", err)
	}
	defer func() {
		// Rollback es no-op si Commit ya pasó.
		_ = tx.Rollback(ctx)
	}()

	decisionsJSON, err := json.Marshal(a.Decisions)
	if err != nil {
		return fmt.Errorf("marshal decisions: %w", err)
	}
	if a.Decisions == nil {
		decisionsJSON = []byte("[]")
	}

	tag, err := tx.Exec(ctx, `
		UPDATE approvals
		SET status = $2, decided_by = $3, decision_note = $4, decided_at = $5, decisions = $6
		WHERE id = $1
	`, a.ID, a.Status, a.DecidedBy, a.DecisionNote, a.DecidedAt, decisionsJSON)
	if err != nil {
		return fmt.Errorf("update approval in tx: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	tag, err = tx.Exec(ctx, `
		UPDATE requests SET
			status = $2, risk_level = $3, decision = $4, decision_reason = $5,
			policy_id = $6, approval_id = $7, execution_result = $8, error_message = $9,
			ai_summary = $10, ai_degraded = $11,
			evaluated_at = $12, decided_at = $13, executed_at = $14, expires_at = $15, updated_at = $16
		WHERE id = $1
	`,
		req.ID, req.Status, req.RiskLevel, req.Decision, req.DecisionReason,
		req.PolicyID, req.ApprovalID, req.ExecutionResult, req.ErrorMessage,
		req.AISummary, req.AIDegraded,
		req.EvaluatedAt, req.DecidedAt, req.ExecutedAt, req.ExpiresAt, req.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update request in tx: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update request in tx: request not found")
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit decision tx: %w", err)
	}
	return nil
}
