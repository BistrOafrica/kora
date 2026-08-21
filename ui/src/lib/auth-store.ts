import { create } from 'zustand'
import type { AuthProvider, CurrentUser } from '@/types/api'
import { sitePath } from './basepath'
import * as authApi from './api/auth'

interface AuthState {
  user: CurrentUser | null
  isAuthenticated: boolean
  isLoading: boolean
  error: string | null
  errorType: string | null
  providers: AuthProvider[]

  login: (email: string, password: string) => Promise<void>
  fetchProviders: () => Promise<AuthProvider[]>
  requestMagicLink: (email: string) => Promise<void>
  requestEmailVerification: (email: string) => Promise<void>
  verifyMagicLink: (token: string) => Promise<void>
  logout: () => Promise<void>
  checkAuth: () => Promise<boolean>
  clearError: () => void
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  isAuthenticated: false,
  // Route authentication is tracked by AuthGuard. This flag is reserved for
  // sign-in, sign-out, and other user-triggered auth actions so public forms
  // are usable immediately.
  isLoading: false,
  error: null,
  errorType: null,
  providers: [],

  login: async (email, password) => {
    set({ isLoading: true, error: null, errorType: null })
    try {
      const user = await authApi.login({ email, password })
      set({ user, isAuthenticated: true, isLoading: false })
    } catch (err: any) {
      set({ isLoading: false, error: err.message || 'Login failed', errorType: err.type || null })
      throw err
    }
  },

  fetchProviders: async () => {
    try {
      const providers = await authApi.fetchProviders()
      set({ providers })
      return providers
    } catch (err: any) {
      const fallback = [{ name: 'password', label: 'Email & Password' }, { name: 'magic_link', label: 'Magic Link' }]
      set({ providers: fallback })
      return fallback
    }
  },

  requestMagicLink: async (email) => {
    set({ isLoading: true, error: null, errorType: null })
    try {
      await authApi.requestMagicLink({ email })
      set({ isLoading: false })
    } catch (err: any) {
      set({ isLoading: false, error: err.message || 'Failed to request magic link', errorType: err.type || null })
      throw err
    }
  },

  requestEmailVerification: async (email) => {
    set({ isLoading: true, error: null, errorType: null })
    try {
      await authApi.requestEmailVerification({ email })
      set({ isLoading: false })
    } catch (err: any) {
      set({ isLoading: false, error: err.message || 'Failed to request verification email', errorType: err.type || null })
      throw err
    }
  },

  verifyMagicLink: async (token) => {
    set({ isLoading: true, error: null, errorType: null })
    try {
      const user = await authApi.verifyMagicLink({ token })
      set({ user, isAuthenticated: true, isLoading: false })
    } catch (err: any) {
      set({ isLoading: false, error: err.message || 'Failed to verify magic link', errorType: err.type || null })
      throw err
    }
  },

  logout: async () => {
    try {
      await authApi.logout()
    } catch {
      // Ignore logout errors.
    }
    set({ user: null, isAuthenticated: false, isLoading: false })
    // Use router navigation instead of full page reload.
    window.location.href = sitePath('/workspace/auth/login')
  },

  checkAuth: async () => {
    if (get().isAuthenticated) return true
    set({ isLoading: true })
    try {
      const user = await authApi.fetchMe()
      set({ user, isAuthenticated: true, isLoading: false })
      return true
    } catch {
      set({ user: null, isAuthenticated: false, isLoading: false })
      return false
    }
  },

  clearError: () => set({ error: null, errorType: null }),
}))
