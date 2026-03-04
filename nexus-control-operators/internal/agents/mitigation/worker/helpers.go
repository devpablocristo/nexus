package worker

import "nexus-control-operators/internal/shared/eventutil"

var (
	ResolveIncidentID = eventutil.ResolveIncidentUUID
	AsString          = eventutil.AsString
	AsInt             = eventutil.AsInt
	ToAnySlice        = eventutil.ToAnySlice
	ToMap             = eventutil.ToMap
	ToStringSlice     = eventutil.ToStringSlice
	Ptr               = eventutil.Ptr
)
