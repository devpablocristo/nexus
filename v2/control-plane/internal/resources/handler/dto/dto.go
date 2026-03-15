package dto

import "time"

type CreateResourceRequest struct {
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Environment string            `json:"environment"`
	Chain       string            `json:"chain"`
	Labels      map[string]string `json:"labels"`
	Criticality string            `json:"criticality"`
}

type UpdateResourceRequest struct {
	Type        *string           `json:"type,omitempty"`
	Name        *string           `json:"name,omitempty"`
	Environment *string           `json:"environment,omitempty"`
	Chain       *string           `json:"chain,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Criticality *string           `json:"criticality,omitempty"`
}

type ResourceResponse struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Environment string            `json:"environment"`
	Chain       string            `json:"chain"`
	Labels      map[string]string `json:"labels,omitempty"`
	Criticality string            `json:"criticality"`
	ArchivedAt  *time.Time        `json:"archived_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type ListResourcesResponse struct {
	Items []ResourceResponse `json:"items"`
}

type ErrorResponse struct {
	Error ErrorObject `json:"error"`
}

type ErrorObject struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
