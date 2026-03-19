import { useState, useEffect } from 'react'
import { simulateRequest, replaySimulate, fetchPolicies, updatePolicy } from '../api'
import { t } from '../i18n'
import RiskBadge from '../components/RiskBadge'
import StatusBadge from '../components/StatusBadge'

const TEMPLATES = [
  { label: 'alert.silence (prod)', data: { requester_type: 'agent', requester_id: 'ops-bot', action_type: 'alert.silence', target_system: 'production', reason: 'maintenance' } },
  { label: 'delete (prod)', data: { requester_type: 'service', requester_id: 'cleanup-svc', action_type: 'delete', target_system: 'production', reason: 'data cleanup' } },
  { label: 'deploy (staging)', data: { requester_type: 'service', requester_id: 'deploy-svc', action_type: 'deploy.trigger', target_system: 'staging', reason: 'release v2.1' } },
  { label: 'runbook (unknown bot)', data: { requester_type: 'agent', requester_id: 'new-bot', action_type: 'runbook.execute', target_system: 'production', target_resource: 'restart-api', reason: 'high cpu' } },
]

const TABS = ['simulate', 'shadow', 'replay']

export default function Sandbox({ lang }) {
  const [tab, setTab] = useState('simulate')

  return (
    <div>
      <div className="flex items-center gap-4 mb-6">
        <h2 className="text-xl font-bold text-white">{t(lang, 'sandboxTitle')}</h2>
        <div className="flex gap-1">
          {TABS.map((id) => (
            <button key={id} onClick={() => setTab(id)}
              className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${
                tab === id ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'
              }`}>
              {t(lang, 'sandbox_' + id)}
            </button>
          ))}
        </div>
      </div>

      {tab === 'simulate' && <SimulateTab lang={lang} />}
      {tab === 'shadow' && <ShadowTab lang={lang} />}
      {tab === 'replay' && <ReplayTab lang={lang} />}
    </div>
  )
}

// --- Simulate Tab ---

function SimulateTab({ lang }) {
  const [form, setForm] = useState({ requester_type: 'agent', requester_id: '', action_type: '', target_system: '', target_resource: '', reason: '' })
  const [result, setResult] = useState(null)
  const [history, setHistory] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  const set = (k, v) => setForm({ ...form, [k]: v })

  const run = () => {
    setLoading(true)
    setError(null)
    setResult(null)
    simulateRequest(form)
      .then((r) => {
        setResult(r)
        setHistory((h) => [{ ...form, result: r, ts: new Date().toLocaleTimeString() }, ...h].slice(0, 10))
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }

  const loadTemplate = (tpl) => setForm({ ...form, ...tpl.data })
  const loadFromHistory = (h) => setForm({ requester_type: h.requester_type, requester_id: h.requester_id, action_type: h.action_type, target_system: h.target_system, target_resource: h.target_resource || '', reason: h.reason || '' })

  return (
    <div className="grid grid-cols-3 gap-6">
      {/* Formulario */}
      <div className="col-span-2 space-y-4">
        {/* Templates */}
        <div className="flex flex-wrap gap-2">
          {TEMPLATES.map((tpl, i) => (
            <button key={i} onClick={() => loadTemplate(tpl)}
              className="px-3 py-1 bg-gray-800 text-gray-400 rounded text-xs hover:bg-gray-700 hover:text-white border border-gray-700">
              {tpl.label}
            </button>
          ))}
        </div>

        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <Field label={t(lang, 'requester')}>
              <select value={form.requester_type} onChange={(e) => set('requester_type', e.target.value)}
                className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-white text-sm">
                <option value="agent">agent</option>
                <option value="service">service</option>
                <option value="human">human</option>
              </select>
            </Field>
            <Field label="Requester ID">
              <input type="text" value={form.requester_id} onChange={(e) => set('requester_id', e.target.value)}
                placeholder="ops-bot" className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-white text-sm" />
            </Field>
          </div>
          <Field label="Action Type *">
            <input type="text" value={form.action_type} onChange={(e) => set('action_type', e.target.value)}
              placeholder="alert.silence" className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-white text-sm" />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="Target System">
              <input type="text" value={form.target_system} onChange={(e) => set('target_system', e.target.value)}
                placeholder="pagerduty" className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-white text-sm" />
            </Field>
            <Field label="Target Resource">
              <input type="text" value={form.target_resource} onChange={(e) => set('target_resource', e.target.value)}
                placeholder="CPU-CRITICAL-PROD" className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-white text-sm" />
            </Field>
          </div>
          <Field label={t(lang, 'reason')}>
            <input type="text" value={form.reason} onChange={(e) => set('reason', e.target.value)}
              placeholder="maintenance window" className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-white text-sm" />
          </Field>
          <button onClick={run} disabled={loading || !form.action_type}
            className="w-full px-4 py-2 bg-blue-600 text-white rounded text-sm font-medium hover:bg-blue-500 disabled:opacity-50">
            {loading ? t(lang, 'simulating') : t(lang, 'runSimulation')}
          </button>
        </div>

        {error && <p className="text-red-400 text-sm">{error}</p>}

        {/* Resultado */}
        {result && (
          <div className="space-y-3">
            <div className={`rounded-lg p-4 border ${
              result.decision === 'allow' ? 'border-green-800 bg-green-900/20' :
              result.decision === 'deny' ? 'border-red-800 bg-red-900/20' :
              'border-yellow-800 bg-yellow-900/20'
            }`}>
              <div className="flex items-center justify-between mb-2">
                <span className="text-white font-bold text-lg uppercase">{result.decision}</span>
                <RiskBadge level={result.risk_level} />
              </div>
              <p className="text-gray-400 text-sm">{result.decision_reason}</p>
              {result.policy_matched && <p className="text-gray-500 text-xs mt-1">{t(lang, 'policyMatched')}: {result.policy_matched}</p>}
            </div>

            {result.risk_assessment && (
              <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
                <h4 className="text-xs font-semibold text-gray-400 uppercase mb-2">{t(lang, 'riskFactors')}</h4>
                <div className="space-y-1.5">
                  {(result.risk_assessment.factors || []).map((f, i) => (
                    <div key={i} className={`flex items-center gap-2 px-3 py-1.5 rounded text-xs ${f.active ? 'bg-gray-800' : 'bg-gray-900/50 opacity-50'}`}>
                      <span className={`w-2 h-2 rounded-full ${f.active ? 'bg-yellow-400' : 'bg-gray-600'}`} />
                      <span className="text-gray-300 font-mono flex-1">{f.name}</span>
                      <span className={`font-medium ${f.score > 0 ? 'text-red-400' : f.score < 0 ? 'text-green-400' : 'text-gray-500'}`}>
                        {f.score > 0 ? '+' : ''}{f.score.toFixed(2)}
                      </span>
                    </div>
                  ))}
                </div>
                <div className="flex items-center justify-between mt-3 px-3 py-2 bg-gray-800 rounded text-xs">
                  <span className="text-gray-400">
                    {t(lang, 'rawScore')}: {result.risk_assessment.raw_score?.toFixed(2)} × {result.risk_assessment.amplification?.toFixed(1)}
                  </span>
                  <span className="text-white font-bold">= {result.risk_assessment.final_score?.toFixed(2)}</span>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Historial */}
      <div>
        <h3 className="text-sm font-semibold text-gray-400 uppercase mb-3">{t(lang, 'simulateHistory')}</h3>
        {history.length === 0 && <p className="text-gray-600 text-xs">{t(lang, 'noHistory')}</p>}
        <div className="space-y-2">
          {history.map((h, i) => (
            <button key={i} onClick={() => loadFromHistory(h)}
              className="w-full text-left bg-gray-900 border border-gray-800 rounded p-2.5 hover:bg-gray-800 transition-colors">
              <div className="flex items-center justify-between">
                <span className="text-white text-xs font-mono">{h.action_type}</span>
                <StatusBadge status={h.result?.status} />
              </div>
              <span className="text-gray-600 text-xs">{h.ts} — {h.requester_id || 'anonymous'}</span>
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}

// --- Shadow Monitor Tab ---

function ShadowTab({ lang }) {
  const [policies, setPolicies] = useState([])
  const [loading, setLoading] = useState(true)

  const load = () => {
    setLoading(true)
    fetchPolicies(false)
      .then((res) => setPolicies((res.data || []).filter((p) => p.mode === 'shadow')))
      .finally(() => setLoading(false))
  }

  useEffect(() => { load() }, [])

  const promote = async (id) => {
    await updatePolicy(id, { mode: 'enforced' })
    load()
  }

  if (loading) return <p className="text-gray-400">{t(lang, 'loading')}</p>

  return (
    <div>
      {policies.length === 0 && (
        <div className="text-center py-12">
          <p className="text-gray-500">{t(lang, 'noShadowPolicies')}</p>
          <p className="text-gray-600 text-xs mt-2">{t(lang, 'noShadowPoliciesHelp')}</p>
        </div>
      )}
      <div className="space-y-3">
        {policies.map((p) => (
          <div key={p.id} className="bg-gray-900 border border-purple-800/50 rounded-lg p-4">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-3">
                <span className="text-white font-medium">{p.name}</span>
                <span className={`text-xs font-mono px-2 py-0.5 rounded ${
                  p.effect === 'deny' ? 'text-red-400 bg-gray-800' :
                  p.effect === 'allow' ? 'text-green-400 bg-gray-800' :
                  'text-yellow-400 bg-gray-800'
                }`}>{p.effect}</span>
              </div>
              <div className="flex items-center gap-3">
                <span className="text-purple-300 font-bold text-lg">{p.shadow_hits}</span>
                <span className="text-purple-400 text-xs">hits</span>
              </div>
            </div>
            <p className="text-xs font-mono text-gray-400 mb-2">{p.expression}</p>
            {p.description && <p className="text-xs text-gray-500 mb-2">{p.description}</p>}
            <div className="flex gap-2 mt-3">
              <button onClick={() => promote(p.id)}
                className="px-4 py-1.5 bg-green-800 hover:bg-green-700 text-green-200 text-xs rounded font-medium transition-colors">
                {t(lang, 'promoteToEnforced')}
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// --- Replay Simulation Tab ---

function ReplayTab({ lang }) {
  const [expression, setExpression] = useState('')
  const [effect, setEffect] = useState('deny')
  const [limit, setLimit] = useState(100)
  const [result, setResult] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  const run = () => {
    setLoading(true)
    setError(null)
    setResult(null)
    replaySimulate({ expression, effect, limit })
      .then((r) => setResult(r))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }

  return (
    <div>
      <p className="text-gray-400 text-sm mb-4">{t(lang, 'replayDesc')}</p>

      <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 space-y-3 mb-4">
        <Field label={t(lang, 'policyExpression') + ' *'}>
          <input type="text" value={expression} onChange={(e) => setExpression(e.target.value)}
            placeholder='request.action_type == "delete" && request.target_system == "production"'
            className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-white text-sm font-mono" />
        </Field>
        <div className="grid grid-cols-3 gap-3">
          <Field label={t(lang, 'policyEffect')}>
            <select value={effect} onChange={(e) => setEffect(e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-white text-sm">
              <option value="allow">allow</option>
              <option value="deny">deny</option>
              <option value="require_approval">require_approval</option>
            </select>
          </Field>
          <Field label={t(lang, 'replayLimit')}>
            <input type="number" value={limit} onChange={(e) => setLimit(parseInt(e.target.value) || 100)}
              min="1" max="500"
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-white text-sm" />
          </Field>
          <div className="flex items-end">
            <button onClick={run} disabled={loading || !expression}
              className="w-full px-4 py-1.5 bg-blue-600 text-white rounded text-sm font-medium hover:bg-blue-500 disabled:opacity-50">
              {loading ? t(lang, 'simulating') : t(lang, 'runReplay')}
            </button>
          </div>
        </div>
      </div>

      {error && <p className="text-red-400 text-sm mb-4">{error}</p>}

      {result && (
        <div className="space-y-4">
          {/* Stats */}
          <div className="grid grid-cols-5 gap-3">
            <Stat label={t(lang, 'replayEvaluated')} value={result.total_evaluated} />
            <Stat label={t(lang, 'replayMatched')} value={result.would_match} color="text-purple-400" />
            <Stat label={t(lang, 'replayWouldAllow')} value={result.would_allow} color="text-green-400" />
            <Stat label={t(lang, 'replayWouldDeny')} value={result.would_deny} color="text-red-400" />
            <Stat label={t(lang, 'replayWouldRequire')} value={result.would_require_approval} color="text-yellow-400" />
          </div>

          {/* Samples */}
          {result.samples && result.samples.length > 0 && (
            <div className="border border-gray-800 rounded-lg overflow-hidden">
              <table className="w-full text-xs">
                <thead className="bg-gray-800/50">
                  <tr className="text-gray-400 text-left">
                    <th className="px-3 py-2">Action</th>
                    <th className="px-3 py-2">Requester</th>
                    <th className="px-3 py-2">Target</th>
                    <th className="px-3 py-2">{t(lang, 'replayOriginal')}</th>
                    <th className="px-3 py-2">{t(lang, 'replayWouldBe')}</th>
                    <th className="px-3 py-2">{t(lang, 'replayChanged')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-800">
                  {result.samples.map((s, i) => (
                    <tr key={i} className={s.changed ? 'bg-yellow-900/10' : ''}>
                      <td className="px-3 py-2 text-white font-mono">{s.action_type}</td>
                      <td className="px-3 py-2 text-gray-400">{s.requester_id}</td>
                      <td className="px-3 py-2 text-gray-400">{s.target_system || '—'}</td>
                      <td className="px-3 py-2"><StatusBadge status={s.original_status} /></td>
                      <td className="px-3 py-2"><StatusBadge status={s.would_decide === 'allow' ? 'allowed' : s.would_decide === 'deny' ? 'denied' : 'pending_approval'} /></td>
                      <td className="px-3 py-2">{s.changed ? <span className="text-yellow-400">!</span> : <span className="text-gray-600">—</span>}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// --- Componentes ---

function Field({ label, children }) {
  return (
    <div>
      <label className="block text-xs text-gray-400 mb-1">{label}</label>
      {children}
    </div>
  )
}

function Stat({ label, value, color = 'text-white' }) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg p-3 text-center">
      <p className={`text-2xl font-bold ${color}`}>{value}</p>
      <p className="text-gray-500 text-xs mt-1">{label}</p>
    </div>
  )
}
