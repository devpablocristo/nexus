package wire

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	sharedpostgres "github.com/devpablocristo/core/databases/postgres/go"
	"github.com/devpablocristo/core/http/go/health"
	"github.com/devpablocristo/nexus/v3/nexus/internal/actiontypes"
	"github.com/devpablocristo/nexus/v3/nexus/internal/approvals"
	"github.com/devpablocristo/nexus/v3/nexus/internal/audit"
	"github.com/devpablocristo/nexus/v3/nexus/internal/callbacks"
	nexusconfig "github.com/devpablocristo/nexus/v3/nexus/internal/config"
	"github.com/devpablocristo/nexus/v3/nexus/internal/dashboard"
	"github.com/devpablocristo/nexus/v3/nexus/internal/delegations"
	"github.com/devpablocristo/nexus/v3/nexus/internal/evidence"
	"github.com/devpablocristo/nexus/v3/nexus/internal/learning"
	"github.com/devpablocristo/nexus/v3/nexus/internal/policies"
	"github.com/devpablocristo/nexus/v3/nexus/internal/requests"
)

type Config struct {
	DatabaseURL          string
	APIKeys              string
	AuthIssuerURL        string
	AuthAudience         string
	ApprovalTTL          time.Duration
	AnthropicKey         string
	SigningKey           string
	CallbackToken        string
	PendingCallbackURLs  []string
	ResolvedCallbackURLs []string
	MigrationFiles       fs.FS
}

func NewServer(cfg Config) (http.Handler, func(), error) {
	ctx := context.Background()

	// Base de datos
	db, err := sharedpostgres.OpenWithConfig(ctx, cfg.DatabaseURL, sharedpostgres.DefaultConfig("nexus-review"))
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	// Migraciones
	if err := sharedpostgres.MigrateUp(ctx, db, "nexus-review", cfg.MigrationFiles, "."); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	// Repositorios (todos postgres)
	policyRepo := policies.NewPostgresRepository(db)
	approvalRepo := approvals.NewPostgresRepository(db)
	auditRepo := audit.NewPostgresRepository(db)
	reqRepo := requests.NewPostgresRepository(db)
	idemStore := requests.NewPostgresIdempotencyStore(db)
	resultReportStore := requests.NewPostgresResultReportStore(db)
	learningRepo := learning.NewPostgresRepository(db)
	configRepo := nexusconfig.NewPostgresRepository(db.Pool())
	actionTypeRepo := actiontypes.NewPostgresRepository(db)

	// Adapters
	auditSink := requests.NewAuditSinkAdapter(auditRepo)
	evaluator := requests.NewPolicyEvaluator()
	riskConfig := requests.DefaultRiskConfig()
	callbackPublisher := callbacks.NewHTTPApprovalPublisher(cfg.CallbackToken, cfg.PendingCallbackURLs, cfg.ResolvedCallbackURLs)

	// AI contextualizer
	var ai requests.AIContextualizer = requests.NewStubContextualizer()
	if cfg.AnthropicKey != "" {
		ai = requests.NewClaudeContextualizer(cfg.AnthropicKey, "claude-sonnet-4-20250514")
	}

	ttl := cfg.ApprovalTTL
	if ttl <= 0 {
		ttl = time.Hour
	}

	// Usecases
	configUC := nexusconfig.NewUsecases(configRepo)
	policyUC := policies.NewUsecases(policyRepo)
	policyLister := newPolicyListerAdapter(policyUC)
	execStats := requests.NewPostgresExecutionStatsStore(db.Pool())

	// Break-glass: default rules (configurable via /v1/config)
	breakGlassCfg := requests.BreakGlassConfig{
		DefaultApprovals: 2,
		Rules: []requests.BreakGlassRule{
			{ActionTypes: []string{"delete"}, RiskLevel: "critical", RequiredApprovals: 2},
			{ActionTypes: []string{"runbook.execute"}, RiskLevel: "high", RequiredApprovals: 2},
		},
	}

	actionTypeUC := actiontypes.NewUsecases(actionTypeRepo)
	delegationRepo := delegations.NewPostgresRepository(db)
	delegationUC := delegations.NewUsecases(delegationRepo)

	attestationStore := requests.NewPostgresAttestationStore(db.Pool())

	reqUC := requests.NewUsecases(reqRepo, policyLister, approvalRepo, evaluator,
		requests.WithIdempotencyStore(idemStore),
		requests.WithAuditSink(auditSink),
		requests.WithRiskConfig(riskConfig),
		requests.WithAI(ai),
		requests.WithApprovalTTL(ttl),
		requests.WithShadowHitRecorder(policyRepo),
		requests.WithExecutionStats(execStats),
		requests.WithBreakGlassConfig(breakGlassCfg),
		requests.WithActionTypeChecker(newActionTypeCheckerAdapter(actionTypeUC)),
		requests.WithDelegationChecker(newDelegationCheckerAdapter(delegationUC)),
		requests.WithAttestationStore(attestationStore),
		requests.WithApprovalGetter(approvalRepo),
		requests.WithApprovalCallbacks(callbackPublisher),
		requests.WithResultReportStore(resultReportStore),
	)
	approvalUC := approvals.NewUsecases(approvalRepo, reqRepo).
		WithAuditSink(auditSink).
		WithApprovalCallbacks(callbackPublisher).
		WithDecisionTx(approvals.NewDecisionApplier(db))
	replayGetter := newReplayRequestGetter(reqRepo)
	auditUC := audit.NewUsecases(auditRepo, replayGetter)

	// Learning con analyzer y proposer
	learningPolicyCreator := newLearningPolicyCreator(policyRepo)
	analyzer := learning.NewInMemoryPatternAnalyzer(reqRepo)
	var proposer learning.PolicyProposer = learning.NewStubProposer()
	if cfg.AnthropicKey != "" {
		proposer = learning.NewAIProposer(cfg.AnthropicKey, "claude-sonnet-4-20250514")
	}
	learningUC := learning.NewUsecases(learningRepo, learningPolicyCreator).
		WithAnalyzer(analyzer).
		WithProposer(proposer)

	// Handlers
	reqHandler := requests.NewHandler(reqUC)
	policyHandler := policies.NewHandler(policyUC)
	auditHandler := audit.NewHandler(auditUC)
	approvalHandler := approvals.NewHandler(approvalUC)
	learningHandler := learning.NewHandler(learningUC)
	dashboardHandler := dashboard.NewHandler(reqRepo)
	configHandler := nexusconfig.NewHandler(configUC)
	actionTypeHandler := actiontypes.NewHandler(actionTypeUC)
	delegationHandler := delegations.NewHandler(delegationUC)

	// Evidence packs.
	// Sin default fallback: si la clave no está, falla startup. Un default
	// hardcodeado terminaría firmando evidence packs en prod con una clave
	// pública.
	if cfg.SigningKey == "" {
		db.Close()
		return nil, nil, fmt.Errorf("NEXUS_SIGNING_KEY is required")
	}
	signer, err := evidence.NewSigner(cfg.SigningKey, "default")
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create evidence signer: %w", err)
	}
	evidenceUC := evidence.NewUsecases(reqRepo, approvalRepo, auditRepo, signer).
		WithAttestationReader(attestationStore)
	evidenceHandler := evidence.NewHandler(evidenceUC)

	// Router
	mux := http.NewServeMux()
	health.RegisterEndpoints(mux, func(ctx context.Context) error {
		return db.Ping(ctx)
	})
	reqHandler.Register(mux)
	policyHandler.Register(mux)
	auditHandler.Register(mux)
	approvalHandler.Register(mux)
	learningHandler.Register(mux)
	dashboardHandler.Register(mux)
	configHandler.Register(mux)
	actionTypeHandler.Register(mux)
	delegationHandler.Register(mux)
	evidenceHandler.Register(mux)

	authMW, err := newAuthMiddleware(cfg.APIKeys, cfg.AuthIssuerURL, cfg.AuthAudience)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create authenticator: %w", err)
	}

	cleanup := func() {
		db.Close()
	}

	return authMW(mux), cleanup, nil
}
