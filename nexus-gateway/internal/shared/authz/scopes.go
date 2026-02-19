package authz

const (
	ScopeToolsRead  = "tools:read"
	ScopeToolsWrite = "tools:write"

	ScopePolicyRead  = "policy:read"
	ScopePolicyWrite = "policy:write"

	ScopeEgressRead  = "egress:read"
	ScopeEgressWrite = "egress:write"

	ScopeAuditRead = "audit:read"

	ScopeGatewayRun      = "gateway:run"
	ScopeGatewaySimulate = "gateway:simulate"

	ScopeMCPRead = "mcp:read"
	ScopeMCPCall = "mcp:call"

	ScopeA2ACall = "a2a:call"

	ScopeSecretsAdmin = "admin:secrets"
)
