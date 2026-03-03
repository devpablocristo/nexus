package wire

import (
	"github.com/google/wire"

	"nexus-core/internal/gateway"
	"nexus-core/internal/mcp"
	"nexus-core/internal/tool"
)

func ProvideMCPToolPort(s *tool.Usecases) mcp.ToolPort { return s }
func ProvideMCPRunPort(s *gateway.Usecases) mcp.RunPort { return s }

var MCPSet = wire.NewSet(
	ProvideMCPToolPort,
	ProvideMCPRunPort,
	mcp.NewUsecases,
	mcp.NewHandler,
)
