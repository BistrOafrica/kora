import { api } from './client'

export interface AiApproval {
  id: string
  operation_id: string
  actor_principal_id: string
  actor_principal_type: string
  tool_name: string
  state: string
  target_fingerprint: string
  argument_hash: string
  record_version: number
  requested_at: string
  expires_at?: string
  granted_at?: string
  granted_by: string
  auth_session_id: string
}

export async function fetchAiApprovals(state = 'pending_approval'): Promise<AiApproval[]> {
  return api.get<AiApproval[]>('/api/v1/ai/approvals', { state })
}

export async function grantAiApproval(id: string, grantedBy?: string): Promise<AiApproval> {
  return api.post<AiApproval>(`/api/v1/ai/approvals/${encodeURIComponent(id)}/grant`, {
    granted_by: grantedBy,
  })
}
