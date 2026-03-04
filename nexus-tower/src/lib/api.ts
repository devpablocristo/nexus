import type { AuditItem, EgressRuleItem, PolicyItem, ToolItem } from './types';

const coreUrl =
  import.meta.env.VITE_NEXUS_CORE_URL || 'http://localhost:8080';

const STORAGE_KEY = 'nexus_api_key';
const ALL_SCOPES = 'tools:read,tools:write,policy:read,policy:write,egress:read,egress:write,audit:read,gateway:run,admin:console:read,admin:console:write';

function getApiKey(): string {
  return localStorage.getItem(STORAGE_KEY) || import.meta.env.VITE_NEXUS_API_KEY || 'nexus-core-local-key';
}

async function call<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    'X-NEXUS-CORE-KEY': getApiKey(),
    'X-NEXUS-SCOPES': ALL_SCOPES,
    'X-NEXUS-ACTOR': 'tower/ui',
    ...(init?.headers || {}),
  };
  const res = await fetch(`${coreUrl}${path}`, { ...init, headers });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API ${res.status}: ${text}`);
  }
  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

// ── Tools ────────────────────────────────────────────────────────────────────

export async function getTools(): Promise<{ items: ToolItem[] }> {
  return call('/v1/tools');
}

export type CreateToolPayload = {
  name: string;
  kind: 'http';
  description?: string;
  method: string;
  url: string;
  input_schema?: Record<string, unknown>;
  action_type: string;
  classification: string;
  sensitivity?: string;
  risk_level?: number;
  enabled?: boolean;
};

export async function createTool(payload: CreateToolPayload): Promise<ToolItem> {
  return call('/v1/tools', { method: 'POST', body: JSON.stringify(payload) });
}

export async function updateTool(name: string, patch: Partial<ToolItem>): Promise<ToolItem> {
  return call(`/v1/tools/${name}`, { method: 'PUT', body: JSON.stringify(patch) });
}

export async function deleteTool(name: string): Promise<void> {
  await call<void>(`/v1/tools/${name}`, { method: 'DELETE' });
}

// ── Egress Rules ─────────────────────────────────────────────────────────────

export async function getEgressRules(toolName: string): Promise<{ items: EgressRuleItem[] }> {
  return call(`/v1/tools/${toolName}/egress-rules`);
}

export async function createEgressRule(toolName: string, host: string): Promise<void> {
  await call(`/v1/tools/${toolName}/egress-rules`, {
    method: 'POST',
    body: JSON.stringify({ host, enabled: true }),
  });
}

export async function deleteEgressRules(toolName: string): Promise<void> {
  await call(`/v1/tools/${toolName}/egress-rules`, { method: 'DELETE' });
}

// ── Policies ─────────────────────────────────────────────────────────────────

export async function getToolPolicies(toolName: string): Promise<{ items: PolicyItem[] }> {
  return call(`/v1/tools/${toolName}/policies`);
}

export async function createToolPolicy(
  toolName: string,
  payload: { name?: string; effect: 'allow' | 'deny'; priority: number; conditions: Record<string, unknown>; enabled: boolean },
): Promise<PolicyItem> {
  return call(`/v1/tools/${toolName}/policies`, { method: 'POST', body: JSON.stringify(payload) });
}

export async function updatePolicy(id: string, patch: { enabled?: boolean; priority?: number }): Promise<PolicyItem> {
  return call(`/v1/policies/${id}`, { method: 'PUT', body: JSON.stringify(patch) });
}

// ── Audit ────────────────────────────────────────────────────────────────────

export type AuditQuery = {
  tool_name?: string;
  decision?: string;
  status?: string;
  from?: string;
  to?: string;
  limit?: number;
};

export async function getAuditLog(query: AuditQuery = {}): Promise<{ items: AuditItem[] }> {
  const params = new URLSearchParams();
  if (query.tool_name) params.set('tool_name', query.tool_name);
  if (query.decision) params.set('decision', query.decision);
  if (query.status) params.set('status', query.status);
  if (query.from) params.set('from', query.from);
  if (query.to) params.set('to', query.to);
  if (query.limit) params.set('limit', String(query.limit));
  const qs = params.toString();
  return call(`/v1/audit${qs ? `?${qs}` : ''}`);
}
