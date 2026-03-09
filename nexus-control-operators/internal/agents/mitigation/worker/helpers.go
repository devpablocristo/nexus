// Package worker exposes test-friendly mitigation helper aliases.
package worker

import "nexus-control-operators/internal/shared/eventutil"

var (
	ResolveIncidentID = eventutil.ResolveIncidentUUID
	AsString          = eventutil.AsString
	AsInt             = eventutil.AsInt
	ToAnySlice        = eventutil.ToAnySlice
	ToMap             = eventutil.ToMap
	ToStringMap       = eventutil.ToStringMap
	ToStringSlice     = eventutil.ToStringSlice
	Ptr               = eventutil.Ptr
)
