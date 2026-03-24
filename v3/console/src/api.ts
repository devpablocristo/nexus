import { request as httpRequest } from '@devpablocristo/core-http/fetch'
import { registerTokenProvider } from '@devpablocristo/core-authn/http/fetch'
import { REVIEW_API_KEY, COMPANION_API_KEY, clerkEnabled } from './auth'

// Token provider: se registra desde AuthTokenBridge (Clerk) o queda null (API key)
let getClerkToken: (() => Promise<string | null>) | null = null

// Registrar en core-authn para que httpRequest lo use automáticamente
if (clerkEnabled) {
  registerTokenProvider(async () => {
    if (getClerkToken) {
      return getClerkToken()
    }
    return null
  })
}

// Llamado desde AuthTokenBridge cuando Clerk está listo
export function setClerkTokenGetter(getter: () => Promise<string | null>) {
  getClerkToken = getter
}

type RequestOptions = Omit<RequestInit, 'headers'> & {
  headers?: Record<string, string>
}

async function request(path: string, options: RequestOptions = {}): Promise<any> {
  const headers: Record<string, string> = { ...options.headers }

  // Si Clerk está habilitado y hay token, core-authn lo inyecta automáticamente.
  // Fallback a API key para desarrollo local sin Clerk.
  if (!clerkEnabled) {
    headers['X-API-Key'] = REVIEW_API_KEY
  }

  return httpRequest(path, { ...options, headers })
}

async function companionRequest(path: string, options: RequestOptions = {}): Promise<any> {
  const headers: Record<string, string> = { ...options.headers }

  if (!clerkEnabled) {
    headers['X-API-Key'] = COMPANION_API_KEY
  }

  return httpRequest(path, { ...options, headers })
}

// Approvals
export const fetchPendingApprovals = () => request('/v1/approvals/pending')
export const approveApproval = (id: string, decidedBy: string, note = '') =>
  request(`/v1/approvals/${id}/approve`, { method: 'POST', body: JSON.stringify({ decided_by: decidedBy, note }) })
export const rejectApproval = (id: string, decidedBy: string, note = '') =>
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
export const acceptProposal = (id: string, decidedBy: string) =>
  request(`/v1/learning/proposals/${id}/accept`, { method: 'POST', body: JSON.stringify({ decided_by: decidedBy }) })
export const dismissProposal = (id: string, decidedBy: string) =>
  request(`/v1/learning/proposals/${id}/dismiss`, { method: 'POST', body: JSON.stringify({ decided_by: decidedBy }) })
export const triggerAnalyze = () => request('/v1/learning/analyze', { method: 'POST' })

// Policies
export const fetchPolicies = (archived = false) =>
  request(`/v1/policies${archived ? '?archived=true' : ''}`)
export const fetchPolicy = (id: string) => request(`/v1/policies/${id}`)
export const createPolicy = (data: unknown) =>
  request('/v1/policies', { method: 'POST', body: JSON.stringify(data) })
export const updatePolicy = (id: string, data: unknown) =>
  request(`/v1/policies/${id}`, { method: 'PATCH', body: JSON.stringify(data) })
export const deletePolicy = (id: string) =>
  request(`/v1/policies/${id}`, { method: 'DELETE' })
export const archivePolicy = (id: string) =>
  request(`/v1/policies/${id}/archive`, { method: 'POST' })
export const restorePolicy = (id: string) =>
  request(`/v1/policies/${id}/restore`, { method: 'POST' })

// Dashboard
export const fetchMetrics = (period = '7d') => request(`/v1/metrics/summary?period=${period}`)

// Action Types
export const fetchActionTypes = () => request('/v1/action-types')
export const createActionType = (data: unknown) =>
  request('/v1/action-types', { method: 'POST', body: JSON.stringify(data) })
export const updateActionType = (id: string, data: unknown) =>
  request(`/v1/action-types/${id}`, { method: 'PATCH', body: JSON.stringify(data) })
export const deleteActionType = (id: string) =>
  request(`/v1/action-types/${id}`, { method: 'DELETE' })

// Delegations
export const fetchDelegations = () => request('/v1/delegations')
export const createDelegation = (data: unknown) =>
  request('/v1/delegations', { method: 'POST', body: JSON.stringify(data) })
export const updateDelegation = (id: string, data: unknown) =>
  request(`/v1/delegations/${id}`, { method: 'PATCH', body: JSON.stringify(data) })
export const deleteDelegation = (id: string) =>
  request(`/v1/delegations/${id}`, { method: 'DELETE' })

// Config
export const fetchConfig = () => request('/v1/config')
export const updateConfigSection = (section: string, data: unknown) =>
  request(`/v1/config/${section}`, { method: 'PATCH', body: JSON.stringify(data) })
export const resetConfig = () =>
  request('/v1/config/reset', { method: 'POST' })

// Companion — Tasks
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

// Companion — Chat (interfaz conversacional del suscriptor)
export const sendChatMessage = (message: string, taskId?: string, channel = 'console') =>
  companionRequest('/companion/v1/chat', {
    method: 'POST',
    body: JSON.stringify({ message, task_id: taskId || undefined, channel }),
  })
