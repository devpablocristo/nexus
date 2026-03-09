import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  executeExecutionIntent,
  getExecutionIntentPreflight,
  getExecutionIntents,
  issueExecutionLease,
} from '../lib/api';
import type { ExecutionIntentItem } from '../lib/types';

function riskClassLabel(value: ExecutionIntentItem['risk_class']) {
  return value.split('_').join(' ');
}

function statusClass(value: string) {
  if (value === 'approved' || value === 'passed' || value === 'executed') return 'run-pill run-pill-allow';
  if (value === 'failed' || value === 'rejected' || value === 'destructive_prod') return 'run-pill run-pill-error';
  return 'run-pill run-pill-blocked';
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
