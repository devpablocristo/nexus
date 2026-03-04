import type {
  ActionItem,
  AlertRuleItem,
  ApprovalItem,
  AssistantResponse,
  AuditEventItem,
  EventItem,
  IncidentItem,
  PolicyProposalItem,
  SessionItem,
} from './types';

const baseUrl = import.meta.env.VITE_NEXUS_CORE_URL || 'http://localhost:8080';
const apiKey = import.meta.env.VITE_NEXUS_API_KEY || '';
const scopes = import.meta.env.VITE_NEXUS_SCOPES || 'admin:console:read,admin:console:write,audit:read';

const baseHeaders: HeadersInit = {
  'Content-Type': 'application/json',
  'X-NEXUS-CORE-KEY': apiKey,
  'X-NEXUS-SCOPES': scopes,
  'X-NEXUS-ACTOR': 'tower/ui',
};

async function call<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: { ...baseHeaders, ...(init?.headers || {}) },
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API ${res.status}: ${text}`);
  }
  return res.json() as Promise<T>;
}

export async function getEvents(cursor = 0, limit = 100): Promise<{ items: EventItem[]; next_cursor: number }> {
  return call(`/v1/events?cursor=${cursor}&limit=${limit}`);
}

export type AuditQueryParams = {
  tool_name?: string;
  decision?: 'allow' | 'deny';
  status?: 'success' | 'error' | 'blocked';
  from?: string; // RFC3339
  to?: string; // RFC3339
  limit?: number;
};

export async function getAuditEvents(params: AuditQueryParams = {}): Promise<{ items: AuditEventItem[] }> {
  const q = new URLSearchParams();
  if (params.tool_name) q.set('tool_name', params.tool_name);
  if (params.decision) q.set('decision', params.decision);
  if (params.status) q.set('status', params.status);
  if (params.from) q.set('from', params.from);
  if (params.to) q.set('to', params.to);
  q.set('limit', String(params.limit ?? 200));
  return call(`/v1/audit?${q.toString()}`);
}

export async function getActions(): Promise<{ items: ActionItem[] }> {
  return call('/v1/actions?limit=100');
}

export async function getIncidents(): Promise<{ items: IncidentItem[] }> {
  return call('/v1/incidents?limit=100');
}

export async function getPolicyProposals(): Promise<{ items: PolicyProposalItem[] }> {
  return call('/v1/policy-proposals?limit=100');
}

export async function approveProposal(id: string): Promise<PolicyProposalItem> {
  return call(`/v1/policy-proposals/${id}/approve`, { method: 'POST', body: '{}' });
}

export async function rejectProposal(id: string): Promise<PolicyProposalItem> {
  return call(`/v1/policy-proposals/${id}/reject`, { method: 'POST', body: '{}' });
}

export async function shadowProposal(id: string): Promise<PolicyProposalItem> {
  return call(`/v1/policy-proposals/${id}/shadow`, { method: 'POST', body: '{}' });
}

export async function queryAssistant(query: string): Promise<AssistantResponse> {
  return call('/v1/assistant/query', { method: 'POST', body: JSON.stringify({ query }) });
}

// -- Approvals --

export async function getApprovals(): Promise<{ items: ApprovalItem[] }> {
  return call('/v1/approvals');
}

export async function approveApproval(id: string): Promise<{ status: string }> {
  return call(`/v1/approvals/${id}/approve`, { method: 'POST', body: JSON.stringify({}) });
}

export async function rejectApproval(id: string): Promise<{ status: string }> {
  return call(`/v1/approvals/${id}/reject`, { method: 'POST', body: JSON.stringify({}) });
}

// -- Alert Rules --

export async function getAlertRules(): Promise<{ items: AlertRuleItem[] }> {
  return call('/v1/alert-rules');
}

export async function createAlertRule(rule: {
  name: string;
  metric: string;
  threshold: number;
  webhook_url: string;
  window_seconds?: number;
  cooldown_seconds?: number;
  enabled?: boolean;
}): Promise<AlertRuleItem> {
  return call('/v1/alert-rules', { method: 'POST', body: JSON.stringify(rule) });
}

export async function deleteAlertRule(id: string): Promise<{ status: string }> {
  return call(`/v1/alert-rules/${id}`, { method: 'DELETE' });
}

// -- Sessions --

export async function getSession(sessionId: string): Promise<SessionItem> {
  return call(`/v1/sessions/${sessionId}`);
}

export async function aiOperatorsTick(): Promise<{ status: string }> {
  const aiOperatorsBase = import.meta.env.VITE_NEXUS_AI_OPERATORS_URL || 'http://localhost:8000';
  const key = import.meta.env.VITE_AI_OPERATORS_ACTION_KEY || '';
  const res = await fetch(`${aiOperatorsBase}/v1/internal/tick`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Operator-Key': key,
    },
  });
  if (!res.ok) {
    throw new Error(`AI operators ${res.status}`);
  }
  return res.json() as Promise<{ status: string }>;
}
