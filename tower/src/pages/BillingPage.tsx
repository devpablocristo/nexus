import { useMemo } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';

import { createCheckoutSession, createPortalSession, getBillingStatus, getUsageSummary } from '../lib/api';
import type { BillingHardLimits, BillingPlanCode, BillingStatus, UsageSummary } from '../lib/types';

type PlanCard = {
  code: BillingPlanCode;
  title: string;
  priceLabel: string;
  tools: number;
  rpm: number;
  auditDays: number;
};

const PLAN_CARDS: PlanCard[] = [
  { code: 'starter', title: 'Starter', priceLabel: 'Free', tools: 20, rpm: 300, auditDays: 30 },
  { code: 'growth', title: 'Growth', priceLabel: '$99 / month', tools: 75, rpm: 1200, auditDays: 90 },
  { code: 'enterprise', title: 'Enterprise', priceLabel: '$499 / month', tools: 250, rpm: 5000, auditDays: 365 },
];

const STATUS_LABELS: Record<BillingStatus['billing_status'], string> = {
  trialing: 'Trialing',
  active: 'Active',
  past_due: 'Past Due',
  canceled: 'Canceled',
  unpaid: 'Unpaid',
};

const METRICS: Array<{ key: keyof UsageSummary['counters']; label: string }> = [
  { key: 'api_calls', label: 'API Calls' },
  { key: 'events_ingested', label: 'Events Ingested' },
  { key: 'incidents_opened', label: 'Incidents Opened' },
  { key: 'actions_executed', label: 'Actions Executed' },
];

export default function BillingPage() {
  const [searchParams] = useSearchParams();
  const upgraded = searchParams.get('ok') === '1';

  const statusQuery = useQuery({
    queryKey: ['billing-status'],
    queryFn: getBillingStatus,
  });
  const usageQuery = useQuery({
    queryKey: ['billing-usage'],
    queryFn: getUsageSummary,
  });

  const checkoutMut = useMutation({
    mutationFn: (planCode: BillingPlanCode) => createCheckoutSession(planCode),
    onSuccess: ({ url }) => window.location.assign(url),
  });

  const portalMut = useMutation({
    mutationFn: createPortalSession,
    onSuccess: ({ url }) => window.location.assign(url),
  });

  const status = statusQuery.data;
  const usage = usageQuery.data ?? status?.usage;
  const maxCounterValue = useMemo(() => {
    if (!usage) return 1;
    const values = Object.values(usage.counters);
    return Math.max(1, ...values);
  }, [usage]);

  return (
    <div className="panel-page billing-page">
      <div className="billing-header">
        <div>
          <h2>Billing</h2>
          <p className="muted">Track plan, subscription state, and monthly usage.</p>
        </div>
        <div className="billing-actions">
          <button className="btn-secondary" onClick={() => portalMut.mutate()} disabled={portalMut.isPending || statusQuery.isPending}>
            {portalMut.isPending ? 'Opening portal...' : 'Manage Subscription'}
          </button>
          {status && status.plan_code !== 'enterprise' && (
            <button onClick={() => checkoutMut.mutate(nextUpgradePlan(status.plan_code))} disabled={checkoutMut.isPending}>
              {checkoutMut.isPending ? 'Redirecting...' : 'Upgrade Plan'}
            </button>
          )}
        </div>
      </div>

      {upgraded && (
        <div className="success-banner">
          <strong>Plan upgraded.</strong> Your billing status was refreshed.
        </div>
      )}

      {statusQuery.isLoading && <p className="muted">Loading billing status...</p>}
      {statusQuery.error && <p className="field-error">{(statusQuery.error as Error).message}</p>}

      {status && (
        <>
          <section className="billing-summary">
            <article className="summary-card">
              <p className="summary-label">Current Plan</p>
              <p className="summary-value">{capitalize(status.plan_code)}</p>
            </article>
            <article className="summary-card">
              <p className="summary-label">Billing Status</p>
              <p className={`summary-value status-${status.billing_status}`}>{STATUS_LABELS[status.billing_status]}</p>
            </article>
            <article className="summary-card">
              <p className="summary-label">Next Billing</p>
              <p className="summary-value">{status.current_period_end ? formatDate(status.current_period_end) : 'Not scheduled'}</p>
            </article>
          </section>

          <section className="billing-section">
            <h3>Usage This Period ({usage?.period ?? '-'})</h3>
            {usageQuery.error && <p className="field-error">{(usageQuery.error as Error).message}</p>}
            {usage ? (
              <div className="usage-list">
                {METRICS.map((metric) => {
                  const value = usage.counters[metric.key];
                  const width = `${Math.max(4, Math.round((value / maxCounterValue) * 100))}%`;
                  return (
                    <article key={metric.key} className="usage-item">
                      <div className="usage-topline">
                        <span>{metric.label}</span>
                        <strong>{formatNumber(value)} / &infin;</strong>
                      </div>
                      <div className="usage-bar">
                        <span style={{ width }} />
                      </div>
                    </article>
                  );
                })}
              </div>
            ) : (
              <p className="muted">No usage data available yet.</p>
            )}
          </section>

          <section className="billing-section">
            <h3>Plan Limits</h3>
            <LimitsGrid hardLimits={status.hard_limits} />
          </section>

          <section className="billing-section">
            <h3>Plans</h3>
            <div className="plan-grid">
              {PLAN_CARDS.map((plan) => {
                const isCurrent = plan.code === status.plan_code;
                const canUpgrade = !isCurrent && isHigherPlan(plan.code, status.plan_code);
                return (
                  <article key={plan.code} className={`plan-card${isCurrent ? ' current' : ''}`}>
                    <p className="plan-title">{plan.title}</p>
                    <p className="plan-price">{plan.priceLabel}</p>
                    <ul className="plan-features">
                      <li>{plan.tools} tools</li>
                      <li>{formatNumber(plan.rpm)} rpm</li>
                      <li>{plan.auditDays} days audit</li>
                    </ul>
                    {isCurrent ? (
                      <span className="plan-current">Current plan</span>
                    ) : canUpgrade ? (
                      <button onClick={() => checkoutMut.mutate(plan.code)} disabled={checkoutMut.isPending}>
                        Upgrade
                      </button>
                    ) : (
                      <button className="btn-secondary" onClick={() => portalMut.mutate()} disabled={portalMut.isPending}>
                        Manage in Portal
                      </button>
                    )}
                  </article>
                );
              })}
            </div>
          </section>
        </>
      )}
    </div>
  );
}

function LimitsGrid({ hardLimits }: { hardLimits: BillingHardLimits }) {
  return (
    <div className="limits-grid">
      <article>
        <p className="summary-label">Tools</p>
        <p className="summary-value">{formatNumber(hardLimits.tools_max)}</p>
      </article>
      <article>
        <p className="summary-label">Rate</p>
        <p className="summary-value">{formatNumber(hardLimits.run_rpm)} rpm</p>
      </article>
      <article>
        <p className="summary-label">Audit Retention</p>
        <p className="summary-value">{formatNumber(hardLimits.audit_retention_days)} days</p>
      </article>
    </div>
  );
}

function nextUpgradePlan(currentPlan: BillingPlanCode): BillingPlanCode {
  if (currentPlan === 'starter') return 'growth';
  if (currentPlan === 'growth') return 'enterprise';
  return 'enterprise';
}

function isHigherPlan(candidate: BillingPlanCode, current: BillingPlanCode): boolean {
  const rank: Record<BillingPlanCode, number> = { starter: 1, growth: 2, enterprise: 3 };
  return rank[candidate] > rank[current];
}

function capitalize(v: string): string {
  return v.charAt(0).toUpperCase() + v.slice(1);
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' });
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value);
}
