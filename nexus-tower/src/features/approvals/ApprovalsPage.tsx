import { useEffect, useState } from 'react';
import { getApprovals, approveApproval, rejectApproval } from '../../lib/api';
import type { ApprovalItem } from '../../lib/types';

export function ApprovalsPage() {
  const [items, setItems] = useState<ApprovalItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<ApprovalItem | null>(null);
  const [acting, setActing] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const data = await getApprovals();
      setItems(data.items ?? []);
    } catch {
      /* ignore */
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleDecision = async (id: string, action: 'approve' | 'reject') => {
    setActing(true);
    try {
      if (action === 'approve') await approveApproval(id);
      else await rejectApproval(id);
      setSelected(null);
      await load();
    } catch {
      /* ignore */
    } finally {
      setActing(false);
    }
  };

  const fmt = (d: string) => {
    try { return new Date(d).toLocaleString(); } catch { return d; }
  };

  const statusColor = (s: string) => {
    if (s === 'pending') return 'var(--warning, #f59e0b)';
    if (s === 'approved') return 'var(--success, #10b981)';
    if (s === 'rejected') return 'var(--danger, #ef4444)';
    return 'var(--text-muted, #888)';
  };

  return (
    <div className="approvals-page">
      <div className="page-header">
        <h2>Pending Approvals</h2>
        <button onClick={load} disabled={loading} className="btn-secondary">
          {loading ? 'Loading...' : 'Refresh'}
        </button>
      </div>

      {items.length === 0 && !loading && (
        <p className="empty-state">No pending approvals.</p>
      )}

      <div className="approvals-grid">
        <div className="approvals-list">
          {items.map((it) => (
            <div
              key={it.id}
              className={`approval-card ${selected?.id === it.id ? 'selected' : ''}`}
              onClick={() => setSelected(it)}
            >
              <div className="approval-card-header">
                <span className="approval-tool">{it.tool_name}</span>
                <span className="approval-status" style={{ color: statusColor(it.status) }}>
                  {it.status}
                </span>
              </div>
              <div className="approval-card-meta">
                {it.actor && <span>Actor: {it.actor}</span>}
                <span>{fmt(it.created_at)}</span>
              </div>
              <div className="approval-reason">{it.reason}</div>
            </div>
          ))}
        </div>

        {selected && (
          <div className="approval-detail">
            <h3>Approval Detail</h3>
            <table className="detail-table">
              <tbody>
                <tr><td>ID</td><td><code>{selected.id}</code></td></tr>
                <tr><td>Request</td><td><code>{selected.request_id}</code></td></tr>
                <tr><td>Tool</td><td>{selected.tool_name}</td></tr>
                <tr><td>Actor</td><td>{selected.actor ?? '—'}</td></tr>
                <tr><td>Role</td><td>{selected.role ?? '—'}</td></tr>
                <tr><td>Status</td><td style={{ color: statusColor(selected.status) }}>{selected.status}</td></tr>
                <tr><td>Reason</td><td>{selected.reason}</td></tr>
                <tr><td>Expires</td><td>{fmt(selected.expires_at)}</td></tr>
                <tr><td>Created</td><td>{fmt(selected.created_at)}</td></tr>
                {selected.decided_by && <tr><td>Decided by</td><td>{selected.decided_by}</td></tr>}
              </tbody>
            </table>

            {selected.input_redacted && Object.keys(selected.input_redacted).length > 0 && (
              <details>
                <summary>Input (redacted)</summary>
                <pre>{JSON.stringify(selected.input_redacted, null, 2)}</pre>
              </details>
            )}

            {selected.status === 'pending' && (
              <div className="approval-actions">
                <button
                  className="btn-approve"
                  onClick={() => handleDecision(selected.id, 'approve')}
                  disabled={acting}
                >
                  Approve
                </button>
                <button
                  className="btn-reject"
                  onClick={() => handleDecision(selected.id, 'reject')}
                  disabled={acting}
                >
                  Reject
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
