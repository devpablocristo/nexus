package wire

import (
	"nexus-gateway/cmd/config"
	"nexus-gateway/internal/gateway"
)

func ProvideAPIConfig(cfg config.Config) config.APIConfig               { return cfg.API }
func ProvideDBConfig(cfg config.Config) config.DBConfig                 { return cfg.DB }
func ProvideHTTPServerConfig(cfg config.Config) config.HTTPServerConfig { return cfg.HTTPServer }
func ProvideServiceConfig(cfg config.Config) config.ServiceConfig       { return cfg.Service }

func ProvideGatewayConfig(cfg config.ServiceConfig) gateway.Config {
	return gateway.Config{
		DefaultRateLimitPerMinute: cfg.RateLimitDefaultPerMin,
		MaxBytesInputDefault:      262144,
		MaxBytesContextDefault:    65536,
	}
}
