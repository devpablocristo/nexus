import type {
  AlertRuleItem,
  ApprovalItem,
  AuditEvent,
  AuditQueryOptions,
  CreateAlertRuleOptions,
  CreateOrgResponse,
  CreatePolicyOptions,
  CreateToolOptions,
  Policy,
  RunOptions,
  RunResponse,
  SessionItem,
  SimulateResponse,
  Tool,
} from './types.js';
import { NexusError } from './types.js';

export interface NexusClientOptions {
  baseUrl?: string;
  apiKey?: string;
  actor?: string;
  role?: string;
  scopes?: string;
  /** Custom fetch implementation (defaults to globalThis.fetch). */
  fetch?: typeof globalThis.fetch;
}

export class NexusClient {
  private readonly baseUrl: string;
  private readonly headers: Record<string, string>;
  private readonly fetchFn: typeof globalThis.fetch;

  constructor(opts: NexusClientOptions = {}) {
    this.baseUrl = (opts.baseUrl ?? 'http://localhost:8080').replace(/\/$/, '');
    this.fetchFn = opts.fetch ?? globalThis.fetch.bind(globalThis);

    this.headers = { 'Content-Type': 'application/json' };
    if (opts.apiKey) this.headers['X-NEXUS-CORE-KEY'] = opts.apiKey;
    if (opts.actor) this.headers['X-NEXUS-ACTOR'] = opts.actor;
    if (opts.role) this.headers['X-NEXUS-ROLE'] = opts.role;
    if (opts.scopes) this.headers['X-NEXUS-SCOPES'] = opts.scopes;
  }

  private async request<T>(method: string, path: string, opts?: { body?: unknown; headers?: Record<string, string>; params?: Record<string, string> }): Promise<T> {
    let url = `${this.baseUrl}${path}`;
    if (opts?.params) {
      const qs = new URLSearchParams(opts.params).toString();
      if (qs) url += `?${qs}`;
    }
    const res = await this.fetchFn(url, {
      method,
      headers: { ...this.headers, ...(opts?.headers ?? {}) },
      body: opts?.body !== undefined ? JSON.stringify(opts.body) : undefined,
    });
    const contentType = res.headers.get('content-type') ?? '';
    if (!res.ok) {
      let code = '';
      let message = '';
      if (contentType.includes('application/json')) {
        const body = await res.json() as Record<string, unknown>;
        const err = body.error as Record<string, string> | undefined;
        code = err?.code ?? (body.code as string ?? '');
        message = err?.message ?? (body.message as string ?? '');
      }
      throw new NexusError(res.status, code, message);
    }
    return (await res.json()) as T;
  }

  // -- Gateway --

  async run(toolName: string, input?: Record<string, unknown>, context?: Record<string, unknown>, opts?: RunOptions): Promise<RunResponse> {
    const body: Record<string, unknown> = { tool_name: toolName, input: input ?? {}, context: context ?? {} };
    if (opts?.requestId) body.request_id = opts.requestId;
    const headers: Record<string, string> = {};
    if (opts?.idempotencyKey) headers['Idempotency-Key'] = opts.idempotencyKey;
    if (opts?.timeoutMs != null) headers['X-Timeout-Ms'] = String(opts.timeoutMs);
    try {
      return await this.request<RunResponse>('POST', '/v1/run', { body, headers });
    } catch (e) {
      if (e instanceof NexusError && (e.status === 403 || e.status === 429)) {
        return { request_id: '', decision: 'deny', tool_name: toolName, status: 'blocked', error: { code: e.code, message: e.errorMessage }, latency_ms: 0 };
      }
      throw e;
    }
  }

  async simulate(toolName: string, input?: Record<string, unknown>, context?: Record<string, unknown>): Promise<SimulateResponse> {
    const body = { tool_name: toolName, input: input ?? {}, context: context ?? {} };
    try {
      return await this.request<SimulateResponse>('POST', '/v1/run/simulate', { body });
    } catch (e) {
      if (e instanceof NexusError && e.status === 403) {
        return { request_id: '', decision: 'deny', tool_name: toolName, status: 'blocked', reason: e.errorMessage, explain: {}, latency_ms: 0 };
      }
      throw e;
    }
  }

  // -- Tools --

  async listTools(): Promise<Tool[]> {
    const data = await this.request<{ items: Tool[] }>('GET', '/v1/tools');
    return data.items ?? [];
  }

  async getTool(name: string): Promise<Tool> {
    return this.request<Tool>('GET', `/v1/tools/${encodeURIComponent(name)}`);
  }

  async createTool(opts: CreateToolOptions): Promise<Tool> {
    return this.request<Tool>('POST', '/v1/tools', {
      body: {
        name: opts.name,
        kind: opts.kind ?? 'http',
        method: opts.method ?? 'POST',
        url: opts.url,
        input_schema: opts.inputSchema ?? { type: 'object' },
        action_type: opts.actionType ?? 'read',
        enabled: opts.enabled ?? true,
        risk_level: opts.riskLevel ?? 1,
        ...(opts.description ? { description: opts.description } : {}),
      },
    });
  }

  async updateTool(name: string, fields: Partial<Tool>): Promise<Tool> {
    return this.request<Tool>('PUT', `/v1/tools/${encodeURIComponent(name)}`, { body: fields });
  }

  // -- Policies --

  async listPolicies(toolName: string): Promise<Policy[]> {
    const data = await this.request<{ items: Policy[] }>('GET', `/v1/tools/${encodeURIComponent(toolName)}/policies`);
    return data.items ?? [];
  }

  async createPolicy(toolName: string, opts: CreatePolicyOptions): Promise<Policy> {
    return this.request<Policy>('POST', `/v1/tools/${encodeURIComponent(toolName)}/policies`, {
      body: {
        effect: opts.effect,
        priority: opts.priority,
        conditions: opts.conditions,
        limits: opts.limits ?? {},
        reason_template: opts.reasonTemplate ?? '',
        enabled: opts.enabled ?? true,
      },
    });
  }

  // -- Audit --

  async queryAudit(opts?: AuditQueryOptions): Promise<AuditEvent[]> {
    const params: Record<string, string> = { limit: String(opts?.limit ?? 200) };
    if (opts?.toolName) params.tool_name = opts.toolName;
    if (opts?.decision) params.decision = opts.decision;
    if (opts?.status) params.status = opts.status;
    const data = await this.request<{ items: AuditEvent[] }>('GET', '/v1/audit', { params });
    return data.items ?? [];
  }

  // -- Egress --

  async addEgressRule(toolName: string, host: string, enabled = true): Promise<unknown> {
    return this.request('POST', `/v1/tools/${encodeURIComponent(toolName)}/egress-rules`, { body: { host, enabled } });
  }

  async listEgressRules(toolName: string): Promise<Array<Record<string, unknown>>> {
    const data = await this.request<{ items: Array<Record<string, unknown>> }>('GET', `/v1/tools/${encodeURIComponent(toolName)}/egress-rules`);
    return data.items ?? [];
  }

  // -- Approvals --

  async listApprovals(limit = 100): Promise<ApprovalItem[]> {
    const data = await this.request<{ items: ApprovalItem[] }>('GET', '/v1/approvals', { params: { limit: String(limit) } });
    return data.items ?? [];
  }

  async getApproval(id: string): Promise<ApprovalItem> {
    return this.request<ApprovalItem>('GET', `/v1/approvals/${encodeURIComponent(id)}`);
  }

  async approve(id: string, decidedBy = ''): Promise<{ status: string }> {
    return this.request('POST', `/v1/approvals/${encodeURIComponent(id)}/approve`, { body: { decided_by: decidedBy } });
  }

  async reject(id: string, decidedBy = ''): Promise<{ status: string }> {
    return this.request('POST', `/v1/approvals/${encodeURIComponent(id)}/reject`, { body: { decided_by: decidedBy } });
  }

  // -- Alert Rules --

  async listAlertRules(): Promise<AlertRuleItem[]> {
    const data = await this.request<{ items: AlertRuleItem[] }>('GET', '/v1/alert-rules');
    return data.items ?? [];
  }

  async createAlertRule(opts: CreateAlertRuleOptions): Promise<AlertRuleItem> {
    return this.request<AlertRuleItem>('POST', '/v1/alert-rules', {
      body: {
        name: opts.name,
        metric: opts.metric,
        threshold: opts.threshold,
        webhook_url: opts.webhookUrl,
        window_seconds: opts.windowSeconds ?? 300,
        cooldown_seconds: opts.cooldownSeconds ?? 600,
        enabled: opts.enabled ?? true,
        ...(opts.toolName ? { tool_name: opts.toolName } : {}),
      },
    });
  }

  async deleteAlertRule(id: string): Promise<{ status: string }> {
    return this.request('DELETE', `/v1/alert-rules/${encodeURIComponent(id)}`);
  }

  // -- Sessions --

  async getSession(sessionId: string): Promise<SessionItem> {
    return this.request<SessionItem>('GET', `/v1/sessions/${encodeURIComponent(sessionId)}`);
  }

  // -- Orgs (Onboarding) --

  async createOrg(name: string, scopes?: string[]): Promise<CreateOrgResponse> {
    const body: Record<string, unknown> = { name };
    if (scopes) body.scopes = scopes;
    return this.request<CreateOrgResponse>('POST', '/v1/orgs', { body });
  }

  // -- Health --

  async health(): Promise<Record<string, unknown>> {
    return this.request('GET', '/healthz');
  }
}
