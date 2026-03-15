package wire

import (
	"net/http"

	"nexus/v2/control-plane/internal/policies"
	"nexus/v2/control-plane/internal/resources"
)

func NewServer() http.Handler {
	resourceRepo := resources.NewInMemoryRepository(nil)
	resourceUC := resources.NewUsecases(resourceRepo)
	policyRepo := policies.NewInMemoryRepository(nil)
	policyUC := policies.NewUsecases(policyRepo, policies.NewEvaluator())
	mux := http.NewServeMux()
	resources.NewHandler(resourceUC).Register(mux)
	policies.NewHandler(policyUC).Register(mux)
	return mux
}
