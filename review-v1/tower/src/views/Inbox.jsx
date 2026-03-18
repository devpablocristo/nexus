import { useState, useEffect } from 'react'
import { fetchPendingApprovals, fetchRequest, approveApproval, rejectApproval } from '../api'
import { t } from '../i18n'
import RiskBadge from '../components/RiskBadge'

function timeRemaining(expiresAt, lang) {
  const diff = new Date(expiresAt) - new Date()
  if (diff <= 0) return t(lang, 'expired')
  const min = Math.floor(diff / 60000)
  if (min < 60) return `${min}min ${t(lang, 'timeLeft')}`
  return `${Math.floor(min / 60)}h ${min % 60}min ${t(lang, 'timeLeft')}`
}

export default function Inbox({ lang, onViewReplay }) {
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const load = async () => {
    try {
      setLoading(true)
      const res = await fetchPendingApprovals()
      const approvals = res.data || []
      const enriched = await Promise.all(
        approvals.map(async (a) => {
          try {
            const req = await fetchRequest(a.request_id)
            return { approval: a, request: req }
          } catch {
            return { approval: a, request: null }
          }
        })
      )
      setItems(enriched)
      setError(null)
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])
  useEffect(() => {
    const id = setInterval(load, 10000)
    return () => clearInterval(id)
  }, [])

  const handleAction = async (approvalId, action) => {
    try {
      if (action === 'approve') {
        await approveApproval(approvalId, 'demo-user', '')
      } else {
        await rejectApproval(approvalId, 'demo-user', '')
      }
      setItems((prev) => prev.filter((i) => i.approval.id !== approvalId))
    } catch (e) {
      setError(e.message)
    }
  }

  if (loading && items.length === 0) return <p className="text-gray-500">{t(lang, 'loading')}</p>
  if (error) return <p className="text-red-400">{error}</p>

  return (
    <div>
      <h2 className="text-xl font-bold mb-4">
        {t(lang, 'approvalInbox')} <span className="text-gray-500 text-sm font-normal ml-2">{items.length} {t(lang, 'pending')}</span>
      </h2>
      {items.length === 0 && <p className="text-gray-500">{t(lang, 'noPendingApprovals')}</p>}
      <div className="space-y-3">
        {items.map(({ approval, request }) => (
          <div key={approval.id} className="bg-gray-900 border border-gray-800 rounded-lg p-4">
            <div className="flex items-center gap-3 mb-2">
              <RiskBadge level={request?.risk_level || 'low'} />
              <span className="font-medium">{request?.action_type || '—'}</span>
              <span className="text-gray-500">{request?.target_resource || ''}</span>
              <span className="text-gray-600 text-sm">{request?.target_system || ''}</span>
              <span className="ml-auto text-gray-500 text-xs">{timeRemaining(approval.expires_at, lang)}</span>
            </div>
            {request?.ai_summary && (
              <p className="text-gray-300 text-sm mb-2 line-clamp-2">{request.ai_summary}</p>
            )}
            <div className="flex items-center gap-2 text-xs text-gray-500 mb-3">
              <span>{request?.requester_id || '—'} ({request?.requester_type || ''})</span>
              {request?.decision_reason && <span>| {request.decision_reason}</span>}
            </div>
            <div className="flex gap-2">
              <button onClick={() => handleAction(approval.id, 'approve')}
                className="px-3 py-1.5 bg-green-600 hover:bg-green-500 text-white text-sm rounded font-medium transition-colors">
                {t(lang, 'approve')}
              </button>
              <button onClick={() => handleAction(approval.id, 'reject')}
                className="px-3 py-1.5 bg-red-600 hover:bg-red-500 text-white text-sm rounded font-medium transition-colors">
                {t(lang, 'reject')}
              </button>
              {request && (
                <button onClick={() => onViewReplay(request.id)}
                  className="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 text-gray-300 text-sm rounded font-medium transition-colors">
                  {t(lang, 'details')}
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
