package alerts

// MetricsSink records minimum business metrics for alerts.
type MetricsSink interface {
	IncAlertCreated(channel, severity string)
}

// WithMetrics enables business metrics emission for alert lifecycle events.
func (u *Usecases) WithMetrics(metrics MetricsSink) *Usecases {
	u.metrics = metrics
	return u
}
