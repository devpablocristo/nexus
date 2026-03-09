import { requestJSON } from '../api/client';
import type {
  APIKeyItem,
  AdminActivityItem,
  AdminBootstrap,
  AdminTenantSettings,
  AssistantResponse,
  AuditItem,
  BillingPlanCode,
  BillingStatus,
  EventItem,
  EgressRuleItem,
  ExecutionLeaseItem,
  IncidentItem,
  NotificationPreference,
  InAppNotification,
  OrgMemberItem,
  PolicyItem,
  PreflightReview,
  ProtectedResourceItem,
  RestoreEvidenceItem,
  SecretItem,
  ToolItem,
  UserMe,
  UsageSummary,
  ExecutionIntentItem,
} from './types';

// Core (gateway/config APIs)

export async function getTools(): Promise<{ items: ToolItem[] }> {
  return requestJSON('core', '/v1/tools');
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
  return requestJSON('core', '/v1/tools', { method: 'POST', body: JSON.stringify(payload) });
}

export async function updateTool(name: string, patch: Partial<ToolItem>): Promise<ToolItem> {
  return requestJSON('core', `/v1/tools/${name}`, { method: 'PUT', body: JSON.stringify(patch) });
}

export async function deleteTool(name: string): Promise<void> {
  await requestJSON<void>('core', `/v1/tools/${name}`, { method: 'DELETE' });
}

export async function getEgressRules(toolName: string): Promise<{ items: EgressRuleItem[] }> {
  return requestJSON('core', `/v1/tools/${toolName}/egress-rules`);
}

export async function createEgressRule(toolName: string, host: string): Promise<void> {
  await requestJSON<void>('core', `/v1/tools/${toolName}/egress-rules`, {
    method: 'POST',
    body: JSON.stringify({ host, enabled: true }),
  });
}

export async function deleteEgressRules(toolName: string): Promise<void> {
  await requestJSON<void>('core', `/v1/tools/${toolName}/egress-rules`, { method: 'DELETE' });
}

export async function getToolPolicies(toolName: string): Promise<{ items: PolicyItem[] }> {
  return requestJSON('core', `/v1/tools/${toolName}/policies`);
}

export async function createToolPolicy(
  toolName: string,
  payload: { name?: string; effect: 'allow' | 'deny'; priority: number; conditions: Record<string, unknown>; enabled: boolean },
): Promise<PolicyItem> {
  return requestJSON('core', `/v1/tools/${toolName}/policies`, { method: 'POST', body: JSON.stringify(payload) });
}

export async function updatePolicy(id: string, patch: { enabled?: boolean; priority?: number }): Promise<PolicyItem> {
  return requestJSON('core', `/v1/policies/${id}`, { method: 'PUT', body: JSON.stringify(patch) });
}

export async function listToolSecrets(toolName: string): Promise<{ items: SecretItem[] }> {
  return requestJSON('core', `/v1/tools/${toolName}/secrets`);
}

export async function upsertToolSecret(
  toolName: string,
  payload: { secret_type: string; key_name: string; value: string; enabled?: boolean },
): Promise<SecretItem> {
  return requestJSON('core', `/v1/tools/${toolName}/secrets`, { method: 'POST', body: JSON.stringify(payload) });
}

export async function deleteToolSecret(toolName: string, keyName: string): Promise<void> {
  const qs = new URLSearchParams({ key_name: keyName }).toString();
  await requestJSON<void>('core', `/v1/tools/${toolName}/secrets?${qs}`, { method: 'DELETE' });
}

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
  return requestJSON('core', `/v1/audit${qs ? `?${qs}` : ''}`);
}

// SaaS (identity, members, usage surfaces)

export async function getUserMe(): Promise<UserMe> {
  return requestJSON('saas', '/v1/users/me');
}

export async function getOrgMembers(orgID: string): Promise<{ items: OrgMemberItem[] }> {
  return requestJSON('saas', `/v1/orgs/${orgID}/members`);
}

export async function getOrgAPIKeys(orgID: string): Promise<{ items: APIKeyItem[] }> {
  return requestJSON('saas', `/v1/orgs/${orgID}/api-keys`);
}

export async function createOrgAPIKey(
  orgID: string,
  payload: { name: string; scopes: string[] },
): Promise<APIKeyItem & { api_key: string }> {
  return requestJSON('saas', `/v1/orgs/${orgID}/api-keys`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export async function revokeOrgAPIKey(orgID: string, keyID: string): Promise<void> {
  await requestJSON<void>('saas', `/v1/orgs/${orgID}/api-keys/${keyID}`, { method: 'DELETE' });
}

export async function rotateOrgAPIKey(orgID: string, keyID: string): Promise<{ id: string; org_id: string; api_key: string; rotated_at: string }> {
  return requestJSON('saas', `/v1/orgs/${orgID}/api-keys/${keyID}/rotate`, { method: 'POST' });
}

export async function getIncidents(limit = 100): Promise<{ items: IncidentItem[] }> {
  return requestJSON('saas', `/v1/incidents?limit=${limit}`);
}

export async function getEvents(limit = 100): Promise<{ items: EventItem[]; next_cursor: number }> {
  return requestJSON('saas', `/v1/events?limit=${limit}`);
}

export async function askAssistant(query: string): Promise<AssistantResponse> {
  return requestJSON('saas', '/v1/assistant/query', {
    method: 'POST',
    body: JSON.stringify({ query }),
  });
}

export async function getBillingStatus(): Promise<BillingStatus> {
  return requestJSON('saas', '/v1/billing/status');
}

export async function createCheckoutSession(planCode: BillingPlanCode): Promise<{ url: string }> {
  return requestJSON('saas', '/v1/billing/checkout', {
    method: 'POST',
    body: JSON.stringify({ plan_code: planCode }),
  });
}

export async function createPortalSession(): Promise<{ url: string }> {
  return requestJSON('saas', '/v1/billing/portal', {
    method: 'POST',
    body: JSON.stringify({}),
  });
}

export async function getUsageSummary(): Promise<UsageSummary> {
  return requestJSON('saas', '/v1/billing/usage');
}

export async function getAdminBootstrap(): Promise<AdminBootstrap> {
  return requestJSON('saas', '/v1/admin/bootstrap');
}

export async function getAdminTenantSettings(): Promise<AdminTenantSettings> {
  return requestJSON('saas', '/v1/admin/tenant-settings');
}

export async function updateAdminTenantSettings(req: {
  plan_code: string;
  hard_limits: Record<string, number>;
}): Promise<AdminTenantSettings> {
  return requestJSON('saas', '/v1/admin/tenant-settings', {
    method: 'PUT',
    body: JSON.stringify(req),
  });
}

export async function getAdminActivity(limit = 50): Promise<{ items: AdminActivityItem[] }> {
  const qs = new URLSearchParams({ limit: String(limit) }).toString();
  return requestJSON('saas', `/v1/admin/activity?${qs}`);
}

export async function getProtectedResources(): Promise<{ items: ProtectedResourceItem[] }> {
  return requestJSON('saas', '/v1/admin/protected-resources');
}

export async function getAdminRestoreEvidence(limit = 10, environment = ''): Promise<{ items: RestoreEvidenceItem[] }> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (environment) params.set('environment', environment);
  return requestJSON('saas', `/v1/admin/restore-evidence?${params.toString()}`);
}

export async function createProtectedResource(req: {
  name: string;
  resource_type: string;
  match_value: string;
  match_mode: string;
  environment: string;
  reason?: string;
  enabled?: boolean;
}): Promise<ProtectedResourceItem> {
  return requestJSON('saas', '/v1/admin/protected-resources', {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

export async function deleteProtectedResource(id: string): Promise<void> {
  await requestJSON<void>('saas', `/v1/admin/protected-resources/${id}`, { method: 'DELETE' });
}

export async function suspendTenant(orgID: string): Promise<AdminTenantSettings> {
  return requestJSON('saas', `/v1/admin/tenants/${orgID}/suspend`, {
    method: 'PUT',
    body: JSON.stringify({}),
  });
}

export async function reactivateTenant(orgID: string): Promise<AdminTenantSettings> {
  return requestJSON('saas', `/v1/admin/tenants/${orgID}/reactivate`, {
    method: 'PUT',
    body: JSON.stringify({}),
  });
}

export async function deleteTenant(orgID: string): Promise<AdminTenantSettings> {
  return requestJSON('saas', `/v1/admin/tenants/${orgID}`, { method: 'DELETE' });
}

export async function runTool(toolName: string, input: Record<string, unknown>): Promise<Record<string, unknown>> {
  return requestJSON('core', '/v1/run', {
    method: 'POST',
    body: JSON.stringify({ tool_name: toolName, input }),
  });
}

export async function getExecutionIntents(limit = 50): Promise<{ items: ExecutionIntentItem[] }> {
  const qs = new URLSearchParams({ limit: String(limit) }).toString();
  return requestJSON('core', `/v1/run/intents?${qs}`);
}

export async function getExecutionIntentPreflight(intentID: string): Promise<PreflightReview> {
  return requestJSON('core', `/v1/run/intents/${intentID}/preflight`);
}

export async function issueExecutionLease(intentID: string): Promise<ExecutionLeaseItem> {
  return requestJSON('core', `/v1/run/intents/${intentID}/lease`, { method: 'POST' });
}

export async function executeExecutionIntent(intentID: string, leaseID: string): Promise<Record<string, unknown>> {
  return requestJSON('core', `/v1/run/intents/${intentID}/execute`, {
    method: 'POST',
    body: JSON.stringify({ lease_id: leaseID }),
  });
}

export async function getNotificationPreferences(): Promise<{ items: NotificationPreference[] }> {
  return requestJSON('saas', '/v1/notifications/preferences');
}

export async function updateNotificationPreferences(
  items: Array<{ notification_type: string; enabled: boolean }>,
): Promise<void> {
  await requestJSON<void>('saas', '/v1/notifications/preferences', {
    method: 'PUT',
    body: JSON.stringify({ items }),
  });
}

export async function getInAppNotifications(limit = 10, offset = 0): Promise<{ items: InAppNotification[] }> {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) }).toString();
  return requestJSON('saas', `/v1/notifications?${params}`);
}

export async function getInAppNotificationUnreadCount(): Promise<{ count: number }> {
  return requestJSON('saas', '/v1/notifications/unread-count');
}

export async function markInAppNotificationRead(id: string): Promise<void> {
  await requestJSON<void>('saas', `/v1/notifications/${id}/read`, {
    method: 'PUT',
    body: JSON.stringify({}),
  });
}
