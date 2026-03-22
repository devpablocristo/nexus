import { useState, useEffect, useCallback } from 'react'
import {
  fetchCompanionTasks,
  fetchCompanionTask,
  createCompanionTask,
  proposeCompanionTask,
  investigateCompanionTask,
  syncCompanionTaskFromReview,
} from '../api'
import { t } from '../i18n'

type TaskRow = {
  id: string
  title: string
  status: string
  updated_at: string
}

type TaskDetail = {
  task: Record<string, unknown>
  messages: Array<Record<string, unknown>>
  actions: Array<Record<string, unknown>>
  artifacts: Array<Record<string, unknown>>
  linked_review_requests: Array<{
    action_id: string
    request?: Record<string, unknown>
  }>
}

export default function Tasks({ lang }: { lang: string }) {
  const [list, setList] = useState<TaskRow[]>([])
  const [detail, setDetail] = useState<TaskDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [msg, setMsg] = useState<{ type: string; text: string } | null>(null)
  const [form, setForm] = useState({ title: '', goal: '', created_by: '' })
  const [proposeNote, setProposeNote] = useState('')
  const [busy, setBusy] = useState(false)

  const loadList = useCallback(() => {
    return fetchCompanionTasks()
      .then((r: { data?: TaskRow[] }) => setList(r.data || []))
      .catch((e: Error) => setMsg({ type: 'err', text: e.message }))
  }, [])

  const loadDetail = useCallback((id: string) => {
    return fetchCompanionTask(id)
      .then((d: TaskDetail) => setDetail(d))
      .catch((e: Error) => setMsg({ type: 'err', text: e.message }))
  }, [])

  useEffect(() => {
    setLoading(true)
    loadList().finally(() => setLoading(false))
  }, [loadList])

  useEffect(() => {
    if (!detail?.task?.id) return
    const id = String(detail.task.id)
    const h = setInterval(() => loadDetail(id), 4000)
    return () => clearInterval(h)
  }, [detail?.task?.id, loadDetail])

  const openTask = (id: string) => {
    setMsg(null)
    loadDetail(id)
  }

  const create = () => {
    if (!form.title.trim()) return
    setBusy(true)
    setMsg(null)
    createCompanionTask({
      title: form.title.trim(),
      goal: form.goal.trim(),
      created_by: form.created_by.trim(),
    })
      .then(() => {
        setForm({ title: '', goal: '', created_by: '' })
        return loadList()
      })
      .catch((e: Error) => setMsg({ type: 'err', text: e.message }))
      .finally(() => setBusy(false))
  }

  const propose = () => {
    if (!detail?.task?.id) return
    setBusy(true)
    setMsg(null)
    proposeCompanionTask(String(detail.task.id), { note: proposeNote })
      .then(() => {
        setProposeNote('')
        return loadDetail(String(detail.task.id))
      })
      .catch((e: Error) => setMsg({ type: 'err', text: e.message }))
      .finally(() => setBusy(false))
  }

  const investigate = () => {
    if (!detail?.task?.id) return
    setBusy(true)
    setMsg(null)
    investigateCompanionTask(String(detail.task.id), '')
      .then(() => loadDetail(String(detail.task.id)))
      .catch((e: Error) => setMsg({ type: 'err', text: e.message }))
      .finally(() => setBusy(false))
  }

  const syncFromReview = () => {
    if (!detail?.task?.id) return
    setBusy(true)
    setMsg(null)
    syncCompanionTaskFromReview(String(detail.task.id))
      .then(() => loadDetail(String(detail.task.id)))
      .catch((e: Error) => setMsg({ type: 'err', text: e.message }))
      .finally(() => setBusy(false))
  }

  const timeline = (): Array<{ kind: string; at: string; label: string; body: string }> => {
    if (!detail) return []
    const rows: Array<{ kind: string; at: string; label: string; body: string }> = []
    for (const m of detail.messages || []) {
      rows.push({
        kind: 'message',
        at: String(m.created_at),
        label: `${m.author_type || 'user'}:${m.author_id || ''}`,
        body: String(m.body || ''),
      })
    }
    for (const a of detail.actions || []) {
      const rid = a.review_request_id ? ` → ${a.review_request_id}` : ''
      const err = a.error_message ? ` (${String(a.error_message)})` : ''
      rows.push({
        kind: 'action',
        at: String(a.created_at),
        label: String(a.action_type || 'action'),
        body: `${rid}${err}`,
      })
    }
    rows.sort((x, y) => new Date(x.at).getTime() - new Date(y.at).getTime())
    return rows
  }

  if (loading) return <p className="text-gray-400">{t(lang, 'loading')}</p>

  return (
    <div className="flex flex-col lg:flex-row gap-8">
      <div className="lg:w-1/3 space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-bold text-white">{t(lang, 'tasksTitle')}</h2>
          <button
            type="button"
            onClick={() => loadList()}
            className="text-sm text-gray-400 hover:text-white"
          >
            {t(lang, 'refresh')}
          </button>
        </div>
        <p className="text-sm text-gray-500">{t(lang, 'tasksSubtitle')}</p>

        <div className="rounded-lg border border-gray-800 bg-gray-900/50 p-4 space-y-2">
          <p className="text-sm font-medium text-gray-300">{t(lang, 'createTask')}</p>
          <input
            className="w-full rounded bg-gray-800 border border-gray-700 px-2 py-1 text-sm text-white"
            placeholder={t(lang, 'taskTitle')}
            value={form.title}
            onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))}
          />
          <input
            className="w-full rounded bg-gray-800 border border-gray-700 px-2 py-1 text-sm text-white"
            placeholder={t(lang, 'taskGoal')}
            value={form.goal}
            onChange={(e) => setForm((f) => ({ ...f, goal: e.target.value }))}
          />
          <input
            className="w-full rounded bg-gray-800 border border-gray-700 px-2 py-1 text-sm text-white"
            placeholder={t(lang, 'taskCreatedBy')}
            value={form.created_by}
            onChange={(e) => setForm((f) => ({ ...f, created_by: e.target.value }))}
          />
          <button
            type="button"
            disabled={busy}
            onClick={create}
            className="w-full py-1.5 rounded bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-500 disabled:opacity-50"
          >
            {t(lang, 'createTask')}
          </button>
        </div>

        {msg && <p className="text-sm text-red-400">{msg.text}</p>}

        <ul className="space-y-1">
          {list.length === 0 && <li className="text-gray-500 text-sm">{t(lang, 'noTasks')}</li>}
          {list.map((row) => (
            <li key={row.id}>
              <button
                type="button"
                onClick={() => openTask(row.id)}
                className={`w-full text-left px-3 py-2 rounded text-sm border ${
                  detail?.task?.id === row.id
                    ? 'border-indigo-500 bg-indigo-950/40 text-white'
                    : 'border-gray-800 bg-gray-900/30 text-gray-300 hover:border-gray-600'
                }`}
              >
                <div className="font-medium">{row.title}</div>
                <div className="text-xs text-gray-500">
                  {row.status} · {row.updated_at}
                </div>
              </button>
            </li>
          ))}
        </ul>
      </div>

      <div className="flex-1 min-h-[320px]">
        {!detail && (
          <p className="text-gray-500 text-sm">{t(lang, 'details')} — select a task</p>
        )}
        {detail && (
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => setDetail(null)}
                className="text-sm text-gray-400 hover:text-white"
              >
                {t(lang, 'backToList')}
              </button>
            </div>
            <div className="rounded-lg border border-gray-800 p-4 bg-gray-900/40">
              <h3 className="text-lg font-semibold text-white mb-2">{String(detail.task.title)}</h3>
              <dl className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
                <div>
                  <dt className="text-gray-500">{t(lang, 'taskStatus')}</dt>
                  <dd className="text-gray-200">{String(detail.task.status)}</dd>
                </div>
                <div>
                  <dt className="text-gray-500">{t(lang, 'taskCreatedBy')}</dt>
                  <dd className="text-gray-200">{String(detail.task.created_by || '—')}</dd>
                </div>
                <div className="sm:col-span-2">
                  <dt className="text-gray-500">{t(lang, 'taskGoal')}</dt>
                  <dd className="text-gray-200">{String(detail.task.goal || '—')}</dd>
                </div>
              </dl>
              <div className="flex flex-wrap gap-2 mt-4">
                <button
                  type="button"
                  disabled={busy}
                  onClick={investigate}
                  className="px-3 py-1.5 rounded bg-gray-700 text-sm text-white hover:bg-gray-600 disabled:opacity-50"
                >
                  {t(lang, 'investigateTask')}
                </button>
                <input
                  className="flex-1 min-w-[120px] rounded bg-gray-800 border border-gray-700 px-2 py-1 text-sm text-white"
                  placeholder={t(lang, 'proposeNote')}
                  value={proposeNote}
                  onChange={(e) => setProposeNote(e.target.value)}
                />
                <button
                  type="button"
                  disabled={busy}
                  onClick={propose}
                  className="px-3 py-1.5 rounded bg-amber-700 text-sm text-white hover:bg-amber-600 disabled:opacity-50"
                >
                  {busy ? t(lang, 'proposeRunning') : t(lang, 'proposeToReview')}
                </button>
                {String(detail.task.status) === 'waiting_for_approval' && (
                  <button
                    type="button"
                    disabled={busy}
                    onClick={syncFromReview}
                    className="px-3 py-1.5 rounded bg-teal-800 text-sm text-white hover:bg-teal-700 disabled:opacity-50"
                  >
                    {t(lang, 'syncFromReview')}
                  </button>
                )}
              </div>
            </div>

            {(detail.linked_review_requests || []).length > 0 && (
              <div className="rounded-lg border border-gray-800 p-4">
                <h4 className="text-sm font-medium text-gray-300 mb-2">{t(lang, 'linkedReview')}</h4>
                <ul className="space-y-2 text-sm">
                  {detail.linked_review_requests.map((lr) => (
                    <li key={lr.action_id} className="text-gray-400 border-l-2 border-indigo-600 pl-2">
                      {lr.request ? (
                        <>
                          <span className="text-white font-mono text-xs">{String(lr.request.id)}</span>
                          <span className="mx-2">·</span>
                          {String(lr.request.status)} · {String(lr.request.decision)} ·{' '}
                          {String(lr.request.risk_level)}
                        </>
                      ) : (
                        <span className="text-gray-500">request unavailable</span>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
            )}

            <div className="rounded-lg border border-gray-800 p-4">
              <h4 className="text-sm font-medium text-gray-300 mb-2">{t(lang, 'taskTimeline')}</h4>
              <ul className="space-y-2 text-sm">
                {timeline().map((row, i) => (
                  <li key={i} className="border-l border-gray-700 pl-2">
                    <span className="text-gray-500 text-xs">{row.at}</span>
                    <span className="text-indigo-400 text-xs ml-2">[{row.kind}]</span>
                    <span className="text-gray-400 text-xs ml-2">{row.label}</span>
                    <div className="text-gray-200">{row.body}</div>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
