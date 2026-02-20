export type EventItem = {
  id: number;
  event_type: string;
  created_at: string;
  payload: Record<string, unknown>;
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
