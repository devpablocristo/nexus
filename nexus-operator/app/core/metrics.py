from prometheus_client import Counter, Gauge


EVENTS_CONSUMED = Counter('nexus_operator_events_consumed_total', 'Total events consumed by operator')
ACTIONS_APPLIED = Counter('nexus_operator_actions_applied_total', 'Total actions applied by operator')
INCIDENTS_OPENED = Counter('nexus_operator_incidents_opened_total', 'Total incidents opened by operator')
PROPOSALS_CREATED = Counter('nexus_operator_proposals_created_total', 'Total policy proposals created by operator')
LAST_CURSOR = Gauge('nexus_operator_last_cursor', 'Last processed cursor for event feed')
