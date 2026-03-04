import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  createEgressRule,
  createTool,
  createToolPolicy,
  type CreateToolPayload,
  deleteEgressRules,
  deleteTool,
  getEgressRules,
  getToolPolicies,
  getTools,
  updatePolicy,
  updateTool,
} from '../../lib/api';
import type { EgressRuleItem, PolicyItem, ToolItem } from '../../lib/types';

// ── helpers ──────────────────────────────────────────────────────────────────

function riskLabel(level: number) {
  if (level >= 3) return { label: 'High', cls: 'risk-high' };
  if (level >= 2) return { label: 'Medium', cls: 'risk-medium' };
  return { label: 'Low', cls: 'risk-low' };
}

function tryParseJson(raw: string): Record<string, unknown> | null {
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

// ── Register Tool form ────────────────────────────────────────────────────────

function RegisterToolForm({ onDone, onCancel }: { onDone: () => void; onCancel: () => void }) {
  const qc = useQueryClient();
  const [name, setName] = useState('my-service');
  const [description, setDescription] = useState('Demo service running on mock-tools');
  const [method, setMethod] = useState('POST');
  const [url, setUrl] = useState('http://mock-tools:8081/echo');
  const [actionType, setActionType] = useState('read');
  const [classification, setClassification] = useState('internal');
  const [sensitivity, setSensitivity] = useState('low');
  const [riskLevel, setRiskLevel] = useState('1');
  const [schemaRaw, setSchemaRaw] = useState('{\n  "type": "object",\n  "properties": {\n    "msg": { "type": "string" }\n  }\n}');
  const [schemaError, setSchemaError] = useState('');
  const [serverError, setServerError] = useState('');

  const mut = useMutation({
    mutationFn: (payload: CreateToolPayload) => createTool(payload),
    onSuccess: async (tool) => {
      // add to cache immediately — no extra GET needed
      qc.setQueryData<{ items: ToolItem[] }>(['tools'], (old) => ({
        items: [...(old?.items ?? []), tool],
      }));
      // automatically add egress rule from the registered URL
      try {
        const hostname = new URL(tool.url).hostname;
        await createEgressRule(tool.name, hostname);
      } catch {
        // non-fatal: egress can be added manually
      }
      onDone();
    },
    onError: (e: Error) => setServerError(e.message),
  });

  const handleSubmit = () => {
    setSchemaError('');
    setServerError('');
    if (!name.trim() || !url.trim()) return;
    const parsed = tryParseJson(schemaRaw);
    if (!parsed) {
      setSchemaError('Invalid JSON schema');
      return;
    }
    mut.mutate({
      name: name.trim(),
      description: description.trim() || undefined,
      kind: 'http',
      method,
      url: url.trim(),
      input_schema: parsed,
      action_type: actionType,
      classification,
      sensitivity,
      risk_level: parseInt(riskLevel, 10),
      enabled: true,
    });
  };

  return (
    <div className="tool-form-overlay">
      <div className="tool-form-card">
        <div className="tool-form-header">
          <h2>Register Tool</h2>
          <button className="icon-btn" onClick={onCancel} aria-label="Close">✕</button>
        </div>

        <p className="tool-form-hint">
          A <strong>tool</strong> is a single endpoint of a backend service.
          Nexus will route consumer requests to this URL after evaluating policies, rate limits, and secrets.
        </p>

        <div className="tool-form-grid">
          <label className="tool-form-label full">
            Name <span className="required">*</span>
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="payment-service" />
          </label>

          <label className="tool-form-label full">
            Description
            <input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Charges a payment method" />
          </label>

          <label className="tool-form-label">
            Method
            <select value={method} onChange={(e) => setMethod(e.target.value)}>
              {['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map((m) => (
                <option key={m}>{m}</option>
              ))}
            </select>
          </label>

          <label className="tool-form-label full">
            Endpoint URL <span className="required">*</span>
            <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="http://payments-svc:3000/charge" />
          </label>

          <label className="tool-form-label">
            Action type
            <select value={actionType} onChange={(e) => setActionType(e.target.value)}>
              <option value="read">read — no side effects</option>
              <option value="write">write — mutates state</option>
            </select>
          </label>

          <label className="tool-form-label">
            Classification
            <select value={classification} onChange={(e) => setClassification(e.target.value)}>
              <option value="internal">internal</option>
              <option value="external">external</option>
            </select>
          </label>

          <label className="tool-form-label">
            Sensitivity
            <select value={sensitivity} onChange={(e) => setSensitivity(e.target.value)}>
              <option value="low">low</option>
              <option value="medium">medium</option>
              <option value="high">high</option>
            </select>
          </label>

          <label className="tool-form-label">
            Risk level
            <select value={riskLevel} onChange={(e) => setRiskLevel(e.target.value)}>
              <option value="1">1 — Low</option>
              <option value="2">2 — Medium</option>
              <option value="3">3 — High</option>
            </select>
          </label>

          <label className="tool-form-label full">
            Input schema (JSON Schema)
            <textarea
              rows={6}
              value={schemaRaw}
              onChange={(e) => setSchemaRaw(e.target.value)}
              className={schemaError ? 'input-error' : ''}
            />
            {schemaError && <span className="field-error">{schemaError}</span>}
          </label>
        </div>

        {serverError && <p className="form-server-error">{serverError}</p>}

        <div className="tool-form-actions">
          <button onClick={onCancel} className="btn-secondary">Cancel</button>
          <button onClick={handleSubmit} disabled={mut.isPending || !name.trim() || !url.trim()}>
            {mut.isPending ? 'Registering…' : 'Register Tool'}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Egress Rules tab ─────────────────────────────────────────────────────────

function EgressTab({ tool }: { tool: ToolItem }) {
  const [host, setHost] = useState('');
  const [addError, setAddError] = useState('');
  const qc = useQueryClient();

  const query = useQuery({
    queryKey: ['egress', tool.name],
    queryFn: () => getEgressRules(tool.name),
  });

  const addMut = useMutation({
    mutationFn: (h: string) => createEgressRule(tool.name, h),
    onSuccess: () => {
      setHost('');
      setAddError('');
      qc.invalidateQueries({ queryKey: ['egress', tool.name] });
    },
    onError: (e: Error) => setAddError(e.message),
  });

  const delMut = useMutation({
    mutationFn: () => deleteEgressRules(tool.name),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['egress', tool.name] }),
  });

  const items: EgressRuleItem[] = query.data?.items ?? [];

  return (
    <div className="tab-content">
      <p className="tab-desc">
        Nexus enforces egress by default-deny. Declare every hostname this tool is allowed to call.
        The host is extracted from the tool URL automatically on registration, but you can add more.
      </p>

      <div className="egress-add-row">
        <input
          value={host}
          onChange={(e) => setHost(e.target.value)}
          placeholder="payments-svc"
          onKeyDown={(e) => e.key === 'Enter' && host.trim() && addMut.mutate(host.trim())}
        />
        <button
          onClick={() => host.trim() && addMut.mutate(host.trim())}
          disabled={!host.trim() || addMut.isPending}
        >
          + Allow host
        </button>
      </div>
      {addError && <p className="field-error">{addError}</p>}

      {items.length === 0 && !query.isLoading && (
        <p className="muted">No egress rules — tool cannot make outbound calls.</p>
      )}

      {items.length > 0 && (
        <>
          <table className="table">
            <thead>
              <tr>
                <th>Host</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {items.map((r) => (
                <tr key={r.id}>
                  <td><code>{r.host}</code></td>
                  <td>
                    <span className={r.enabled ? 'badge-enabled' : 'badge-disabled'}>
                      {r.enabled ? 'allowed' : 'denied'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <div className="tab-header-row" style={{ marginTop: '8px' }}>
            <button
              className="btn-danger-sm"
              onClick={() => delMut.mutate()}
              disabled={delMut.isPending}
            >
              {delMut.isPending ? 'Clearing…' : 'Clear all rules'}
            </button>
          </div>
        </>
      )}
    </div>
  );
}

// ── Policies tab ─────────────────────────────────────────────────────────────

function PoliciesTab({ tool }: { tool: ToolItem }) {
  const qc = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [effect, setEffect] = useState<'allow' | 'deny'>('allow');
  const [priority, setPriority] = useState('10');
  const [conditionsRaw, setConditionsRaw] = useState('{}');
  const [condError, setCondError] = useState('');
  const [addError, setAddError] = useState('');

  const query = useQuery({
    queryKey: ['policies', tool.name],
    queryFn: () => getToolPolicies(tool.name),
  });

  const addMut = useMutation({
    mutationFn: (payload: Parameters<typeof createToolPolicy>[1]) =>
      createToolPolicy(tool.name, payload),
    onSuccess: () => {
      setShowForm(false);
      setConditionsRaw('{}');
      setAddError('');
      qc.invalidateQueries({ queryKey: ['policies', tool.name] });
    },
    onError: (e: Error) => setAddError(e.message),
  });

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => updatePolicy(id, { enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['policies', tool.name] }),
  });

  const handleAdd = () => {
    setCondError('');
    const parsed = tryParseJson(conditionsRaw);
    if (!parsed) { setCondError('Invalid JSON'); return; }
    addMut.mutate({ effect, priority: parseInt(priority, 10), conditions: parsed, enabled: true });
  };

  const items: PolicyItem[] = query.data?.items ?? [];

  return (
    <div className="tab-content">
      <p className="tab-desc">
        Policies evaluate each run request. <strong>write</strong> tools default-deny — they need at least one
        allow policy. <strong>read</strong> tools default-allow.
      </p>

      <div className="tab-header-row">
        <button onClick={() => setShowForm(!showForm)} className="btn-secondary">
          {showForm ? 'Cancel' : '+ Add Policy'}
        </button>
      </div>

      {showForm && (
        <div className="policy-form">
          <div className="tool-form-grid">
            <label className="tool-form-label">
              Effect
              <select value={effect} onChange={(e) => setEffect(e.target.value as 'allow' | 'deny')}>
                <option value="allow">allow</option>
                <option value="deny">deny</option>
              </select>
            </label>
            <label className="tool-form-label">
              Priority (lower = first)
              <input type="number" value={priority} onChange={(e) => setPriority(e.target.value)} />
            </label>
            <label className="tool-form-label full">
              Conditions (JSON — empty = always matches)
              <textarea
                rows={4}
                value={conditionsRaw}
                onChange={(e) => setConditionsRaw(e.target.value)}
                className={condError ? 'input-error' : ''}
              />
              {condError && <span className="field-error">{condError}</span>}
            </label>
          </div>
          {addError && <p className="field-error">{addError}</p>}
          <div className="tool-form-actions">
            <button onClick={handleAdd} disabled={addMut.isPending}>
              {addMut.isPending ? 'Saving…' : 'Add Policy'}
            </button>
          </div>
        </div>
      )}

      {items.length === 0 && !query.isLoading && !showForm && (
        <p className="muted">No policies. Write tools are blocked until at least one allow policy is added.</p>
      )}

      {items.length > 0 && (
        <table className="table">
          <thead>
            <tr>
              <th>Priority</th>
              <th>Effect</th>
              <th>Conditions</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {items.map((p) => (
              <tr key={p.id}>
                <td>{p.priority}</td>
                <td>
                  <span className={p.effect === 'allow' ? 'badge-enabled' : 'badge-disabled'}>
                    {p.effect}
                  </span>
                </td>
                <td>
                  <code className="conditions-preview">
                    {Object.keys(p.conditions).length === 0
                      ? '(always)'
                      : JSON.stringify(p.conditions)}
                  </code>
                </td>
                <td>
                  <span className={p.enabled ? 'badge-enabled' : 'badge-disabled'}>
                    {p.enabled ? 'active' : 'disabled'}
                  </span>
                </td>
                <td>
                  <button
                    className="btn-secondary-sm"
                    onClick={() => toggleMut.mutate({ id: p.id, enabled: !p.enabled })}
                    disabled={toggleMut.isPending}
                  >
                    {p.enabled ? 'Disable' : 'Enable'}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

// ── Edit Tool form ────────────────────────────────────────────────────────────

function EditToolForm({ tool, onDone, onCancel }: { tool: ToolItem; onDone: () => void; onCancel: () => void }) {
  const qc = useQueryClient();
  const [description, setDescription] = useState(tool.description ?? '');
  const [method, setMethod] = useState<'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'>(tool.method);
  const [url, setUrl] = useState(tool.url);
  const [actionType, setActionType] = useState<'read' | 'write'>(tool.action_type);
  const [classification, setClassification] = useState<'internal' | 'external'>(tool.classification);
  const [sensitivity, setSensitivity] = useState<'low' | 'medium' | 'high'>(tool.sensitivity);
  const [riskLevel, setRiskLevel] = useState(String(tool.risk_level));
  const [schemaRaw, setSchemaRaw] = useState(
    tool.input_schema ? JSON.stringify(tool.input_schema, null, 2) : '{\n  "type": "object"\n}',
  );
  const [schemaError, setSchemaError] = useState('');
  const [serverError, setServerError] = useState('');

  const mut = useMutation({
    mutationFn: (patch: Parameters<typeof updateTool>[1]) => updateTool(tool.name, patch),
    onSuccess: (updated) => {
      qc.setQueryData<{ items: ToolItem[] }>(['tools'], (old) => ({
        items: (old?.items ?? []).map((t) => (t.id === updated.id ? updated : t)),
      }));
      onDone();
    },
    onError: (e: Error) => setServerError(e.message),
  });

  const handleSave = () => {
    setSchemaError('');
    setServerError('');
    const parsed = tryParseJson(schemaRaw);
    if (!parsed) { setSchemaError('Invalid JSON schema'); return; }
    mut.mutate({
      description: description || undefined,
      method,
      url,
      action_type: actionType as ToolItem['action_type'],
      classification: classification as ToolItem['classification'],
      sensitivity: sensitivity as ToolItem['sensitivity'],
      risk_level: parseInt(riskLevel, 10) as ToolItem['risk_level'],
      input_schema: parsed,
    });
  };

  return (
    <div className="tab-content">
      <div className="tool-form-grid">
        <label className="tool-form-label full">
          Description
          <input value={description} onChange={(e) => setDescription(e.target.value)} />
        </label>
        <label className="tool-form-label">
          Method
          <select value={method} onChange={(e) => setMethod(e.target.value as typeof method)}>
            {['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map((m) => <option key={m}>{m}</option>)}
          </select>
        </label>
        <label className="tool-form-label full">
          Endpoint URL
          <input value={url} onChange={(e) => setUrl(e.target.value)} />
        </label>
        <label className="tool-form-label">
          Action type
          <select value={actionType} onChange={(e) => setActionType(e.target.value as 'read' | 'write')}>
            <option value="read">read</option>
            <option value="write">write</option>
          </select>
        </label>
        <label className="tool-form-label">
          Classification
          <select value={classification} onChange={(e) => setClassification(e.target.value as 'internal' | 'external')}>
            <option value="internal">internal</option>
            <option value="external">external</option>
          </select>
        </label>
        <label className="tool-form-label">
          Sensitivity
          <select value={sensitivity} onChange={(e) => setSensitivity(e.target.value as 'low' | 'medium' | 'high')}>
            <option value="low">low</option>
            <option value="medium">medium</option>
            <option value="high">high</option>
          </select>
        </label>
        <label className="tool-form-label">
          Risk level
          <select value={riskLevel} onChange={(e) => setRiskLevel(e.target.value)}>
            <option value="1">1 — Low</option>
            <option value="2">2 — Medium</option>
            <option value="3">3 — High</option>
          </select>
        </label>
        <label className="tool-form-label full">
          Input schema (JSON Schema)
          <textarea
            rows={6}
            value={schemaRaw}
            onChange={(e) => setSchemaRaw(e.target.value)}
            className={schemaError ? 'input-error' : ''}
          />
          {schemaError && <span className="field-error">{schemaError}</span>}
        </label>
      </div>
      {serverError && <p className="form-server-error">{serverError}</p>}
      <div className="tool-form-actions">
        <button onClick={onCancel} className="btn-secondary">Cancel</button>
        <button onClick={handleSave} disabled={mut.isPending}>
          {mut.isPending ? 'Saving…' : 'Save changes'}
        </button>
      </div>
    </div>
  );
}

// ── Tool Detail panel ─────────────────────────────────────────────────────────

type Tab = 'details' | 'edit' | 'egress' | 'policies';

function ToolDetail({ tool, onClose, onDeleted }: { tool: ToolItem; onClose: () => void; onDeleted: () => void }) {
  const qc = useQueryClient();
  const [tab, setTab] = useState<Tab>('details');
  const [confirmDelete, setConfirmDelete] = useState(false);

  const toggleMut = useMutation({
    mutationFn: (enabled: boolean) => updateTool(tool.name, { enabled }),
    onSuccess: (updated) => {
      qc.setQueryData<{ items: ToolItem[] }>(['tools'], (old) => ({
        items: (old?.items ?? []).map((t) => (t.id === updated.id ? updated : t)),
      }));
    },
  });

  const deleteMut = useMutation({
    mutationFn: () => deleteTool(tool.name),
    onSuccess: () => {
      qc.setQueryData<{ items: ToolItem[] }>(['tools'], (old) => ({
        items: (old?.items ?? []).filter((t) => t.id !== tool.id),
      }));
      onDeleted();
    },
  });

  const risk = riskLabel(tool.risk_level);

  return (
    <div className="tool-detail">
      <div className="tool-detail-header">
        <div className="tool-detail-title">
          <h2>{tool.name}</h2>
          {tool.description && <p className="tool-desc">{tool.description}</p>}
        </div>
        <div className="tool-detail-actions">
          {tool.enabled ? (
            <button
              className="btn-action archive"
              onClick={() => toggleMut.mutate(false)}
              disabled={toggleMut.isPending}
              title="Archive (disable)"
            >
              Archive
            </button>
          ) : (
            <button
              className="btn-action restore"
              onClick={() => toggleMut.mutate(true)}
              disabled={toggleMut.isPending}
              title="Restore (enable)"
            >
              Restore
            </button>
          )}
          {confirmDelete ? (
            <span className="confirm-delete-inline">
              <span className="confirm-delete-label">Sure?</span>
              <button
                className="btn-action danger"
                onClick={() => { setConfirmDelete(false); deleteMut.mutate(); }}
                disabled={deleteMut.isPending}
              >
                {deleteMut.isPending ? 'Deleting…' : 'Confirm'}
              </button>
              <button
                className="btn-action"
                onClick={() => setConfirmDelete(false)}
                disabled={deleteMut.isPending}
              >
                Cancel
              </button>
            </span>
          ) : (
            <button
              className="btn-action danger"
              onClick={() => setConfirmDelete(true)}
              disabled={deleteMut.isPending}
            >
              Delete
            </button>
          )}
          <button className="icon-btn" onClick={onClose} aria-label="Close">✕</button>
        </div>
      </div>

      <div className="tool-detail-meta">
        <span className="meta-badge method">{tool.method}</span>
        <code className="meta-url">{tool.url}</code>
        <span className={`meta-badge ${risk.cls}`}>risk {risk.label}</span>
        <span className="meta-badge">{tool.action_type}</span>
        <span className="meta-badge">{tool.classification}</span>
        <span className={`status-indicator ${tool.enabled ? 'status-on' : 'status-off'}`}>
          {tool.enabled ? '● active' : '○ archived'}
        </span>
      </div>

      <nav className="tab-nav">
        {(['details', 'edit', 'egress', 'policies'] as Tab[]).map((t) => (
          <button key={t} className={`tab-btn ${tab === t ? 'active' : ''}`} onClick={() => setTab(t)}>
            {t.charAt(0).toUpperCase() + t.slice(1)}
          </button>
        ))}
      </nav>

      {tab === 'details' && (
        <div className="tab-content">
          <table className="table detail-table">
            <tbody>
              <tr><td>ID</td><td><code>{tool.id}</code></td></tr>
              <tr><td>Kind</td><td>{tool.kind}</td></tr>
              <tr><td>Sensitivity</td><td>{tool.sensitivity}</td></tr>
              <tr><td>Created</td><td>{new Date(tool.created_at).toLocaleString()}</td></tr>
              <tr><td>Updated</td><td>{new Date(tool.updated_at).toLocaleString()}</td></tr>
            </tbody>
          </table>
          {tool.input_schema && (
            <details open>
              <summary>Input Schema</summary>
              <pre>{JSON.stringify(tool.input_schema, null, 2)}</pre>
            </details>
          )}
        </div>
      )}

      {tab === 'edit' && (
        <EditToolForm
          tool={tool}
          onDone={() => setTab('details')}
          onCancel={() => setTab('details')}
        />
      )}
      {tab === 'egress' && <EgressTab tool={tool} />}
      {tab === 'policies' && <PoliciesTab tool={tool} />}
    </div>
  );
}

// ── Tool list ─────────────────────────────────────────────────────────────────

function ToolCard({ tool, selected, onClick }: { tool: ToolItem; selected: boolean; onClick: () => void }) {
  const risk = riskLabel(tool.risk_level);
  return (
    <button className={`tool-card ${selected ? 'selected' : ''} ${!tool.enabled ? 'disabled' : ''}`} onClick={onClick}>
      <div className="tool-card-top">
        <span className="tool-card-name">{tool.name}</span>
        <span className={`risk-badge ${risk.cls}`}>{risk.label}</span>
      </div>
      <div className="tool-card-url">
        <span className="method-tag">{tool.method}</span>
        <span className="url-text">{tool.url}</span>
      </div>
      <div className="tool-card-bottom">
        <span className="type-tag">{tool.action_type}</span>
        <span className={tool.enabled ? 'status-on' : 'status-off'}>
          {tool.enabled ? '● active' : '○ disabled'}
        </span>
      </div>
    </button>
  );
}

// ── Page root ─────────────────────────────────────────────────────────────────

export function ToolsPage() {
  const [selected, setSelected] = useState<ToolItem | null>(null);
  const [showRegister, setShowRegister] = useState(false);

  const query = useQuery({ queryKey: ['tools'], queryFn: getTools, refetchInterval: 15000, refetchOnWindowFocus: false });
  const tools: ToolItem[] = query.data?.items ?? [];

  const handleRegisterDone = () => {
    setShowRegister(false);
  };

  // Keep selected in sync after refetch
  const selectedFresh = tools.find((t) => t.id === selected?.id) ?? null;

  return (
    <div className="tools-page">
      <div className="tools-list-col">
        <div className="tools-list-header">
          <div>
            <h2>Tools</h2>
            <p className="muted">{tools.length} registered</p>
          </div>
          <button onClick={() => setShowRegister(true)}>+ Register Tool</button>
        </div>

        {query.isLoading && <p className="muted">Loading…</p>}

        {!query.isLoading && tools.length === 0 && (
          <div className="tools-empty">
            <p>No tools registered yet.</p>
            <p className="muted">
              A tool is a single endpoint of a backend service. Register one to let consumers call it through Nexus.
            </p>
            <button onClick={() => setShowRegister(true)}>Register your first tool</button>
          </div>
        )}

        <div className="tool-card-list">
          {tools.map((t) => (
            <ToolCard
              key={t.id}
              tool={t}
              selected={selectedFresh?.id === t.id}
              onClick={() => setSelected(t)}
            />
          ))}
        </div>
      </div>

      <div className="tools-detail-col">
        {selectedFresh ? (
          <ToolDetail tool={selectedFresh} onClose={() => setSelected(null)} onDeleted={() => setSelected(null)} />
        ) : (
          <div className="tools-detail-empty">
            <p>Select a tool to view details, egress rules, and policies.</p>
          </div>
        )}
      </div>

      {showRegister && (
        <RegisterToolForm
          onDone={handleRegisterDone}
          onCancel={() => setShowRegister(false)}
        />
      )}
    </div>
  );
}
