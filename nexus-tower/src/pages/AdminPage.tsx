import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';

import { EditLimitsModal } from '../components/EditLimitsModal';
import {
  getAdminActivity,
  getAdminBootstrap,
  getBillingStatus,
  getOrgMembers,
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
