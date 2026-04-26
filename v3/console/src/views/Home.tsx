import { useEffect, useState } from 'react'
import { fetchPendingApprovals, fetchRequest } from '../api'
import { t } from '../i18n'
import RiskBadge from '../components/RiskBadge'

type Approval = {
  id: string
  request_id: string
  risk_level?: string
  required_approvals?: number
  current_approvals?: number
  expires_at?: string
}

type ReviewRequest = {
  id: string
  action_type: string
  target_system?: string
  status: string
  risk_level?: string
  requester_id?: string
  created_at: string
}

export default function Home({
  lang,
  onViewReplay,
  onViewInbox,
}: {
  lang: string
  onViewReplay: (requestId: string) => void
  onViewInbox: () => void
}) {
  const [approvals, setApprovals] = useState<Array<{ approval: Approval; request: ReviewRequest | null }>>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let alive = true
    const load = async () => {
      try {
        setLoading(true)
        const res = await fetchPendingApprovals()
        const items = (res.data || []).slice(0, 5) as Approval[]
        const enriched = await Promise.all(
          items.map(async (a) => {
            try {
              const r = (await fetchRequest(a.request_id)) as ReviewRequest
              return { approval: a, request: r }
            } catch {
              return { approval: a, request: null }
            }
          }),
        )
        if (alive) {
          setApprovals(enriched)
          setError(null)
        }
      } catch (e: any) {
        if (alive) setError(e?.message || 'load failed')
      } finally {
        if (alive) setLoading(false)
      }
    }
    load()
    return () => {
      alive = false
    }
  }, [])

  return (
    <div className="space-y-6">
      <header className="border-b border-gray-800 pb-4">
        <h2 className="text-2xl font-bold text-white">{t(lang, 'homeTitle')}</h2>
        <p className="text-sm text-gray-400 mt-1">{t(lang, 'homeSubtitle')}</p>
      </header>

      <section className="bg-gray-900 border border-gray-800 rounded-lg p-5">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-lg font-semibold text-white">{t(lang, 'pendingApprovals')}</h3>
          <button
            onClick={onViewInbox}
            className="text-xs px-3 py-1 rounded bg-gray-800 text-gray-300 hover:bg-gray-700 hover:text-white"
          >
            {t(lang, 'goToInbox')}
          </button>
        </div>

        {loading && <p className="text-sm text-gray-500">…</p>}
        {error && <p className="text-sm text-red-400">{error}</p>}
        {!loading && !error && approvals.length === 0 && (
          <p className="text-sm text-gray-500">{t(lang, 'noPendingApprovals')}</p>
        )}

        <ul className="space-y-2">
          {approvals.map(({ approval, request }) => (
            <li
              key={approval.id}
              className="flex items-center justify-between gap-3 p-3 rounded bg-gray-800/40 border border-gray-800"
            >
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 text-sm text-white">
                  <span className="font-medium truncate">
                    {request?.action_type || approval.request_id}
                  </span>
                  {approval.risk_level && <RiskBadge level={approval.risk_level} />}
                </div>
                {request && (
                  <div className="text-xs text-gray-500 mt-0.5 truncate">
                    {request.target_system} · {request.requester_id || '—'}
                  </div>
                )}
              </div>
              <button
                onClick={() => onViewReplay(approval.request_id)}
                className="text-xs px-2 py-1 rounded bg-gray-700 text-gray-200 hover:bg-gray-600"
              >
                {t(lang, 'viewReplay')}
              </button>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}
