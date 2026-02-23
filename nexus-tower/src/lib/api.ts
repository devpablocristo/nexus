import type {
  ActionItem,
  AssistantResponse,
  AuditEventItem,
  EventItem,
  IncidentItem,
  PolicyProposalItem,
  WorldEventItem,
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

export type WorldEventStreamBatch = {
  items: WorldEventItem[];
  next_seq: number;
};

export type WorldEventStreamOptions = {
  signal: AbortSignal;
  onBatch: (batch: WorldEventStreamBatch) => void;
  onPing?: (nextSeq: number) => void;
};

export async function streamWorldEvents(runId: string, fromSeq: number, opts: WorldEventStreamOptions): Promise<void> {
  const params = new URLSearchParams();
  params.set('run_id', runId);
  params.set('from_seq', String(Math.max(0, fromSeq)));
  params.set('limit', '200');

  const res = await fetch(`${baseUrl}/v1/world/events/stream?${params.toString()}`, {
    method: 'GET',
    headers: {
      ...baseHeaders,
      Accept: 'text/event-stream',
    },
    signal: opts.signal,
  });

  if (!res.ok || !res.body) {
    const text = await res.text();
    throw new Error(`world stream ${res.status}: ${text}`);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  const handleFrame = (frame: string) => {
    const lines = frame.split('\n');
    let eventName = 'message';
    const dataLines: string[] = [];
    for (const rawLine of lines) {
      const line = rawLine.trimEnd();
      if (line.startsWith('event:')) {
        eventName = line.slice('event:'.length).trim();
      } else if (line.startsWith('data:')) {
        dataLines.push(line.slice('data:'.length).trimStart());
      }
    }
    if (dataLines.length === 0) {
      return;
    }
    const payloadRaw = dataLines.join('\n');
    if (eventName === 'world.batch') {
      const payload = JSON.parse(payloadRaw) as WorldEventStreamBatch;
      opts.onBatch({
        items: Array.isArray(payload.items) ? payload.items : [],
        next_seq: Number(payload.next_seq || 0),
      });
      return;
    }
    if (eventName === 'cursor' || eventName === 'ping') {
      try {
        const payload = JSON.parse(payloadRaw) as { next_seq?: number };
        opts.onPing?.(Number(payload.next_seq || 0));
      } catch {
        // no-op
      }
      return;
    }
    if (eventName === 'error') {
      let message = 'world stream error';
      try {
        const payload = JSON.parse(payloadRaw) as { message?: string };
        if (typeof payload.message === 'string' && payload.message.trim() !== '') {
          message = payload.message;
        }
      } catch {
        // no-op
      }
      throw new Error(message);
    }
  };

  while (true) {
    const { value, done } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n');
    let frameEnd = buffer.indexOf('\n\n');
    while (frameEnd >= 0) {
      const frame = buffer.slice(0, frameEnd).trim();
      buffer = buffer.slice(frameEnd + 2);
      if (frame !== '') {
        handleFrame(frame);
      }
      frameEnd = buffer.indexOf('\n\n');
    }
  }
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
