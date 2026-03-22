import { request as httpRequest } from '@devpablocristo/core-http/fetch'

const API_KEY = 'nexus-review-admin-dev-key'

type RequestOptions = Omit<RequestInit, 'headers'> & {
  headers?: Record<string, string>
}

async function request(path: string, options: RequestOptions = {}): Promise<any> {
  return httpRequest(path, {
    ...options,
    headers: {
      'X-API-Key': API_KEY,
      ...options.headers,
    },
  })
}

// Approvals
export const fetchPendingApprovals = () => request('/v1/approvals/pending')
export const approveApproval = (id, decidedBy, note = '') =>
  request(`/v1/approvals/${id}/approve`, { method: 'POST', body: JSON.stringify({ decided_by: decidedBy, note }) })
export const rejectApproval = (id, decidedBy, note = '') =>
  request(`/v1/approvals/${id}/reject`, { method: 'POST', body: JSON.stringify({ decided_by: decidedBy, note }) })

// Requests
export const fetchRequests = (params: Record<string, string | number | boolean> = {}) => {
  const q = new URLSearchParams(
    Object.entries(params).map(([key, value]) => [key, String(value)])
  ).toString()
  return request(`/v1/requests${q ? '?' + q : ''}`)
}
export const fetchRequest = (id: string) => request(`/v1/requests/${id}`)
export const simulateRequest = (data: unknown) =>
  request('/v1/requests/simulate', { method: 'POST', body: JSON.stringify(data) })
export const replaySimulate = (data: unknown) =>
  request('/v1/requests/simulate/replay', { method: 'POST', body: JSON.stringify(data) })
export const batchSimulate = (data: unknown) =>
  request('/v1/requests/simulate/batch', { method: 'POST', body: JSON.stringify(data) })
export const simulateApproval = (data: unknown) =>
  request('/v1/requests/simulate/approval', { method: 'POST', body: JSON.stringify(data) })
export const fetchReplay = (id: string) => request(`/v1/requests/${id}/replay`)
export const fetchEvidence = (id: string) => request(`/v1/requests/${id}/evidence`)
export const fetchAttestation = (id: string) => request(`/v1/requests/${id}/attestation`)

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

// Action Types
export const fetchActionTypes = () => request('/v1/action-types')
export const createActionType = (data) =>
  request('/v1/action-types', { method: 'POST', body: JSON.stringify(data) })
export const updateActionType = (id, data) =>
  request(`/v1/action-types/${id}`, { method: 'PATCH', body: JSON.stringify(data) })
export const deleteActionType = (id) =>
  request(`/v1/action-types/${id}`, { method: 'DELETE' })

// Delegations
export const fetchDelegations = () => request('/v1/delegations')
export const createDelegation = (data) =>
  request('/v1/delegations', { method: 'POST', body: JSON.stringify(data) })
export const updateDelegation = (id, data) =>
  request(`/v1/delegations/${id}`, { method: 'PATCH', body: JSON.stringify(data) })
export const deleteDelegation = (id) =>
  request(`/v1/delegations/${id}`, { method: 'DELETE' })

// Config
export const fetchConfig = () => request('/v1/config')
export const updateConfigSection = (section, data) =>
  request(`/v1/config/${section}`, { method: 'PATCH', body: JSON.stringify(data) })
export const resetConfig = () =>
  request('/v1/config/reset', { method: 'POST' })

// Companion (API key propia; rutas bajo /companion → nginx/Vite proxy)
const COMPANION_API_KEY = 'nexus-companion-admin-dev-key'

async function companionRequest(path: string, options: RequestOptions = {}): Promise<any> {
  return httpRequest(path, {
    ...options,
    headers: {
      'X-API-Key': COMPANION_API_KEY,
      ...options.headers,
    },
  })
}

export const fetchCompanionTasks = () => companionRequest('/companion/v1/tasks')
export const fetchCompanionTask = (id: string) => companionRequest(`/companion/v1/tasks/${id}`)
export const createCompanionTask = (data: unknown) =>
  companionRequest('/companion/v1/tasks', { method: 'POST', body: JSON.stringify(data) })
export const proposeCompanionTask = (id: string, data: Record<string, unknown> = {}) =>
  companionRequest(`/companion/v1/tasks/${id}/propose`, { method: 'POST', body: JSON.stringify(data) })
export const investigateCompanionTask = (id: string, note = '') =>
  companionRequest(`/companion/v1/tasks/${id}/investigate`, {
    method: 'POST',
    body: JSON.stringify({ note }),
  })
export const syncCompanionTaskFromReview = (id: string) =>
  companionRequest(`/companion/v1/tasks/${id}/sync`, { method: 'POST' })
