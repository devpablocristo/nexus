package config

type ServiceConfig struct {
	LogLevel               string
	SwaggerCDN             bool
	HTTPTimeoutMS          int
	HTTPMaxResponseBytes   int64
	RateLimitDefaultPerMin int
}
