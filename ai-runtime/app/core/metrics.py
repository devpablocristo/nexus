from prometheus_client import Counter, Gauge, Histogram


EVENTS_CONSUMED = Counter('nexus_operator_events_consumed_total', 'Total events consumed by operator')
ACTIONS_APPLIED = Counter('nexus_operator_actions_applied_total', 'Total actions applied by operator')
INCIDENTS_OPENED = Counter('nexus_operator_incidents_opened_total', 'Total incidents opened by operator')
PROPOSALS_CREATED = Counter('nexus_operator_proposals_created_total', 'Total policy proposals created by operator')
LAST_CURSOR = Gauge('nexus_operator_last_cursor', 'Last processed cursor for event feed')
DEAD_LETTER_EVENTS = Counter('nexus_dead_letter_events_total', 'Total permanently failed events written to dead-letter logs')

PROMPT_REQUESTS = Counter(
    'nexus_ai_prompt_requests_total',
    'Total prompt runtime requests by backend and prompt version.',
    ['backend', 'prompt_id', 'prompt_version'],
)
PROMPT_FALLBACKS = Counter(
    'nexus_ai_prompt_fallback_total',
    'Total deterministic prompt fallbacks by backend, prompt, and reason.',
    ['backend', 'prompt_id', 'prompt_version', 'reason'],
)
PROMPT_LATENCY = Histogram(
    'nexus_ai_prompt_latency_seconds',
    'Prompt runtime latency in seconds.',
    ['backend', 'prompt_id', 'prompt_version'],
    buckets=(0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30),
)
PROMPT_TOKENS = Counter(
    'nexus_ai_prompt_tokens_total',
    'Prompt runtime token usage when reported by the backend.',
    ['backend', 'prompt_id', 'prompt_version', 'direction'],
)
PROMPT_GUARDRAIL_VIOLATIONS = Counter(
    'nexus_ai_prompt_guardrail_violations_total',
    'Prompt runtime guardrail violations by rule.',
    ['backend', 'prompt_id', 'prompt_version', 'rule'],
)
