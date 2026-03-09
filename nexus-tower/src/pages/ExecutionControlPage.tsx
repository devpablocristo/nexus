import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  approveApproval,
  executeExecutionIntent,
  getApproval,
  getExecutionIntentPreflight,
  getExecutionIntents,
  getPendingApprovals,
  getUserMe,
  issueExecutionLease,
  rejectApproval,
} from '../lib/api';
import type { ApprovalItem, ExecutionIntentItem } from '../lib/types';

function riskClassLabel(value: ExecutionIntentItem['risk_class']) {
  return value.split('_').join(' ');
}

function statusClass(value: string) {
  if (value === 'approved' || value === 'passed' || value === 'executed') return 'run-pill run-pill-allow';
  if (value === 'failed' || value === 'rejected' || value === 'destructive_prod') return 'run-pill run-pill-error';
  return 'run-pill run-pill-blocked';
}

function formatApprovalMode(value: ApprovalItem['approval_mode']) {
  return value.split('_').join(' ');
}

function formatDateTime(value?: string) {
  if (!value) return '—';
  return new Date(value).toLocaleString();
}

export default function ExecutionControlPage() {
  const queryClient = useQueryClient();
  const [selectedIntentID, setSelectedIntentID] = useState('');

  const intentsQuery = useQuery({
    queryKey: ['execution-intents', 50],
    queryFn: () => getExecutionIntents(50),
    refetchInterval: 5000,
    refetchOnWindowFocus: false,
  });

  const items = intentsQuery.data?.items ?? [];
  useEffect(() => {
    if (!items.length) {
      setSelectedIntentID('');
      return;
    }
    if (!selectedIntentID || !items.some((item) => item.id === selectedIntentID)) {
      setSelectedIntentID(items[0].id);
    }
  }, [items, selectedIntentID]);

  const selectedIntent = useMemo(
    () => items.find((item) => item.id === selectedIntentID) ?? null,
    [items, selectedIntentID],
  );

  const preflightQuery = useQuery({
    queryKey: ['execution-intent-preflight', selectedIntentID],
    queryFn: () => getExecutionIntentPreflight(selectedIntentID),
    enabled: Boolean(selectedIntentID),
  });

  const meQuery = useQuery({
    queryKey: ['user-me'],
    queryFn: () => getUserMe(),
  });

  const pendingApprovalsQuery = useQuery({
    queryKey: ['pending-approvals', 100],
    queryFn: () => getPendingApprovals(100),
    refetchInterval: 5000,
    refetchOnWindowFocus: false,
  });

  const linkedApprovalQuery = useQuery({
    queryKey: ['approval', selectedIntent?.approval_id],
    queryFn: () => getApproval(selectedIntent!.approval_id!),
    enabled: Boolean(selectedIntent?.approval_id),
  });

  const decidedBy =
    meQuery.data?.user?.email ??
    meQuery.data?.user?.name ??
    meQuery.data?.external_id ??
    'tower-user';

  const approvalMutation = useMutation({
    mutationFn: async ({ id, action }: { id: string; action: 'approve' | 'reject' }) => {
      if (action === 'approve') {
        await approveApproval(id, decidedBy);
        return;
      }
      await rejectApproval(id, decidedBy);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['execution-intents'] });
      await queryClient.invalidateQueries({ queryKey: ['execution-intent-preflight', selectedIntentID] });
      await queryClient.invalidateQueries({ queryKey: ['pending-approvals'] });
      await queryClient.invalidateQueries({ queryKey: ['approval', selectedIntent?.approval_id] });
    },
  });

  const selectedIntentApprovals = useMemo(() => {
    if (!selectedIntent) return [];
    const itemsByID = new Map<string, ApprovalItem>();
    const maybeLinked = linkedApprovalQuery.data;
    if (maybeLinked && maybeLinked.intent_id === selectedIntent.id) {
      itemsByID.set(maybeLinked.id, maybeLinked);
    }
    for (const item of pendingApprovalsQuery.data?.items ?? []) {
      if (item.intent_id === selectedIntent.id) {
        itemsByID.set(item.id, item);
      }
    }
    return Array.from(itemsByID.values()).sort((a, b) => {
      if (a.approval_step !== b.approval_step) return a.approval_step - b.approval_step;
      return a.created_at.localeCompare(b.created_at);
    });
  }, [linkedApprovalQuery.data, pendingApprovalsQuery.data?.items, selectedIntent]);

  const executeMutation = useMutation({
    mutationFn: async (intentID: string) => {
      const lease = await issueExecutionLease(intentID);
      return executeExecutionIntent(intentID, lease.id);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['execution-intents'] });
      await queryClient.invalidateQueries({ queryKey: ['execution-intent-preflight', selectedIntentID] });
    },
  });

  return (
    <section className="panel-page execution-control-page">
      <header className="execution-control-hero">
        <div>
          <p className="execution-control-kicker">Execution Control</p>
          <h2>Preflight Review</h2>
        </div>
        <p className="muted execution-control-intro">
          Review governed intents, inspect deterministic preflight evidence, and execute only the runs that already passed approval.
        </p>
      </header>

      {intentsQuery.error && <p className="field-error">{(intentsQuery.error as Error).message}</p>}

      <div className="execution-control-layout">
        <aside className="execution-intents-column">
          <div className="execution-column-header">
            <h3>Recent Intents</h3>
            <span className="muted">{items.length} visible</span>
          </div>
          {intentsQuery.isLoading && <p className="muted">Loading intents...</p>}
          {!intentsQuery.isLoading && items.length === 0 && (
            <p className="muted">No governed intents yet. Requests that require approval will appear here.</p>
          )}
          <div className="execution-intent-list">
            {items.map((item) => (
              <button
                key={item.id}
                type="button"
                className={`execution-intent-card ${selectedIntentID === item.id ? 'selected' : ''}`}
                onClick={() => setSelectedIntentID(item.id)}
              >
                <div className="execution-intent-topline">
                  <span className="execution-intent-tool">{item.tool_name}</span>
                  <span className={statusClass(item.status)}>{item.status}</span>
                </div>
                <div className="execution-intent-meta">
                  <span className={statusClass(item.preflight_status)}>{item.preflight_status}</span>
                  <span className="execution-risk">{riskClassLabel(item.risk_class)}</span>
                </div>
                <p className="execution-intent-reason">{item.reason}</p>
                <div className="execution-intent-footer">
                  <span>{formatDateTime(item.created_at)}</span>
                  {item.approval_id ? <code>{item.approval_id.slice(0, 8)}</code> : <span className="muted">no approval</span>}
                </div>
              </button>
            ))}
          </div>
        </aside>

        <div className="execution-review-column">
          {!selectedIntent && <p className="muted">Select an intent to inspect its preflight dossier.</p>}

          {selectedIntent && (
            <>
              <div className="execution-review-header">
                <div>
                  <p className="execution-control-kicker">Intent Dossier</p>
                  <h3>{selectedIntent.tool_name}</h3>
                </div>
                <div className="execution-review-badges">
                  <span className={statusClass(selectedIntent.status)}>{selectedIntent.status}</span>
                  <span className={statusClass(selectedIntent.preflight_status)}>{selectedIntent.preflight_status}</span>
                </div>
              </div>

              <div className="execution-review-grid">
                <article className="execution-review-card">
                  <span className="execution-review-label">Risk Class</span>
                  <strong>{riskClassLabel(selectedIntent.risk_class)}</strong>
                </article>
                <article className="execution-review-card">
                  <span className="execution-review-label">Intent ID</span>
                  <code>{selectedIntent.id}</code>
                </article>
                <article className="execution-review-card">
                  <span className="execution-review-label">Approval</span>
                  <code>{selectedIntent.approval_id ?? 'pending / n-a'}</code>
                </article>
                <article className="execution-review-card">
                  <span className="execution-review-label">Completed</span>
                  <strong>{formatDateTime(selectedIntent.preflight_completed_at)}</strong>
                </article>
              </div>

              <section className="execution-review-panel">
                <div className="execution-panel-header">
                  <h4>Preflight Summary</h4>
                  {preflightQuery.data?.artifact_sha256 && <code>{preflightQuery.data.artifact_sha256}</code>}
                </div>
                {preflightQuery.isLoading && <p className="muted">Loading preflight review...</p>}
                {preflightQuery.error && <p className="field-error">{(preflightQuery.error as Error).message}</p>}
                {preflightQuery.data && (
                  <>
                    <div className="execution-summary-list">
                      {Object.entries(preflightQuery.data.summary).map(([key, value]) => (
                        <div key={key} className="execution-summary-row">
                          <span>{key}</span>
                          <code>{typeof value === 'string' ? value : JSON.stringify(value)}</code>
                        </div>
                      ))}
                    </div>
                    <details className="execution-json-block" open>
                      <summary>Raw Review Payload</summary>
                      <pre>{JSON.stringify(preflightQuery.data, null, 2)}</pre>
                    </details>
                  </>
                )}
              </section>

              <section className="execution-review-panel">
                <div className="execution-panel-header">
                  <h4>Approval Chain</h4>
                  <span className="muted">
                    {selectedIntentApprovals.length > 0
                      ? `${selectedIntentApprovals.length} step${selectedIntentApprovals.length === 1 ? '' : 's'}`
                      : 'No approval records loaded'}
                  </span>
                </div>
                {(pendingApprovalsQuery.isLoading || linkedApprovalQuery.isLoading) && (
                  <p className="muted">Loading approval state...</p>
                )}
                {(pendingApprovalsQuery.error || linkedApprovalQuery.error) && (
                  <p className="field-error">
                    {((pendingApprovalsQuery.error ?? linkedApprovalQuery.error) as Error).message}
                  </p>
                )}
                {!pendingApprovalsQuery.isLoading && !linkedApprovalQuery.isLoading && selectedIntentApprovals.length === 0 && (
                  <p className="muted">This intent has no linked approval records yet.</p>
                )}
                <div className="execution-approvals-list">
                  {selectedIntentApprovals.map((approval) => (
                    <article key={approval.id} className="execution-approval-card">
                      <div className="execution-approval-topline">
                        <div>
                          <strong>
                            Step {approval.approval_step} / {approval.approval_steps_total}
                          </strong>
                          <p className="muted execution-approval-reason">{approval.reason}</p>
                        </div>
                        <span className={statusClass(approval.status)}>{approval.status}</span>
                      </div>
                      <div className="execution-approval-meta">
                        <span>{formatApprovalMode(approval.approval_mode)}</span>
                        <span>{approval.tool_name}</span>
                        <span>expires {formatDateTime(approval.expires_at)}</span>
                        {approval.decided_by ? <span>by {approval.decided_by}</span> : <span>awaiting reviewer</span>}
                      </div>
                      <div className="execution-approval-footer">
                        <code>{approval.id}</code>
                        {approval.status === 'pending' && (
                          <div className="execution-approval-actions">
                            <button
                              type="button"
                              className="execution-approval-btn"
                              disabled={approvalMutation.isPending}
                              onClick={() => approvalMutation.mutate({ id: approval.id, action: 'approve' })}
                            >
                              Approve
                            </button>
                            <button
                              type="button"
                              className="execution-approval-btn execution-approval-btn-danger"
                              disabled={approvalMutation.isPending}
                              onClick={() => approvalMutation.mutate({ id: approval.id, action: 'reject' })}
                            >
                              Reject
                            </button>
                          </div>
                        )}
                      </div>
                    </article>
                  ))}
                </div>
                {approvalMutation.error && <p className="field-error">{(approvalMutation.error as Error).message}</p>}
              </section>

              <section className="execution-review-panel">
                <div className="execution-panel-header">
                  <h4>Intent Payload</h4>
                  <span className="muted">Context frozen at intent creation</span>
                </div>
                <div className="execution-payload-grid">
                  <details className="execution-json-block" open>
                    <summary>Input</summary>
                    <pre>{JSON.stringify(selectedIntent.input, null, 2)}</pre>
                  </details>
                  <details className="execution-json-block" open>
                    <summary>Context</summary>
                    <pre>{JSON.stringify(selectedIntent.context, null, 2)}</pre>
                  </details>
                </div>
              </section>

              <section className="execution-review-panel execution-execute-panel">
                <div>
                  <h4>Execution Gate</h4>
                  <p className="muted">
                    Execution now requires a short-lived lease bound to the approved intent. Failed preflights and non-approved intents remain blocked by the runtime.
                  </p>
                </div>
                <button
                  className="execution-execute-btn"
                  disabled={selectedIntent.status !== 'approved' || executeMutation.isPending}
                  onClick={() => executeMutation.mutate(selectedIntent.id)}
                >
                  {executeMutation.isPending ? 'Issuing lease...' : 'Issue Lease & Execute'}
                </button>
                {executeMutation.error && <p className="field-error">{(executeMutation.error as Error).message}</p>}
                {executeMutation.isSuccess && <p className="muted">Execution request sent. The list will refresh automatically.</p>}
              </section>
            </>
          )}
        </div>
      </div>
    </section>
  );
}
