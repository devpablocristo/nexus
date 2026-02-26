package llm

import (
	"time"
)

type Config struct {
	Provider      string
	Model         string
	OllamaBaseURL string
	CloudBaseURL  string
	CloudAPIKey   string
	SchemaDir     string
	Timeout       time.Duration
}
