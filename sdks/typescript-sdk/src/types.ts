export class NexusError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    public readonly errorMessage: string,
  ) {
    super(`[${status}] ${code}: ${errorMessage}`);
    this.name = 'NexusError';
  }
}

export interface IdempotencyInfo {
  present: boolean;
  outcome: string;
}

export interface RunResponse {
  request_id: string;
  decision: 'allow' | 'deny';
  tool_name: string;
  status: 'success' | 'error' | 'blocked';
  result?: unknown;
  reason?: string;
  error?: { code: string; message: string };
  latency_ms: number;
  idempotency?: IdempotencyInfo;
}

export interface SimulateResponse {
  request_id: string;
  decision: 'allow' | 'deny';
  tool_name: string;
  status: string;
  reason?: string;
  error?: unknown;
  explain: Record<string, unknown>;
  latency_ms: number;
}

export interface Tool {
  id: string;
  name: string;
  kind: string;
  method: string;
  url: string;
  action_type: string;
  enabled: boolean;
  description?: string;
  input_schema?: Record<string, unknown>;
  output_schema?: Record<string, unknown>;
  classification?: string;
  sensitivity?: string;
  risk_level?: number;
  created_at?: string;
  updated_at?: string;
}

export interface AuditEvent {
  request_id: string;
  tool_name: string;
  decision: 'allow' | 'deny';
  status: 'success' | 'error' | 'blocked';
  reason?: string;
  latency_ms: number;
  created_at: string;
  actor?: string;
  role?: string;
  error?: { code?: string; message?: string };
  input?: unknown;
  context?: unknown;
  output?: unknown;
  dlp_summary?: unknown;
}

export interface Policy {
  id: string;
  tool_id: string;
  effect: string;
  priority: number;
  conditions: Record<string, unknown>;
  limits: Record<string, unknown>;
  reason_template: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface RunOptions {
  idempotencyKey?: string;
  timeoutMs?: number;
  requestId?: string;
}

export interface AuditQueryOptions {
  toolName?: string;
  decision?: 'allow' | 'deny';
  status?: 'success' | 'error' | 'blocked';
  limit?: number;
}

export interface CreateToolOptions {
  name: string;
  kind?: string;
  method?: string;
  url: string;
  inputSchema?: Record<string, unknown>;
  actionType?: string;
  enabled?: boolean;
  description?: string;
  riskLevel?: number;
}

export interface CreatePolicyOptions {
  effect: 'allow' | 'deny';
  priority: number;
  conditions: Record<string, unknown>;
  limits?: Record<string, unknown>;
  reasonTemplate?: string;
  enabled?: boolean;
}

export interface ApprovalItem {
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
}

export interface AlertRuleItem {
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
}

export interface CreateAlertRuleOptions {
  name: string;
  metric: string;
  threshold: number;
  webhookUrl: string;
  windowSeconds?: number;
  cooldownSeconds?: number;
  toolName?: string;
  enabled?: boolean;
}

export interface SessionItem {
  id: string;
  session_id: string;
  actor?: string;
  total_calls: number;
  total_writes: number;
  total_denials: number;
  metadata: Record<string, unknown>;
  created_at: string;
  last_call_at: string;
}

export interface CreateOrgResponse {
  org_id: string;
  api_key: string;
  name: string;
}
