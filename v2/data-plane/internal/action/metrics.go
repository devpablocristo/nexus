package action

// MetricsSink records minimum business metrics for the action lifecycle.
type MetricsSink interface {
	IncActionCreated(actionType string)
	IncActionBlocked(actionType string)
	IncActionApproved(actionType string)
	IncActionExecuted(actionType string)
}

// WithMetrics enables business metrics emission for action lifecycle events.
func (u *Usecases) WithMetrics(metrics MetricsSink) *Usecases {
	u.metrics = metrics
	return u
}
