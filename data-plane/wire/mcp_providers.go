package wire

import (
	"github.com/google/wire"

	"data-plane/internal/gateway"
	"data-plane/internal/mcp"
	"data-plane/internal/tool"
)

func ProvideMCPToolPort(s *tool.Usecases) mcp.ToolPort { return s }
func ProvideMCPRunPort(s *gateway.Usecases) mcp.RunPort { return s }

var MCPSet = wire.NewSet(
	ProvideMCPToolPort,
	ProvideMCPRunPort,
	mcp.NewUsecases,
	mcp.AsMCPUsecase,
	mcp.NewHandler,
)
