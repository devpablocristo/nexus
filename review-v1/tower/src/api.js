const API_KEY = 'nexus-decision-plane-admin-dev-key'

async function request(path, options = {}) {
  const res = await fetch(path, {
    ...options,
    headers: {
      'X-API-Key': API_KEY,
      'Content-Type': 'application/json',
      ...options.headers,
    },
  })
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    let msg = res.statusText
    try {
      const body = JSON.parse(text)
      if (body.error && typeof body.error === 'object') {
        msg = body.error.message || body.error.code || res.statusText
      } else {
        msg = body.message || body.error || res.statusText
      }
    } catch {
      msg = text || res.statusText
    }
    throw new Error(msg)
  }
  if (res.status === 204) return null
  return res.json()
}

// Approvals
export const fetchPendingApprovals = () => request('/v1/approvals/pending')
export const approveApproval = (id, decidedBy, note = '') =>
  request(`/v1/approvals/${id}/approve`, { method: 'POST', body: JSON.stringify({ decided_by: decidedBy, note }) })
export const rejectApproval = (id, decidedBy, note = '') =>
  request(`/v1/approvals/${id}/reject`, { method: 'POST', body: JSON.stringify({ decided_by: decidedBy, note }) })

// Requests
export const fetchRequests = (params = {}) => {
  const q = new URLSearchParams(params).toString()
  return request(`/v1/requests${q ? '?' + q : ''}`)
}
export const fetchRequest = (id) => request(`/v1/requests/${id}`)
export const fetchReplay = (id) => request(`/v1/requests/${id}/replay`)

// Learning
export const fetchProposals = () => request('/v1/learning/proposals')
export const acceptProposal = (id, decidedBy) =>
  request(`/v1/learning/proposals/${id}/accept`, { method: 'POST', body: JSON.stringify({ decided_by: decidedBy }) })
export const dismissProposal = (id, decidedBy) =>
  request(`/v1/learning/proposals/${id}/dismiss`, { method: 'POST', body: JSON.stringify({ decided_by: decidedBy }) })
export const triggerAnalyze = () => request('/v1/learning/analyze', { method: 'POST' })

// Policies
export const fetchPolicies = (archived = false) =>
  request(`/v1/policies${archived ? '?archived=true' : ''}`)
export const fetchPolicy = (id) => request(`/v1/policies/${id}`)
export const createPolicy = (data) =>
  request('/v1/policies', { method: 'POST', body: JSON.stringify(data) })
export const updatePolicy = (id, data) =>
  request(`/v1/policies/${id}`, { method: 'PATCH', body: JSON.stringify(data) })
export const deletePolicy = (id) =>
  request(`/v1/policies/${id}`, { method: 'DELETE' })
export const archivePolicy = (id) =>
  request(`/v1/policies/${id}/archive`, { method: 'POST' })
export const restorePolicy = (id) =>
  request(`/v1/policies/${id}/restore`, { method: 'POST' })

// Dashboard
export const fetchMetrics = (period = '7d') => request(`/v1/metrics/summary?period=${period}`)
