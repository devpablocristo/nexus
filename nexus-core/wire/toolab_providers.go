package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/toolab"
)

var ToolabSet = wire.NewSet(
	toolab.NewRepository,
	wire.Bind(new(toolab.RepositoryPort), new(*toolab.Repository)),
	ProvideToolabConfig,
	toolab.NewService,
	toolab.NewHandler,
)

func ProvideToolabConfig() toolab.Config {
	return toolab.Config{AppVersion: "1.0.0"}
}
