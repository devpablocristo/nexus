export type NexusEvent = {
  id: number;
  event_type: string;
  created_at: string;
  payload: Record<string, unknown>;
};

export type NexusAction = {
  id: string;
  scope_type: "tenant" | "tool" | "agent" | "global";
  scope_id?: string;
  action_type: string;
  params: Record<string, unknown>;
  ttl_seconds: number;
  status: "active" | "expired" | "rolled_back";
  evidence_refs: string[];
  created_by?: string;
  created_at: string;
  rolled_back_at?: string;
  rolled_back_by?: string;
};

export type NexusIncident = {
  id: string;
  severity: "LOW" | "MED" | "HIGH" | "CRIT";
  status: "open" | "closed";
  title: string;
  summary: string;
  related_action_ids: string[];
  evidence_refs: string[];
  opened_at: string;
  closed_at?: string;
};

export type NexusPolicyProposal = {
  id: string;
  status: "draft" | "pending" | "approved" | "rejected" | "shadow";
  diff: Record<string, unknown>;
  rationale: string;
  tests_suggested: string[];
  rollback_plan: string;
  created_by?: string;
  created_at: string;
  decided_by?: string;
  decided_at?: string;
};
