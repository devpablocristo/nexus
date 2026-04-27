package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	coreai "github.com/devpablocristo/core/ai/go"
	"github.com/google/uuid"
	learningdomain "github.com/devpablocristo/nexus/governance/internal/learning/usecases/domain"
)

// PolicyProposer genera propuestas de políticas a partir de patrones detectados.
type PolicyProposer interface {
	GenerateProposal(ctx context.Context, pattern Pattern) (*learningdomain.PolicyProposal, error)
}

// --- Stub (desarrollo local sin API key) ---

// StubProposer genera propuestas sin usar IA (basado en templates).
type StubProposer struct{}

func NewStubProposer() *StubProposer {
	return &StubProposer{}
}

func (s *StubProposer) GenerateProposal(_ context.Context, pattern Pattern) (*learningdomain.PolicyProposal, error) {
	return buildProposal(pattern, stubGenerate(pattern)), nil
}

// --- Implementación con core/ai/go ---

// AIProposer genera propuestas usando un LLM via core/ai/go.
type AIProposer struct {
	provider coreai.Provider
}

// NewAIProposer crea un proposer con IA. Usa el provider de core/ai/go.
func NewAIProposer(apiKey, model string) *AIProposer {
	return &AIProposer{
		provider: coreai.NewProvider("anthropic", apiKey, model),
	}
}

func (a *AIProposer) GenerateProposal(ctx context.Context, pattern Pattern) (*learningdomain.PolicyProposal, error) {
	generated := a.askLLM(ctx, pattern)
	return buildProposal(pattern, generated), nil
}

// generatedFields contiene los campos que el LLM genera.
type generatedFields struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Expression  string `json:"expression"`
	Effect      string `json:"effect"`
	Summary     string `json:"summary"`
	Priority    int    `json:"priority"`
}

func (a *AIProposer) askLLM(ctx context.Context, pattern Pattern) generatedFields {
	userMsg := fmt.Sprintf(
		"Patrón detectado:\n- action_type: %s\n- aprobadas: %d de %d (%.1f%%)\n- ventana: %s\n\nGenerá una propuesta de política CEL.",
		pattern.ActionType, pattern.Approved, pattern.Total, pattern.ApprovalRate*100, pattern.TimeWindow,
	)

	resp, err := a.provider.Chat(ctx, coreai.ChatRequest{
		SystemPrompt: proposerSystemPrompt,
		Messages:     []coreai.Message{{Role: "user", Content: userMsg}},
		MaxTokens:    500,
	})
	if err != nil {
		return generatedFields{}
	}
	if resp.Text == "" {
		return generatedFields{}
	}

	var fields generatedFields
	if err := json.Unmarshal([]byte(resp.Text), &fields); err != nil {
		slog.Warn("ai proposer response not valid json, extracting text", "response", resp.Text)
		// Si no es JSON válido, usar la respuesta como summary y generar el resto con template
		fallback := stubGenerate(pattern)
		fallback.Summary = resp.Text
		return fallback
	}

	// Validar campos obligatorios
	if fields.Expression == "" {
		fields.Expression = fmt.Sprintf("request.action_type == '%s'", pattern.ActionType)
	}
	if fields.Effect == "" {
		fields.Effect = "allow"
	}
	if fields.Name == "" {
		fields.Name = fmt.Sprintf("auto-approve-%s", pattern.ActionType)
	}
	if fields.Priority <= 0 {
		fields.Priority = 100
	}

	return fields
}

const proposerSystemPrompt = `Sos un experto en gobernanza de Nexus Review. Analizás patrones de aprobación y generás propuestas de políticas CEL.

Respondé SOLO con un JSON válido con esta estructura:
{
  "name": "nombre-kebab-case de la política",
  "description": "descripción concisa en inglés de qué hace la política",
  "expression": "expresión CEL válida (ej: request.action_type == 'deploy')",
  "effect": "allow | deny | require_approval",
  "summary": "resumen en español del análisis del patrón y por qué se recomienda esta política",
  "priority": 100
}

Reglas:
- La expresión CEL debe usar variables del namespace request (action_type, target_system, requester_type, etc.) o time (hour, day_of_week).
- Si la tasa de aprobación es ≥ 95%, recomendar effect "allow".
- Si es entre 80-95%, recomendar "allow" con una expresión más restrictiva (ej: agregar horario o target_system).
- Si es < 80%, recomendar "require_approval".
- priority: 100 por defecto, menor para políticas más específicas.
- El summary debe explicar el razonamiento.`

// --- Helpers compartidos ---

func stubGenerate(pattern Pattern) generatedFields {
	return generatedFields{
		Name: fmt.Sprintf("auto-approve-%s", pattern.ActionType),
		Description: fmt.Sprintf(
			"Auto-approve %s — historically approved %.0f%% of the time (%d/%d)",
			pattern.ActionType, pattern.ApprovalRate*100, pattern.Approved, pattern.Total,
		),
		Expression: fmt.Sprintf("request.action_type == '%s'", pattern.ActionType),
		Effect:     "allow",
		Summary: fmt.Sprintf(
			"En los últimos %s, %.0f%% de las requests '%s' fueron aprobadas (%d de %d).",
			pattern.TimeWindow, pattern.ApprovalRate*100, pattern.ActionType, pattern.Approved, pattern.Total,
		),
		Priority: 100,
	}
}

func buildProposal(pattern Pattern, gen generatedFields) *learningdomain.PolicyProposal {
	return &learningdomain.PolicyProposal{
		ID:                  uuid.New(),
		ProposedName:        gen.Name,
		ProposedDescription: gen.Description,
		ProposedExpression:  gen.Expression,
		ProposedEffect:      gen.Effect,
		ProposedActionType:  &pattern.ActionType,
		ProposedPriority:    gen.Priority,
		PatternSummary:      gen.Summary,
		Confidence:          pattern.ApprovalRate,
		SampleSize:          pattern.Total,
		TimeWindow:          pattern.TimeWindow,
		Status:              learningdomain.ProposalStatusPending,
		CreatedAt:           time.Now().UTC(),
	}
}
