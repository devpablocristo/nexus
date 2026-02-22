import type {
  ActionItem,
  AssistantResponse,
  AuditEventItem,
  EventItem,
  IncidentItem,
  PolicyProposalItem,
  WorldEventsResponse,
  WorldReplayResponse,
  WorldRunCreateResponse,
  WorldRunsResponse,
  WorldStateResponse,
} from './types';

const baseUrl = import.meta.env.VITE_NEXUS_CORE_URL || 'http://localhost:8080';
const apiKey = import.meta.env.VITE_NEXUS_API_KEY || '';
const scopes = import.meta.env.VITE_NEXUS_SCOPES || 'admin:console:read,admin:console:write,audit:read';

const baseHeaders: HeadersInit = {
  'Content-Type': 'application/json',
  'X-NEXUS-GATEWAY-KEY': apiKey,
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

export async function getAuditEvents(toolName?: string, limit = 200): Promise<{ items: AuditEventItem[] }> {
  const params = new URLSearchParams();
  if (toolName) {
    params.set('tool_name', toolName);
  }
  params.set('limit', String(limit));
  return call(`/v1/audit?${params.toString()}`);
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

export async function getWorldRuns(limit = 100, cursor = ''): Promise<WorldRunsResponse> {
  const params = new URLSearchParams();
  params.set('limit', String(limit));
  if (cursor.trim() !== '') {
    params.set('cursor', cursor);
  }
  return call(`/v1/world/runs?${params.toString()}`);
}

export async function getWorldState(runId: string, stepId?: number): Promise<WorldStateResponse> {
  const params = new URLSearchParams();
  params.set('run_id', runId);
  if (typeof stepId === 'number') {
    params.set('step_id', String(stepId));
  }
  return call(`/v1/world/state?${params.toString()}`);
}

export async function getWorldEvents(runId: string, fromSeq = 0, limit = 200): Promise<WorldEventsResponse> {
  const params = new URLSearchParams();
  params.set('run_id', runId);
  params.set('from_seq', String(fromSeq));
  params.set('limit', String(limit));
  return call(`/v1/world/events?${params.toString()}`);
}

export async function createWorldRun(seed?: number, agentCount = 50): Promise<WorldRunCreateResponse> {
  const body: Record<string, unknown> = { agent_count: agentCount };
  if (typeof seed === 'number') {
    body.seed = seed;
  }
  return call('/v1/world/run/create', { method: 'POST', body: JSON.stringify(body) });
}

export async function replayWorldRun(runId: string): Promise<WorldReplayResponse> {
  return call('/v1/world/replay', { method: 'POST', body: JSON.stringify({ run_id: runId }) });
}

export async function operatorTick(): Promise<{ status: string }> {
  const operatorBase = import.meta.env.VITE_NEXUS_OPERATOR_URL || 'http://localhost:8000';
  const key = import.meta.env.VITE_OPERATOR_ACTION_KEY || '';
  const res = await fetch(`${operatorBase}/v1/internal/tick`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Operator-Key': key,
    },
  });
  if (!res.ok) {
    throw new Error(`Operator ${res.status}`);
  }
  return res.json() as Promise<{ status: string }>;
}
