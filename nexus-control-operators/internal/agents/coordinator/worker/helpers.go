package worker

import (
	"strings"

	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

// ResolveIncidentID extrae el incident_id del evento.
func ResolveIncidentID(event opsdomain.StoredEvent) string {
	if event.Envelope.Correlation.IncidentID != nil && *event.Envelope.Correlation.IncidentID != "" {
		return *event.Envelope.Correlation.IncidentID
	}
	if s := strings.TrimSpace(AsString(event.Envelope.Payload["incident_id"])); s != "" {
		return s
	}
	return ""
}

// AsString convierte un valor a string.
func AsString(v any) string {
	s, _ := v.(string)
	return s
}
