package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/secrets"
	"nexus-core/internal/tool"
)

func ProvideSecretsToolLookup(s *tool.Usecases) secrets.ToolLookupPort { return s }

var SecretsSet = wire.NewSet(
	secrets.NewRepository,
	wire.Bind(new(secrets.RepositoryPort), new(*secrets.Repository)),
	ProvideSecretsToolLookup,
	secrets.NewUsecases,
	secrets.NewHandler,
)
