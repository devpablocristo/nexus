package wire

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	sharedapikey "github.com/devpablocristo/nexus/v3/pkgs/go-pkg/apikey"
	sharedhandlers "github.com/devpablocristo/nexus/v3/pkgs/go-pkg/handlers"
	sharedpostgres "github.com/devpablocristo/nexus/v3/pkgs/go-pkg/postgres"
	"github.com/devpablocristo/nexus/v3/review/internal/approvals"
	"github.com/devpablocristo/nexus/v3/review/internal/audit"
	nexusconfig "github.com/devpablocristo/nexus/v3/review/internal/config"
	"github.com/devpablocristo/nexus/v3/review/internal/dashboard"
	"github.com/devpablocristo/nexus/v3/review/internal/learning"
	"github.com/devpablocristo/nexus/v3/review/internal/policies"
	"github.com/devpablocristo/nexus/v3/review/internal/requests"
)

type Config struct {
	DatabaseURL    string
	APIKeys        string
	ApprovalTTL    time.Duration
	AnthropicKey   string
	MigrationFiles fs.FS
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
	learningRepo := learning.NewPostgresRepository(db)
	configRepo := nexusconfig.NewPostgresRepository(db.Pool())

	// Adapters
	auditSink := requests.NewAuditSinkAdapter(auditRepo)
	evaluator := requests.NewPolicyEvaluator()
	riskConfig := requests.DefaultRiskConfig()

	// AI contextualizer
	var ai requests.AIContextualizer = requests.NewStubContextualizer()
	if cfg.AnthropicKey != "" {
		ai = requests.NewClaudeContextualizer(cfg.AnthropicKey, "claude-sonnet-4-20250514", 5*time.Second)
	}

	ttl := cfg.ApprovalTTL
	if ttl <= 0 {
		ttl = time.Hour
	}

	// Usecases
	configUC := nexusconfig.NewUsecases(configRepo)
	policyUC := policies.NewUsecases(policyRepo)
	policyLister := newPolicyListerAdapter(policyUC)
	reqUC := requests.NewUsecases(reqRepo, policyLister, approvalRepo, evaluator,
		requests.WithIdempotencyStore(idemStore),
		requests.WithAuditSink(auditSink),
		requests.WithRiskConfig(riskConfig),
		requests.WithAI(ai),
		requests.WithApprovalTTL(ttl),
	)
	approvalUC := approvals.NewUsecases(approvalRepo, reqRepo).WithAuditSink(auditSink)
	replayGetter := newReplayRequestGetter(reqRepo)
	auditUC := audit.NewUsecases(auditRepo, replayGetter)

	// Learning con analyzer y proposer
	learningPolicyCreator := newLearningPolicyCreator(policyRepo)
	analyzer := learning.NewInMemoryPatternAnalyzer(reqRepo)
	proposer := learning.NewStubProposer()
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

	// Router
	mux := http.NewServeMux()
	sharedhandlers.RegisterHealthEndpoints(mux, func(ctx context.Context) error {
		return db.Ping(ctx)
	})
	reqHandler.Register(mux)
	policyHandler.Register(mux)
	auditHandler.Register(mux)
	approvalHandler.Register(mux)
	learningHandler.Register(mux)
	dashboardHandler.Register(mux)
	configHandler.Register(mux)

	// Auth middleware
	authn, err := sharedapikey.NewAuthenticator(cfg.APIKeys)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create authenticator: %w", err)
	}

	cleanup := func() {
		db.Close()
	}

	return authn.Middleware(mux), cleanup, nil
}
