package requests

import (
	"context"
	"time"
)

type SummarizeInput struct {
	RequesterType  string
	RequesterID    string
	ActionType     string
	TargetSystem   string
	TargetResource string
	Params         map[string]any
	Reason         string
	Context        string
	Decision       string
	DecisionReason string
	RiskLevel      string
}

type AIContextualizer interface {
	Summarize(ctx context.Context, input SummarizeInput) (summary string, degraded bool, err error)
}

type stubContextualizer struct{}

func NewStubContextualizer() AIContextualizer {
	return &stubContextualizer{}
}

func (s *stubContextualizer) Summarize(ctx context.Context, input SummarizeInput) (string, bool, error) {
	_ = ctx
	return "Resumen no disponible (modo fallback). Requester: " + input.RequesterID + ", Acción: " + input.ActionType + " sobre " + input.TargetResource + ". Motivo: " + input.Reason, true, nil
}

type claudeContextualizer struct {
	apiKey     string
	timeout    time.Duration
	model      string
}

func NewClaudeContextualizer(apiKey, model string, timeout time.Duration) AIContextualizer {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &claudeContextualizer{apiKey: apiKey, timeout: timeout, model: model}
}

func (c *claudeContextualizer) Summarize(ctx context.Context, input SummarizeInput) (string, bool, error) {
	if c.apiKey == "" {
		return "Resumen no disponible (ANTHROPIC_API_KEY no configurada).", true, nil
	}
	// TODO: call Anthropic API with a short system prompt and the input as user message.
	// For PoC we return stub; integrate anthropic-sdk-go when adding real AI.
	return "Resumen no disponible (Claude no integrado en PoC). Decidí usando los datos de la request.", true, nil
}
