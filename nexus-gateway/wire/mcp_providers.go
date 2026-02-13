package wire

import (
	"github.com/google/wire"

	"nexus-gateway/internal/gateway"
	"nexus-gateway/internal/mcp"
	"nexus-gateway/internal/tool"
)

func ProvideMCPToolPort(s tool.Service) mcp.ToolPort  { return s }
func ProvideMCPRunPort(s gateway.Service) mcp.RunPort { return s }

var MCPSet = wire.NewSet(
	ProvideMCPToolPort,
	ProvideMCPRunPort,
	mcp.NewService,
	mcp.NewHandler,
)
