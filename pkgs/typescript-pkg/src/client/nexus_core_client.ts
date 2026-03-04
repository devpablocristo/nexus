export type NexusCoreClientConfig = {
  baseUrl: string;
  apiKey: string;
  scopes?: string[];
  actor?: string;
};

export class NexusCoreClient {
  constructor(private readonly cfg: NexusCoreClientConfig) {}

  private headers(extra?: Record<string, string>): HeadersInit {
    const h: Record<string, string> = {
      "Content-Type": "application/json",
      "X-NEXUS-CORE-KEY": this.cfg.apiKey,
    };
    if (this.cfg.scopes && this.cfg.scopes.length > 0) {
      h["X-NEXUS-SCOPES"] = this.cfg.scopes.join(",");
    }
    if (this.cfg.actor) {
      h["X-NEXUS-ACTOR"] = this.cfg.actor;
    }
    return { ...h, ...(extra || {}) };
  }

  async listEvents(cursor = 0, limit = 100) {
    const url = `${this.cfg.baseUrl}/v1/events?cursor=${cursor}&limit=${limit}`;
    const resp = await fetch(url, { headers: this.headers() });
    if (!resp.ok) throw new Error(`listEvents failed: ${resp.status}`);
    return resp.json();
  }

  async applyAction(payload: unknown) {
    const resp = await fetch(`${this.cfg.baseUrl}/v1/actions/apply`, {
      method: "POST",
      headers: this.headers(),
      body: JSON.stringify(payload),
    });
    if (!resp.ok) throw new Error(`applyAction failed: ${resp.status}`);
    return resp.json();
  }

  async listIncidents() {
    const resp = await fetch(`${this.cfg.baseUrl}/v1/incidents`, { headers: this.headers() });
    if (!resp.ok) throw new Error(`listIncidents failed: ${resp.status}`);
    return resp.json();
  }

  async listPolicyProposals() {
    const resp = await fetch(`${this.cfg.baseUrl}/v1/policy-proposals`, { headers: this.headers() });
    if (!resp.ok) throw new Error(`listPolicyProposals failed: ${resp.status}`);
    return resp.json();
  }
}
