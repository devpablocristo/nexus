export type EventItem = {
  id: number;
  event_type: string;
  created_at: string;
  payload: Record<string, unknown>;
};

export type AuditEventItem = {
  request_id: string;
  tool_name: string;
  decision: 'allow' | 'deny';
  status: 'success' | 'error' | 'blocked';
  policy_id?: string;
  reason?: string;
  latency_ms: number;
  created_at: string;
  input?: Record<string, unknown>;
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
