package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/policy"
	"nexus-core/internal/tool"
)

func ProvidePolicyToolLookup(s *tool.Usecases) policy.ToolLookupPort {
	return s
}

var PolicySet = wire.NewSet(
	policy.NewRepository,
	wire.Bind(new(policy.PolicyRepositoryPort), new(*policy.Repository)),
	ProvidePolicyToolLookup,
	policy.NewEvaluator,
	policy.NewUsecases,
	policy.NewHandler,
)
