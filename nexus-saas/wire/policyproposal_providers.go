package wire

import (
	"github.com/google/wire"

	"nexus-saas/internal/events"
	"nexus-saas/internal/policyproposal"
)

func ProvidePolicyProposalEventSink(s *events.Usecases) policyproposal.EventSink { return s }

var PolicyProposalSet = wire.NewSet(
	policyproposal.NewRepository,
	wire.Bind(new(policyproposal.RepositoryPort), new(*policyproposal.Repository)),
	ProvidePolicyProposalEventSink,
	policyproposal.NewUsecases,
	policyproposal.NewHandler,
)
