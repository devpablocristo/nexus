export type ToolItem = {
  id: string;
  name: string;
  kind: 'http';
  description?: string;
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  url: string;
  input_schema?: Record<string, unknown>;
  output_schema?: Record<string, unknown>;
  action_type: 'read' | 'write';
  classification: 'internal' | 'external';
  sensitivity: 'low' | 'medium' | 'high';
  risk_level: 1 | 2 | 3;
  enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type EgressRuleItem = {
  id: string;
  tool_id: string;
  host: string;
  enabled: boolean;
  created_at: string;
};

export type PolicyItem = {
  id: string;
  tool_id: string;
  name?: string;
  effect: 'allow' | 'deny';
  priority: number;
  conditions: Record<string, unknown>;
  limits: Record<string, unknown>;
  reason_template?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type AuditItem = {
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
  input?: unknown;
  context?: unknown;
  dlp_summary?: unknown;
  output?: unknown;
  error?: { code: string; message: string };
};

export type UserInfo = {
  id: string;
  external_id: string;
  email: string;
  name: string;
  avatar_url?: string;
  created_at: string;
  updated_at: string;
};

export type UserMe = {
  org_id: string;
  external_id: string;
  role?: string;
  scopes?: string[];
  user?: UserInfo;
};

export type OrgMemberItem = {
  id: string;
  org_id: string;
  user_id: string;
  role: 'admin' | 'secops';
  joined_at: string;
  user: UserInfo;
};

export type APIKeyItem = {
  id: string;
  org_id: string;
  name: string;
  scopes: string[];
  created_at: string;
};

export type SecretItem = {
  id: string;
  secret_type: string;
  key_name: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type IncidentItem = {
  id: string;
  severity: string;
  status: string;
  title: string;
  summary: string;
  related_action_ids: string[];
  evidence_refs: string[];
  created_by?: string;
  opened_at: string;
  closed_at?: string;
};

export type EventItem = {
  id: number;
  event_type: string;
  payload: Record<string, unknown>;
  created_at: string;
};

export type AssistantTable = {
  title: string;
  columns: string[];
  rows: Array<Record<string, string>>;
};

export type AssistantAction = {
  label: string;
  action_type: string;
  payload: Record<string, unknown>;
};

export type AssistantResponse = {
  summary: string;
  tables?: AssistantTable[];
  actions?: AssistantAction[];
};

export type BillingPlanCode = 'starter' | 'growth' | 'enterprise';

export type BillingLifecycleStatus = 'trialing' | 'active' | 'past_due' | 'canceled' | 'unpaid';

export type BillingHardLimits = {
  tools_max: number;
  run_rpm: number;
  audit_retention_days: number;
};

export type UsageSummary = {
  period: string;
  counters: {
    api_calls: number;
    events_ingested: number;
    incidents_opened: number;
    actions_executed: number;
  };
};

export type BillingStatus = {
  plan_code: BillingPlanCode;
  billing_status: BillingLifecycleStatus;
  current_period_end?: string;
  hard_limits: BillingHardLimits;
  usage: UsageSummary;
};

export type AdminTenantSettings = {
  plan_code: string;
  hard_limits: {
    tools_max: number;
    run_rpm: number;
    audit_retention_days: number;
  };
  updated_by?: string;
  updated_at?: string;
  created_at?: string;
};

export type AdminBootstrap = {
  org_id: string;
  actor?: string;
  role?: string;
  scopes: string[];
  auth_method: string;
  can_read_admin: boolean;
  can_write_admin: boolean;
  tenant_settings: AdminTenantSettings;
};

export type AdminActivityItem = {
  id: string;
  actor?: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  payload: Record<string, unknown>;
  created_at: string;
};

export type NotificationPreference = {
  notification_type: string;
  channel: string;
  enabled: boolean;
};
