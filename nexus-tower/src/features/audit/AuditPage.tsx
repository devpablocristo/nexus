import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';

import { getAuditLog, getTools, type AuditQuery } from '../../lib/api';
import type { AuditItem, ToolItem } from '../../lib/types';

function decisionCls(d: string) {
  if (d === 'allow') return 'run-pill run-pill-allow';
  return 'run-pill run-pill-deny';
}

function statusCls(s: string) {
  if (s === 'success') return 'run-pill run-pill-allow';
  if (s === 'error') return 'run-pill run-pill-error';
  return 'run-pill run-pill-blocked';
}

function fmtTime(iso: string) {
  const d = new Date(iso);
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function fmtDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

export function AuditPage() {
  const [toolFilter, setToolFilter] = useState('');
  const [decisionFilter, setDecisionFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [limit, setLimit] = useState(50);
  const [selected, setSelected] = useState<AuditItem | null>(null);

  const toolsQuery = useQuery({
    queryKey: ['tools'],
    queryFn: getTools,
    refetchOnWindowFocus: false,
  });
  const toolNames: string[] = (toolsQuery.data?.items ?? []).map((t: ToolItem) => t.name);

  const query: AuditQuery = {};
  if (toolFilter) query.tool_name = toolFilter;
  if (decisionFilter) query.decision = decisionFilter;
  if (statusFilter) query.status = statusFilter;
  query.limit = limit;

  const auditQuery = useQuery({
    queryKey: ['audit', toolFilter, decisionFilter, statusFilter, limit],
    queryFn: () => getAuditLog(query),
    refetchInterval: 5000,
    refetchOnWindowFocus: false,
  });

  const items: AuditItem[] = auditQuery.data?.items ?? [];

  return (
    <div className="audit-page">
      <div className="audit-filters">
        <label>
          Tool
          <select value={toolFilter} onChange={(e) => setToolFilter(e.target.value)}>
            <option value="">All tools</option>
            {toolNames.map((n) => (
              <option key={n} value={n}>{n}</option>
            ))}
          </select>
        </label>

        <label>
          Decision
          <select value={decisionFilter} onChange={(e) => setDecisionFilter(e.target.value)}>
            <option value="">All</option>
            <option value="allow">allow</option>
            <option value="deny">deny</option>
          </select>
        </label>

        <label>
          Status
          <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
            <option value="">All</option>
            <option value="success">success</option>
            <option value="error">error</option>
            <option value="blocked">blocked</option>
          </select>
        </label>

        <label>
          Limit
          <select value={String(limit)} onChange={(e) => setLimit(Number(e.target.value))}>
            <option value="25">25</option>
            <option value="50">50</option>
            <option value="100">100</option>
            <option value="200">200</option>
          </select>
        </label>
      </div>

      {auditQuery.isLoading && <p className="muted">Loading audit log...</p>}

      {!auditQuery.isLoading && items.length === 0 && (
        <p className="muted">No audit entries found. Run requests through the gateway to generate entries.</p>
      )}

      <div className="audit-layout">
        <div className="audit-table-wrap">
          {items.length > 0 && (
            <table className="table audit-table">
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Tool</th>
                  <th>Decision</th>
                  <th>Status</th>
                  <th>Latency</th>
                  <th>Actor</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item, i) => (
                  <tr
                    key={item.request_id + i}
                    className={`audit-row ${selected?.request_id === item.request_id ? 'selected' : ''}`}
                    onClick={() => setSelected(item)}
                  >
                    <td className="audit-time">
                      <span className="audit-date">{fmtDate(item.created_at)}</span>{' '}
                      {fmtTime(item.created_at)}
                    </td>
                    <td><code className="run-tool">{item.tool_name}</code></td>
                    <td><span className={decisionCls(item.decision)}>{item.decision}</span></td>
                    <td><span className={statusCls(item.status)}>{item.status}</span></td>
                    <td className="audit-latency">{item.latency_ms}ms</td>
                    <td className="audit-actor">{item.actor ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {selected && (
          <div className="audit-detail">
            <div className="audit-detail-header">
              <h3>Request Detail</h3>
              <button className="icon-btn" onClick={() => setSelected(null)} aria-label="Close">✕</button>
            </div>

            <div className="audit-detail-grid">
              <p><strong>Request ID</strong><br /><code>{selected.request_id}</code></p>
              <p><strong>Tool</strong><br />{selected.tool_name}</p>
              <p><strong>Decision</strong><br /><span className={decisionCls(selected.decision)}>{selected.decision}</span></p>
              <p><strong>Status</strong><br /><span className={statusCls(selected.status)}>{selected.status}</span></p>
              <p><strong>Latency</strong><br />{selected.latency_ms}ms</p>
              <p><strong>Actor</strong><br />{selected.actor ?? '—'}</p>
              <p><strong>Role</strong><br />{selected.role ?? '—'}</p>
              <p><strong>Scopes</strong><br />{selected.scopes?.join(', ') || '—'}</p>
            </div>

            {selected.reason && (
              <div className="audit-detail-section">
                <strong>Reason</strong>
                <p>{selected.reason}</p>
              </div>
            )}

            {selected.error && (
              <div className="audit-detail-section">
                <strong>Error</strong>
                <p><code>{selected.error.code}</code>: {selected.error.message}</p>
              </div>
            )}

            {selected.idempotency_present && (
              <div className="audit-detail-section">
                <strong>Idempotency</strong>
                <p>Outcome: <code>{selected.idempotency_outcome}</code></p>
              </div>
            )}

            {selected.stage_durations_ms && Object.keys(selected.stage_durations_ms).length > 0 && (
              <div className="audit-detail-section">
                <strong>Stage Durations (ms)</strong>
                <div className="stage-bars">
                  {Object.entries(selected.stage_durations_ms)
                    .sort(([, a], [, b]) => b - a)
                    .map(([stage, ms]) => (
                      <div key={stage} className="stage-bar-row">
                        <span className="stage-bar-label">{stage}</span>
                        <div className="stage-bar-track">
                          <div
                            className="stage-bar-fill"
                            style={{ width: `${Math.min(100, (ms / selected.latency_ms) * 100)}%` }}
                          />
                        </div>
                        <span className="stage-bar-value">{ms}ms</span>
                      </div>
                    ))}
                </div>
              </div>
            )}

            {selected.input != null && (
              <details>
                <summary>Input</summary>
                <pre>{JSON.stringify(selected.input, null, 2)}</pre>
              </details>
            )}

            {selected.context != null && (
              <details>
                <summary>Context</summary>
                <pre>{JSON.stringify(selected.context, null, 2)}</pre>
              </details>
            )}

            {selected.output != null && (
              <details>
                <summary>Output</summary>
                <pre>{JSON.stringify(selected.output, null, 2)}</pre>
              </details>
            )}

            {selected.dlp_summary != null && (
              <details>
                <summary>DLP Summary</summary>
                <pre>{JSON.stringify(selected.dlp_summary, null, 2)}</pre>
              </details>
            )}

            <div className="audit-detail-hashes">
              {selected.event_hash && <p className="muted">Hash: <code>{selected.event_hash.slice(0, 16)}...</code></p>}
              {selected.prev_event_hash && <p className="muted">Prev: <code>{selected.prev_event_hash.slice(0, 16)}...</code></p>}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
