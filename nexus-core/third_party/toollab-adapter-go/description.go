package toollab

import "context"

// ServiceDescription provides rich semantic metadata about the service.
// This data powers toollab's comprehension reports — the more complete
// the description, the better toollab can help humans understand the service.
type ServiceDescription struct {
	// Purpose is a human-readable description of what the service does.
	// Example: "API de gestión de políticas de seguridad para agentes de IA"
	Purpose string `json:"purpose"`

	// Domain identifies the business domain.
	// Example: "security", "e-commerce", "authentication", "ai-agents"
	Domain string `json:"domain"`

	// Consumers describes who uses this API.
	// Example: "Frontend web dashboard, CLI tools, other microservices"
	Consumers string `json:"consumers,omitempty"`

	// Models describes the data entities managed by the service.
	Models []ModelDescription `json:"models,omitempty"`

	// EndpointDescriptions provides human-readable info for each endpoint.
	EndpointDescriptions []EndpointDescription `json:"endpoints,omitempty"`

	// Dependencies lists external services this service depends on.
	Dependencies []Dependency `json:"dependencies,omitempty"`
}

// ModelDescription describes a data entity/resource.
type ModelDescription struct {
	// Name of the model (e.g., "Tool", "User", "Policy").
	Name string `json:"name"`

	// Description explains what this model represents.
	Description string `json:"description"`

	// Fields lists the model's fields with types and descriptions.
	Fields []FieldDescription `json:"fields,omitempty"`

	// Relations describes how this model relates to others.
	Relations []Relation `json:"relations,omitempty"`
}

// FieldDescription describes a single field in a model.
type FieldDescription struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
	Example     string `json:"example,omitempty"`
}

// Relation describes a relationship between models.
type Relation struct {
	Target      string `json:"target"`
	Type        string `json:"type"` // "has_many", "belongs_to", "has_one", "many_to_many"
	Description string `json:"description,omitempty"`
}

// EndpointDescription provides human-readable metadata for an endpoint.
type EndpointDescription struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"` // "business", "admin", "auth", "infra", "docs"

	// RequestExample is a sample request body (JSON string).
	RequestExample string `json:"request_example,omitempty"`

	// ResponseExample is a sample successful response body (JSON string).
	ResponseExample string `json:"response_example,omitempty"`

	// ErrorCodes lists possible error status codes with descriptions.
	ErrorCodes []ErrorCode `json:"error_codes,omitempty"`

	// RequiresAuth indicates whether this endpoint requires authentication.
	RequiresAuth bool `json:"requires_auth"`
}

// ErrorCode describes a possible error response.
type ErrorCode struct {
	Status      int    `json:"status"`
	Description string `json:"description"`
}

// Dependency describes an external service dependency.
type Dependency struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "database", "cache", "queue", "api", "storage"
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}

// ServiceDescriptionProvider exposes the service description capability.
type ServiceDescriptionProvider interface {
	ServiceDescription(ctx context.Context) (*ServiceDescription, error)
}
