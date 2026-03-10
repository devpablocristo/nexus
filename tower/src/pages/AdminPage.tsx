import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';

import { EditLimitsModal } from '../components/EditLimitsModal';
import {
  createProtectedResource,
  deleteProtectedResource,
  getAdminActivity,
  getAdminBootstrap,
  getAdminRestoreEvidence,
  getBillingStatus,
  getOrgMembers,
  getProtectedResources,
  getTools,
  deleteTenant,
  reactivateTenant,
  suspendTenant,
  getUsageSummary,
  updateAdminTenantSettings,
} from '../lib/api';
import type { UsageSummary } from '../lib/types';

const USAGE_METRICS: Array<{ key: keyof UsageSummary['counters']; label: string }> = [
  { key: 'api_calls', label: 'API Calls' },
  { key: 'events_ingested', label: 'Events' },
  { key: 'incidents_opened', label: 'Incidents' },
  { key: 'actions_executed', label: 'Actions' },
];

export default function AdminPage() {
  const [editingLimits, setEditingLimits] = useState(false);
  const [tenantActionError, setTenantActionError] = useState('');
  const [protectedResourceError, setProtectedResourceError] = useState('');
  const [protectedResourceForm, setProtectedResourceForm] = useState({
    name: '',
    resource_type: 'terraform_address',
    match_value: '',
    match_mode: 'exact',
    environment: 'prod',
    reason: '',
  });
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const bootstrapQuery = useQuery({
    queryKey: ['admin-bootstrap'],
    queryFn: getAdminBootstrap,
  });

  const orgID = bootstrapQuery.data?.org_id ?? '';
  const canReadAdmin = bootstrapQuery.data?.can_read_admin ?? false;
  const canWriteAdmin = bootstrapQuery.data?.can_write_admin ?? false;

  const toolsQuery = useQuery({
    queryKey: ['tools'],
    queryFn: getTools,
    enabled: canReadAdmin,
  });

  const membersQuery = useQuery({
    queryKey: ['org-members', orgID],
    queryFn: () => getOrgMembers(orgID),
    enabled: canReadAdmin && Boolean(orgID),
  });

  const billingStatusQuery = useQuery({
    queryKey: ['billing-status'],
    queryFn: getBillingStatus,
    enabled: canReadAdmin,
  });

  const usageQuery = useQuery({
    queryKey: ['billing-usage'],
    queryFn: getUsageSummary,
    enabled: canReadAdmin,
  });

  const activityQuery = useQuery({
    queryKey: ['admin-activity', 10],
    queryFn: () => getAdminActivity(10),
    enabled: canReadAdmin,
  });

  const protectedResourcesQuery = useQuery({
    queryKey: ['admin-protected-resources'],
    queryFn: getProtectedResources,
    enabled: canReadAdmin,
  });

  const restoreEvidenceQuery = useQuery({
    queryKey: ['admin-restore-evidence', 5, 'prod'],
    queryFn: () => getAdminRestoreEvidence(5, 'prod'),
    enabled: canReadAdmin,
  });

  const updateLimitsMutation = useMutation({
    mutationFn: updateAdminTenantSettings,
    onSuccess: () => {
      setEditingLimits(false);
      queryClient.invalidateQueries({ queryKey: ['admin-bootstrap'] });
      queryClient.invalidateQueries({ queryKey: ['billing-status'] });
      queryClient.invalidateQueries({ queryKey: ['billing-usage'] });
      queryClient.invalidateQueries({ queryKey: ['admin-activity'] });
    },
  });

  const suspendMutation = useMutation({
    mutationFn: () => suspendTenant(orgID),
    onSuccess: () => {
      setTenantActionError('');
      queryClient.invalidateQueries({ queryKey: ['admin-bootstrap'] });
      navigate('/suspended', { replace: true });
    },
    onError: (error) => setTenantActionError((error as Error).message),
  });

  const reactivateMutation = useMutation({
    mutationFn: () => reactivateTenant(orgID),
    onSuccess: () => {
      setTenantActionError('');
      queryClient.invalidateQueries({ queryKey: ['admin-bootstrap'] });
    },
    onError: (error) => setTenantActionError((error as Error).message),
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteTenant(orgID),
    onSuccess: () => {
      setTenantActionError('');
      queryClient.invalidateQueries({ queryKey: ['admin-bootstrap'] });
      navigate('/suspended', { replace: true });
    },
    onError: (error) => setTenantActionError((error as Error).message),
  });

  const createProtectedResourceMutation = useMutation({
    mutationFn: createProtectedResource,
    onSuccess: () => {
      setProtectedResourceError('');
      setProtectedResourceForm({
        name: '',
        resource_type: 'terraform_address',
        match_value: '',
        match_mode: 'exact',
        environment: 'prod',
        reason: '',
      });
      queryClient.invalidateQueries({ queryKey: ['admin-protected-resources'] });
      queryClient.invalidateQueries({ queryKey: ['admin-activity'] });
    },
    onError: (error) => setProtectedResourceError((error as Error).message),
  });

  const deleteProtectedResourceMutation = useMutation({
    mutationFn: deleteProtectedResource,
    onSuccess: () => {
      setProtectedResourceError('');
      queryClient.invalidateQueries({ queryKey: ['admin-protected-resources'] });
      queryClient.invalidateQueries({ queryKey: ['admin-activity'] });
    },
    onError: (error) => setProtectedResourceError((error as Error).message),
  });

  const usage = usageQuery.data ?? billingStatusQuery.data?.usage;
  const maxCounterValue = useMemo(() => {
    if (!usage) return 1;
    const values = Object.values(usage.counters);
    return Math.max(1, ...values);
  }, [usage]);

  if (bootstrapQuery.isLoading) {
    return (
      <div className="panel-page admin-page">
        <h2>Admin Console</h2>
        <p className="muted">Loading admin settings...</p>
      </div>
    );
  }

  if (bootstrapQuery.error) {
    return (
      <div className="panel-page admin-page">
        <h2>Admin Console</h2>
        {isForbidden(bootstrapQuery.error) ? (
          <p className="muted">You don&apos;t have permission to view admin settings.</p>
        ) : (
          <p className="field-error">{(bootstrapQuery.error as Error).message}</p>
        )}
      </div>
    );
  }

  if (!bootstrapQuery.data || !canReadAdmin) {
    return (
      <div className="panel-page admin-page">
        <h2>Admin Console</h2>
        <p className="muted">You don&apos;t have permission to view admin settings.</p>
      </div>
    );
  }

  const tenant = bootstrapQuery.data.tenant_settings;
  const tenantStatus = tenant.status || 'active';
  const membersCount = membersQuery.data?.items.length ?? 0;
  const toolsCount = toolsQuery.data?.items.length ?? 0;
  const billingStatus = billingStatusQuery.data?.billing_status;
  const billingStatusLabel = billingStatus
    ? toStatusLabel(billingStatus)
    : billingStatusQuery.error
      ? 'Unavailable'
      : 'Loading';
  const billingStatusClass = billingStatus ? `status-${billingStatus}` : 'status-unavailable';

  return (
    <div className="panel-page admin-page">
      <div className="admin-hero">
        <div>
          <h2>Admin Console</h2>
          <p className="muted">Manage your organization&apos;s plan, limits, and activity.</p>
        </div>
        {!canWriteAdmin && <span className="admin-readonly">Read-only mode</span>}
      </div>

      <section className="billing-section">
        <h3>Overview</h3>
        <div className="admin-overview-grid">
          <article className="summary-card">
            <p className="summary-label">Plan</p>
            <p className="summary-value">{capitalize(tenant.plan_code)}</p>
          </article>
          <article className="summary-card">
            <p className="summary-label">Status</p>
            <p className={`summary-value ${billingStatusClass}`}>{billingStatusLabel}</p>
          </article>
          <article className="summary-card">
            <p className="summary-label">Members</p>
            <p className="summary-value">{formatNumber(membersCount)}</p>
          </article>
          <article className="summary-card">
            <p className="summary-label">Tools</p>
            <p className="summary-value">
              {formatNumber(toolsCount)} / {formatNumber(tenant.hard_limits.tools_max)}
            </p>
          </article>
        </div>
        {membersQuery.error && <p className="field-error">{(membersQuery.error as Error).message}</p>}
        {toolsQuery.error && <p className="field-error">{(toolsQuery.error as Error).message}</p>}
        {billingStatusQuery.error && <p className="field-error">{(billingStatusQuery.error as Error).message}</p>}
      </section>

      <section className="billing-section">
        <div className="admin-section-title">
          <h3>Plan &amp; Limits</h3>
          {canWriteAdmin && (
            <button className="btn-secondary" onClick={() => setEditingLimits(true)}>
              Edit Limits
            </button>
          )}
        </div>
        <div className="admin-limits-grid">
          <article>
            <p className="summary-label">Plan Code</p>
            <p className="summary-value">{capitalize(tenant.plan_code)}</p>
          </article>
          <article>
            <p className="summary-label">Tenant Status</p>
            <p className={`summary-value tenant-status-${tenantStatus}`}>{capitalize(tenantStatus)}</p>
          </article>
          <article>
            <p className="summary-label">Tools</p>
            <p className="summary-value">
              {formatNumber(toolsCount)} / {formatNumber(tenant.hard_limits.tools_max)}
            </p>
          </article>
          <article>
            <p className="summary-label">Rate Limit</p>
            <p className="summary-value">{formatNumber(tenant.hard_limits.run_rpm)} rpm</p>
          </article>
          <article>
            <p className="summary-label">Audit Retention</p>
            <p className="summary-value">{formatNumber(tenant.hard_limits.audit_retention_days)} days</p>
          </article>
        </div>
        {tenant.deleted_at && <p className="muted">Deleted at: {formatDateTime(tenant.deleted_at)}</p>}
        <p className="muted admin-updated-by">
          Last updated {formatDateTime(tenant.updated_at)} by {tenant.updated_by || 'system'}
        </p>
        {canWriteAdmin && (
          <div className="admin-tenant-actions">
            {tenantStatus === 'active' && (
              <button
                className="btn-secondary"
                disabled={suspendMutation.isPending}
                onClick={() => {
                  if (window.confirm('Suspend this tenant? Core requests will be rejected until reactivated.')) {
                    suspendMutation.mutate();
                  }
                }}
              >
                {suspendMutation.isPending ? 'Suspending...' : 'Suspend Tenant'}
              </button>
            )}
            {tenantStatus !== 'active' && (
              <button
                className="btn-secondary"
                disabled={reactivateMutation.isPending}
                onClick={() => reactivateMutation.mutate()}
              >
                {reactivateMutation.isPending ? 'Reactivating...' : 'Reactivate Tenant'}
              </button>
            )}
            {tenantStatus !== 'deleted' && (
              <button
                className="btn-danger-sm"
                disabled={deleteMutation.isPending}
                onClick={() => {
                  if (
                    window.confirm(
                      'Soft-delete this tenant? Data stays recoverable for 30 days and API traffic will be blocked.',
                    )
                  ) {
                    deleteMutation.mutate();
                  }
                }}
              >
                {deleteMutation.isPending ? 'Deleting...' : 'Delete Tenant'}
              </button>
            )}
          </div>
        )}
        {tenantActionError && <p className="field-error">{tenantActionError}</p>}
      </section>

      <section className="billing-section">
        <h3>Usage This Period ({usage?.period ?? '-'})</h3>
        {usageQuery.error && <p className="field-error">{(usageQuery.error as Error).message}</p>}
        {usage ? (
          <div className="usage-list">
            {USAGE_METRICS.map((metric) => {
              const value = usage.counters[metric.key];
              const width = `${Math.max(4, Math.round((value / maxCounterValue) * 100))}%`;
              return (
                <article key={metric.key} className="usage-item">
                  <div className="usage-topline">
                    <span>{metric.label}</span>
                    <strong>{formatNumber(value)}</strong>
                  </div>
                  <div className="usage-bar">
                    <span style={{ width }} />
                  </div>
                </article>
              );
            })}
          </div>
        ) : (
          <p className="muted">Usage data is currently unavailable.</p>
        )}
      </section>

      <section className="billing-section">
        <div className="admin-section-title">
          <h3>Protected Resources</h3>
          <span className="muted">Crown jewels blocked by deterministic preflight.</span>
        </div>
        {protectedResourcesQuery.error && <p className="field-error">{(protectedResourcesQuery.error as Error).message}</p>}
        <div className="admin-activity-table-wrap">
          <table className="table admin-activity-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Type</th>
                <th>Match</th>
                <th>Env</th>
                <th>Reason</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {(protectedResourcesQuery.data?.items ?? []).map((item) => (
                <tr key={item.id}>
                  <td>{item.name}</td>
                  <td>{item.resource_type}</td>
                  <td>
                    <code>{item.match_mode}</code> {item.match_value}
                  </td>
                  <td>{item.environment}</td>
                  <td>{item.reason || '—'}</td>
                  <td>
                    {canWriteAdmin && (
                      <button
                        className="btn-danger-sm"
                        disabled={deleteProtectedResourceMutation.isPending}
                        onClick={() => {
                          if (window.confirm(`Delete protected resource "${item.name}"?`)) {
                            deleteProtectedResourceMutation.mutate(item.id);
                          }
                        }}
                      >
                        Delete
                      </button>
                    )}
                  </td>
                </tr>
              ))}
              {!protectedResourcesQuery.isLoading && (protectedResourcesQuery.data?.items ?? []).length === 0 && (
                <tr>
                  <td colSpan={6} className="muted">
                    No protected resources configured yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        {canWriteAdmin && (
          <form
            className="admin-protected-form"
            onSubmit={(event) => {
              event.preventDefault();
              createProtectedResourceMutation.mutate(protectedResourceForm);
            }}
          >
            <input
              value={protectedResourceForm.name}
              onChange={(event) => setProtectedResourceForm((current) => ({ ...current, name: event.target.value }))}
              placeholder="Name"
            />
            <select
              value={protectedResourceForm.resource_type}
              onChange={(event) =>
                setProtectedResourceForm((current) => ({ ...current, resource_type: event.target.value }))
              }
            >
              <option value="terraform_address">Terraform Address</option>
              <option value="kubernetes_object">Kubernetes Object</option>
              <option value="host">Host</option>
              <option value="generic">Generic</option>
            </select>
            <input
              value={protectedResourceForm.match_value}
              onChange={(event) =>
                setProtectedResourceForm((current) => ({ ...current, match_value: event.target.value }))
              }
              placeholder="Match value"
            />
            <select
              value={protectedResourceForm.match_mode}
              onChange={(event) => setProtectedResourceForm((current) => ({ ...current, match_mode: event.target.value }))}
            >
              <option value="exact">Exact</option>
              <option value="contains">Contains</option>
            </select>
            <select
              value={protectedResourceForm.environment}
              onChange={(event) =>
                setProtectedResourceForm((current) => ({ ...current, environment: event.target.value }))
              }
            >
              <option value="prod">Prod</option>
              <option value="nonprod">Nonprod</option>
              <option value="*">Any</option>
            </select>
            <input
              value={protectedResourceForm.reason}
              onChange={(event) => setProtectedResourceForm((current) => ({ ...current, reason: event.target.value }))}
              placeholder="Reason"
            />
            <button className="btn-secondary" disabled={createProtectedResourceMutation.isPending}>
              {createProtectedResourceMutation.isPending ? 'Saving...' : 'Add Protected Resource'}
            </button>
          </form>
        )}
        {protectedResourceError && <p className="field-error">{protectedResourceError}</p>}
      </section>

      <section className="billing-section">
        <div className="admin-section-title">
          <h3>Recent Activity</h3>
          <Link to="/admin/activity" className="admin-view-all-link">
            View all →
          </Link>
        </div>
        {activityQuery.error && <p className="field-error">{(activityQuery.error as Error).message}</p>}
        <div className="admin-activity-table-wrap">
          <table className="table admin-activity-table">
            <thead>
              <tr>
                <th>When</th>
                <th>Who</th>
                <th>Action</th>
                <th>Resource</th>
              </tr>
            </thead>
            <tbody>
              {(activityQuery.data?.items ?? []).map((item) => (
                <tr key={item.id}>
                  <td>{formatDateTime(item.created_at)}</td>
                  <td>{item.actor || '—'}</td>
                  <td>{item.action}</td>
                  <td>{item.resource_type}</td>
                </tr>
              ))}
              {!activityQuery.isLoading && (activityQuery.data?.items ?? []).length === 0 && (
                <tr>
                  <td colSpan={4} className="muted">
                    No activity found.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      <section className="billing-section">
        <div className="admin-section-title">
          <h3>Restore Evidence</h3>
          <span className="muted">Recent DR validation artifacts for prod preflights.</span>
        </div>
        {restoreEvidenceQuery.error && <p className="field-error">{(restoreEvidenceQuery.error as Error).message}</p>}
        <div className="admin-activity-table-wrap">
          <table className="table admin-activity-table">
            <thead>
              <tr>
                <th>Completed</th>
                <th>System</th>
                <th>Status</th>
                <th>Snapshot</th>
                <th>Source</th>
              </tr>
            </thead>
            <tbody>
              {(restoreEvidenceQuery.data?.items ?? []).map((item) => (
                <tr key={item.id}>
                  <td>{formatDateTime(item.completed_at || item.created_at)}</td>
                  <td>{item.system}</td>
                  <td>{item.status}</td>
                  <td>{item.snapshot_id || '—'}</td>
                  <td>{item.source || '—'}</td>
                </tr>
              ))}
              {!restoreEvidenceQuery.isLoading && (restoreEvidenceQuery.data?.items ?? []).length === 0 && (
                <tr>
                  <td colSpan={5} className="muted">
                    No restore evidence recorded yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {editingLimits && (
        <EditLimitsModal
          settings={tenant}
          isSaving={updateLimitsMutation.isPending}
          error={updateLimitsMutation.error ? (updateLimitsMutation.error as Error).message : ''}
          onClose={() => setEditingLimits(false)}
          onSave={(req) => updateLimitsMutation.mutate(req)}
        />
      )}
    </div>
  );
}

function isForbidden(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  return error.message.includes('API 403');
}

function capitalize(v: string): string {
  return v.charAt(0).toUpperCase() + v.slice(1);
}

function toStatusLabel(v: string): string {
  return v
    .split('_')
    .map((chunk) => capitalize(chunk))
    .join(' ');
}

function formatDateTime(v?: string): string {
  if (!v) return '—';
  const date = new Date(v);
  return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value);
}
