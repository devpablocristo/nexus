export type EventItem = {
  id: number;
  event_type: string;
  created_at: string;
  payload: Record<string, unknown>;
};

export type AuditEventItem = {
  request_id: string;
  org_id?: string;
  tool_name: string;
  actor?: string;
  role?: string;
  scopes?: string[];
  decision: 'allow' | 'deny';
  status: 'success' | 'error' | 'blocked';
  reason?: string;
  latency_ms: number;
  idempotency_present?: boolean;
  idempotency_outcome?: string;
  timeout_ms?: number;
  budget_remaining_ms_at_execute?: number;
  stage_durations_ms?: Record<string, number>;
  prev_event_hash?: string;
  event_hash?: string;
  hash_algo?: string;
  created_at: string;
  input?: Record<string, unknown>;
  context?: Record<string, unknown>;
  dlp_summary?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: {
    code?: string;
    message?: string;
  };
};

export type ActionItem = {
  id: string;
  scope_type: string;
  action_type: string;
  status: string;
  ttl_seconds: number;
  created_at: string;
};

export type IncidentItem = {
  id: string;
  severity: 'LOW' | 'MED' | 'HIGH' | 'CRIT';
  status: 'open' | 'closed';
  title: string;
  summary: string;
  opened_at: string;
  closed_at?: string;
};

export type PolicyProposalItem = {
  id: string;
  status: 'draft' | 'pending' | 'approved' | 'rejected' | 'shadow';
  rationale: string;
  diff: Record<string, unknown>;
  tests_suggested: string[];
  rollback_plan: string;
  created_at: string;
};

export type AssistantResponse = {
  summary: string;
  tables?: {
    title: string;
    columns: string[];
    rows: Record<string, string>[];
  }[];
  actions?: {
    label: string;
    action_type: string;
    payload: Record<string, unknown>;
  }[];
};

export type WorldRunItem = {
  run_id: string;
  org_id: string;
  seed: number;
  config_hash: string;
  created_at: string;
};

export type WorldRunsResponse = {
  items: WorldRunItem[];
  next_cursor: string;
};

export type WorldEventItem = {
  id: number;
  run_id: string;
  step_id: number;
  seq: number;
  org_id: string;
  agent_id: string;
  tool_name: string;
  request_id: string;
  created_at: string;
  payload: Record<string, unknown>;
};

export type WorldEventsResponse = {
  items: WorldEventItem[];
  next_seq: number;
};

export type WorldAgentState = {
  id: string;
  x: number;
  y: number;
  vx: number;
  vy: number;
  heading: number;
  intention_x?: number;
  intention_y?: number;
};

export type WorldState = {
  config: {
    width: number;
    height: number;
    door_x: number;
    door_min_y: number;
    door_max_y: number;
    agent_radius: number;
    agent_count: number;
  };
  agents: WorldAgentState[];
};

export type WorldStateResponse = {
  run_id: string;
  step_id: number;
  state_hash: string;
  state: WorldState;
};

export type WorldRunCreateResponse = {
  run_id: string;
  seed: number;
  config_hash: string;
  state_hash: string;
};

export type WorldReplayResponse = {
  run_id: string;
  replayed_moves: number;
  state_hash: string;
};

export type ApprovalItem = {
  id: string;
  request_id: string;
  tool_name: string;
  actor?: string;
  role?: string;
  input_redacted: Record<string, unknown>;
  context_redacted: Record<string, unknown>;
  reason: string;
  status: string;
  decided_by?: string;
  decided_at?: string;
  expires_at: string;
  created_at: string;
};

export type AlertRuleItem = {
  id: string;
  name: string;
  metric: string;
  threshold: number;
  window_seconds: number;
  tool_name?: string;
  webhook_url: string;
  cooldown_seconds: number;
  enabled: boolean;
  last_fired_at?: string;
  created_at: string;
};

export type SessionItem = {
  id: string;
  session_id: string;
  actor?: string;
  total_calls: number;
  total_writes: number;
  total_denials: number;
  metadata: Record<string, unknown>;
  created_at: string;
  last_call_at: string;
};
