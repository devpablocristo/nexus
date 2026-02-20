import type { ActionItem, AssistantResponse, EventItem, IncidentItem, PolicyProposalItem } from './types';

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
