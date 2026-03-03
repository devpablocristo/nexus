import { useState } from 'react';
import { getSession } from '../../lib/api';
import type { SessionItem } from '../../lib/types';

export function SessionsPage() {
  const [sessionId, setSessionId] = useState('');
  const [session, setSession] = useState<SessionItem | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleLookup = async () => {
    if (!sessionId.trim()) return;
    setLoading(true);
    setError('');
    setSession(null);
    try {
      const data = await getSession(sessionId.trim());
      setSession(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Session not found');
    } finally {
      setLoading(false);
    }
  };

  const fmt = (d: string) => {
    try { return new Date(d).toLocaleString(); } catch { return d; }
  };

  return (
    <div className="sessions-page">
      <h2>Agent Sessions</h2>
      <p className="page-description">
        Look up an agent session to see call counts, write activity, and denial history.
      </p>

      <div className="session-lookup">
        <input
          value={sessionId}
          onChange={(e) => setSessionId(e.target.value)}
          placeholder="Enter session ID..."
          onKeyDown={(e) => e.key === 'Enter' && handleLookup()}
        />
        <button onClick={handleLookup} disabled={loading} className="btn-primary">
          {loading ? 'Searching...' : 'Lookup'}
        </button>
      </div>

      {error && <p className="error-msg">{error}</p>}

      {session && (
        <div className="session-detail">
          <div className="session-stats">
            <div className="stat-card">
              <span className="stat-value">{session.total_calls}</span>
              <span className="stat-label">Total Calls</span>
            </div>
            <div className="stat-card">
              <span className="stat-value">{session.total_writes}</span>
              <span className="stat-label">Writes</span>
            </div>
            <div className="stat-card">
              <span className="stat-value">{session.total_denials}</span>
              <span className="stat-label">Denials</span>
            </div>
          </div>

          <table className="detail-table">
            <tbody>
              <tr><td>Session ID</td><td><code>{session.session_id}</code></td></tr>
              <tr><td>Internal ID</td><td><code>{session.id}</code></td></tr>
              <tr><td>Actor</td><td>{session.actor ?? '—'}</td></tr>
              <tr><td>Created</td><td>{fmt(session.created_at)}</td></tr>
              <tr><td>Last Call</td><td>{fmt(session.last_call_at)}</td></tr>
            </tbody>
          </table>

          {session.metadata && Object.keys(session.metadata).length > 0 && (
            <details>
              <summary>Metadata</summary>
              <pre>{JSON.stringify(session.metadata, null, 2)}</pre>
            </details>
          )}
        </div>
      )}
    </div>
  );
}
