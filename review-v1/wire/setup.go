package wire

import (
	"net/http"
	"time"

	sharedhandlers "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/handlers"
	sharedapikey "github.com/devpablocristo/nexus/v2/pkgs/go-pkg/apikey"
	"github.com/devpablocristo/nexus/review-v1/internal/requests"
	"github.com/devpablocristo/nexus/review-v1/internal/policies"
	"github.com/devpablocristo/nexus/review-v1/internal/approvals"
	"github.com/devpablocristo/nexus/review-v1/internal/audit"
	"github.com/devpablocristo/nexus/review-v1/internal/learning"
	"github.com/devpablocristo/nexus/review-v1/internal/dashboard"
)

type Config struct {
	APIKeys      string
	ApprovalTTL  time.Duration
	AnthropicKey string
}

func NewServer(cfg Config) (http.Handler, error) {
	policyRepo := policies.NewInMemoryRepository()
	approvalRepo := approvals.NewInMemoryRepository()
	auditRepo := audit.NewInMemoryRepository()
	reqRepo := requests.NewInMemoryRepository()
	idemStore := requests.NewInMemoryIdempotencyStore()
	auditSink := requests.NewAuditSinkAdapter(auditRepo)
	evaluator := requests.NewPolicyEvaluator()
	riskConfig := requests.DefaultRiskConfig()
	ai := requests.NewStubContextualizer()
	if cfg.AnthropicKey != "" {
		ai = requests.NewClaudeContextualizer(cfg.AnthropicKey, "claude-3-5-haiku-20241022", 5*time.Second)
	}
	ttl := cfg.ApprovalTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	reqUC := requests.NewUsecases(reqRepo, policyRepo, approvalRepo, idemStore, auditSink, evaluator, riskConfig, ai, ttl)
	reqHandler := requests.NewHandler(reqUC)

	policyUC := policies.NewUsecases(policyRepo)
	policyHandler := policies.NewHandler(policyUC)

	replayGetter := newReplayRequestGetter(reqRepo)
	auditUC := audit.NewUsecases(auditRepo, replayGetter)
	auditHandler := audit.NewHandler(auditUC)

	approvalUC := approvals.NewUsecases(approvalRepo, reqRepo)
	approvalHandler := approvals.NewHandler(approvalUC)

	learningRepo := learning.NewInMemoryRepository()
	learningPolicyCreator := newLearningPolicyCreator(policyRepo)
	learningUC := learning.NewUsecases(learningRepo, learningPolicyCreator)
	learningHandler := learning.NewHandler(learningUC)

	dashboardHandler := dashboard.NewHandler()

	mux := http.NewServeMux()
	sharedhandlers.RegisterHealthEndpoints(mux, nil)
	reqHandler.Register(mux)
	policyHandler.Register(mux)
	auditHandler.Register(mux)
	approvalHandler.Register(mux)
	learningHandler.Register(mux)
	dashboardHandler.Register(mux)

	authn, err := sharedapikey.NewAuthenticator(cfg.APIKeys)
	if err != nil {
		return nil, err
	}
	return authn.Middleware(mux), nil
}
