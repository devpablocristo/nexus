package incidents

// MetricsSink records minimum business metrics for incidents.
type MetricsSink interface {
	IncIncidentCreated(trigger, severity string)
}

// WithMetrics enables business metrics emission for incident lifecycle events.
func (u *Usecases) WithMetrics(metrics MetricsSink) *Usecases {
	u.metrics = metrics
	return u
}
