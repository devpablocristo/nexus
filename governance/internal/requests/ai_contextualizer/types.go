package aicontextualizer

// SummarizeInput contiene los datos necesarios para generar un resumen AI.
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

// AnthropicResponse representa la respuesta de la API de Anthropic.
type AnthropicResponse struct {
	Content []ContentBlock `json:"content"`
}

// ContentBlock representa un bloque de contenido en la respuesta.
type ContentBlock struct {
	Text string `json:"text"`
}
