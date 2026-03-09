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
  status: 'active' | 'suspended' | 'deleted' | string;
  deleted_at?: string;
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

export type ProtectedResourceItem = {
  id: string;
  name: string;
  resource_type: string;
  match_value: string;
  match_mode: 'exact' | 'contains' | string;
  environment: 'prod' | 'nonprod' | '*' | string;
  reason?: string;
  enabled: boolean;
  created_by?: string;
  updated_by?: string;
  created_at: string;
  updated_at: string;
};

export type RestoreEvidenceItem = {
  id: string;
  environment: string;
  system: string;
  status: 'passed' | 'failed' | string;
  snapshot_id?: string;
  restore_target?: string;
  started_at?: string;
  completed_at?: string;
  source?: string;
  artifact_sha256?: string;
  summary: Record<string, unknown>;
  created_at: string;
};

export type NotificationPreference = {
  notification_type: string;
  channel: string;
  enabled: boolean;
};

export type InAppNotification = {
  id: string;
  org_id: string;
  actor_id: string;
  type: string;
  title: string;
  body: string;
  read_at?: string | null;
  created_at: string;
};

export type ExecutionIntentItem = {
  id: string;
  request_id: string;
  tool_name: string;
  actor?: string;
  role?: string;
  scopes: string[];
  input: Record<string, unknown>;
  context: Record<string, unknown>;
  policy_id?: string;
  risk_class: 'read' | 'plan' | 'mutate_nonprod' | 'mutate_prod' | 'destructive_prod' | 'break_glass';
  reason: string;
  approval_id?: string;
  status: 'pending_approval' | 'approved' | 'rejected' | 'executed' | 'expired';
  preflight_status: 'not_required' | 'passed' | 'failed';
  preflight_summary: Record<string, unknown>;
  preflight_artifact_sha256?: string;
  preflight_completed_at?: string;
  expires_at: string;
  approved_at?: string;
  executed_at?: string;
  created_at: string;
  updated_at: string;
};

export type PreflightReview = {
  intent_id: string;
  tool_name: string;
  risk_class: ExecutionIntentItem['risk_class'];
  reason: string;
  status: ExecutionIntentItem['preflight_status'];
  summary: Record<string, unknown>;
  artifact_sha256?: string;
  completed_at?: string;
  approval_id?: string;
  intent_status: ExecutionIntentItem['status'];
};

export type ExecutionLeaseItem = {
  id: string;
  intent_id: string;
  tool_name: string;
  risk_class: ExecutionIntentItem['risk_class'];
  status: 'active' | 'used' | 'expired' | 'revoked' | string;
  credential_mode: string;
  credential_hints: Record<string, unknown>;
  expires_at: string;
  used_at?: string;
  created_at: string;
};

export type ApprovalItem = {
  id: string;
  intent_id?: string;
  approval_mode: 'standard' | 'break_glass' | string;
  approval_group_id?: string;
  approval_step: number;
  approval_steps_total: number;
  request_id: string;
  tool_name: string;
  actor?: string;
  role?: string;
  input_redacted: Record<string, unknown>;
  context_redacted: Record<string, unknown>;
  reason: string;
  status: 'pending' | 'approved' | 'rejected' | 'expired' | string;
  decided_by?: string;
  decided_at?: string;
  expires_at: string;
  created_at: string;
};
