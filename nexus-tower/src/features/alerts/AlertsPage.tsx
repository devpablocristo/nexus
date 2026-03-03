import { useEffect, useState } from 'react';
import { getAlertRules, createAlertRule, deleteAlertRule } from '../../lib/api';
import type { AlertRuleItem } from '../../lib/types';

const METRICS = ['deny_rate', 'error_rate', 'latency_p95', 'rate_limited_count'];

export function AlertsPage() {
  const [rules, setRules] = useState<AlertRuleItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);

  const [name, setName] = useState('');
  const [metric, setMetric] = useState(METRICS[0]);
  const [threshold, setThreshold] = useState('0.5');
  const [webhookUrl, setWebhookUrl] = useState('');
  const [windowSeconds, setWindowSeconds] = useState('300');
  const [cooldownSeconds, setCooldownSeconds] = useState('600');

  const load = async () => {
    setLoading(true);
    try {
      const data = await getAlertRules();
      setRules(data.items ?? []);
    } catch {
      /* ignore */
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleCreate = async () => {
    if (!name || !webhookUrl) return;
    try {
      await createAlertRule({
        name,
        metric,
        threshold: parseFloat(threshold),
        webhook_url: webhookUrl,
        window_seconds: parseInt(windowSeconds, 10),
        cooldown_seconds: parseInt(cooldownSeconds, 10),
        enabled: true,
      });
      setShowForm(false);
      setName('');
      setWebhookUrl('');
      await load();
    } catch {
      /* ignore */
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteAlertRule(id);
      await load();
    } catch {
      /* ignore */
    }
  };

  const fmt = (d: string) => {
    try { return new Date(d).toLocaleString(); } catch { return d; }
  };

  return (
    <div className="alerts-page">
      <div className="page-header">
        <h2>Alert Rules</h2>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button onClick={() => setShowForm(!showForm)} className="btn-secondary">
            {showForm ? 'Cancel' : '+ New Rule'}
          </button>
          <button onClick={load} disabled={loading} className="btn-secondary">Refresh</button>
        </div>
      </div>

      {showForm && (
        <div className="alert-form">
          <div className="form-row">
            <label>Name<input value={name} onChange={(e) => setName(e.target.value)} placeholder="high-deny-rate" /></label>
            <label>Metric
              <select value={metric} onChange={(e) => setMetric(e.target.value)}>
                {METRICS.map((m) => <option key={m} value={m}>{m}</option>)}
              </select>
            </label>
            <label>Threshold<input type="number" step="0.01" value={threshold} onChange={(e) => setThreshold(e.target.value)} /></label>
          </div>
          <div className="form-row">
            <label>Webhook URL<input value={webhookUrl} onChange={(e) => setWebhookUrl(e.target.value)} placeholder="https://hooks.slack.com/..." /></label>
            <label>Window (s)<input type="number" value={windowSeconds} onChange={(e) => setWindowSeconds(e.target.value)} /></label>
            <label>Cooldown (s)<input type="number" value={cooldownSeconds} onChange={(e) => setCooldownSeconds(e.target.value)} /></label>
          </div>
          <button onClick={handleCreate} className="btn-primary">Create Rule</button>
        </div>
      )}

      {rules.length === 0 && !loading && !showForm && (
        <p className="empty-state">No alert rules configured.</p>
      )}

      <div className="alert-rules-list">
        {rules.map((r) => (
          <div key={r.id} className="alert-rule-card">
            <div className="alert-rule-header">
              <strong>{r.name}</strong>
              <span className={`alert-badge ${r.enabled ? 'enabled' : 'disabled'}`}>
                {r.enabled ? 'Enabled' : 'Disabled'}
              </span>
            </div>
            <div className="alert-rule-body">
              <span>Metric: <code>{r.metric}</code></span>
              <span>Threshold: <code>{r.threshold}</code></span>
              <span>Window: {r.window_seconds}s</span>
              <span>Cooldown: {r.cooldown_seconds}s</span>
              {r.tool_name && <span>Tool: {r.tool_name}</span>}
              {r.last_fired_at && <span>Last fired: {fmt(r.last_fired_at)}</span>}
            </div>
            <div className="alert-rule-footer">
              <span className="alert-created">{fmt(r.created_at)}</span>
              <button onClick={() => handleDelete(r.id)} className="btn-danger-sm">Delete</button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
