import { api } from './client'
import type { CurrentUser, LoginRequest, AuthProvider, MagicLinkRequest, MagicLinkVerifyRequest } from '@/types/api'

export async function login(req: LoginRequest): Promise<CurrentUser> {
  return api.post<CurrentUser>('/api/auth/login', req)
}

export async function logout(): Promise<void> {
  return api.post<void>('/api/auth/logout')
}

export async function fetchMe(): Promise<CurrentUser> {
  return api.get<CurrentUser>('/api/auth/me')
}

export async function fetchProviders(): Promise<AuthProvider[]> {
  const data = await api.get<{ providers: AuthProvider[] }>('/api/auth/providers')
  return data.providers
}

export async function requestMagicLink(req: MagicLinkRequest): Promise<{ message: string }> {
  return api.post<{ message: string }>('/api/auth/magic-link/request', req)
}

export async function requestEmailVerification(req: MagicLinkRequest): Promise<{ message: string }> {
  return api.post<{ message: string }>('/api/auth/magic-link/request', req)
}

export async function verifyMagicLink(req: MagicLinkVerifyRequest): Promise<CurrentUser> {
  return api.post<CurrentUser>('/api/auth/magic-link/verify', req)
}

export interface MagicLinkItem {
  id: string
  email: string
  created_at: string
  expires_at: string
  used_at?: string | null
  revoked_at?: string | null
}

export async function listMagicLinks(): Promise<{ links: MagicLinkItem[] }> {
  return api.get<{ links: MagicLinkItem[] }>('/api/auth/magic-links')
}

export async function revokeMagicLink(id: string): Promise<{ message: string }> {
  return api.post<{ message: string }>(`/api/auth/magic-links/${id}/revoke`)
}

export async function revokeAllMagicLinks(): Promise<{ message: string }> {
  return api.post<{ message: string }>('/api/auth/magic-links/revoke-all')
}
