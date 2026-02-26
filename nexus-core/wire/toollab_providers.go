package wire

import (
	"os"
	"strings"

	"github.com/google/wire"

	"nexus-core/internal/toollab"
)

var ToollabSet = wire.NewSet(
	toollab.NewRepository,
	wire.Bind(new(toollab.RepositoryPort), new(*toollab.Repository)),
	ProvideToollabConfig,
	toollab.NewService,
	toollab.NewHandler,
)

func ProvideToollabConfig() toollab.Config {
	return toollab.Config{
		AppVersion:           envOrDefault("NEXUS_APP_VERSION", "1.0.0"),
		Environment:          envOrDefault("NEXUS_ENV", "dev"),
		GitSHA:               os.Getenv("NEXUS_GIT_SHA"),
		ReadOnly:             strings.EqualFold(os.Getenv("NEXUS_READ_ONLY"), "true"),
		OpenAPIPath:          envOrDefault("NEXUS_OPENAPI_PATH", "docs/openapi.yaml"),
		DefaultRateRPS:       20,
		DefaultRateBurst:     40,
		DefaultTimeoutMS:     5000,
		MaxTimeoutMS:         30000,
		MaxInflight:          100,
		MaxQueue:             1000,
		MaxRequestBodyBytes:  262144,
		MaxResponseBodyBytes: 1048576,
		MaxLogsLines:         500,
		MaxTracesSpans:       5000,
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
