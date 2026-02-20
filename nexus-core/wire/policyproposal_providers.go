package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/events"
	"nexus-core/internal/policyproposal"
)

func ProvidePolicyProposalEventSink(s events.Service) policyproposal.EventSink { return s }

var PolicyProposalSet = wire.NewSet(
	policyproposal.NewRepository,
	wire.Bind(new(policyproposal.RepositoryPort), new(*policyproposal.Repository)),
	ProvidePolicyProposalEventSink,
	policyproposal.NewService,
	policyproposal.NewHandler,
)
