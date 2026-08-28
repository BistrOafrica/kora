import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useAuthStore } from '@/lib/auth-store'
import { Navigate, useLocation } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'

interface AuthGuardProps {
  children: ReactNode
}

const PUBLIC_PATHS = ['/workspace/auth/login', '/console/login', '/console']

function AuthLoadingScreen() {
  return (
    <div className="flex min-h-svh items-center justify-center bg-background text-foreground">
      <div className="w-full max-w-md px-6">
        <div className="rounded-2xl border bg-card px-6 py-8 shadow-sm">
          <div className="flex flex-col items-center gap-3 text-center" role="status" aria-live="polite">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" aria-hidden="true" />
            <div className="space-y-1">
              <p className="text-sm font-medium">Loading workspace</p>
              <p className="text-sm text-muted-foreground">Getting your data ready.</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export function AuthGuard({ children }: AuthGuardProps) {
  const { isAuthenticated, isLoading, checkAuth } = useAuthStore()
  const location = useLocation()
  const hasChecked = useRef(false)

  // Check public paths against the path WITHOUT site prefix.
  // E.g., /s/fieldwork/workspace/auth/login → check /workspace/auth/login.
  const pathWithoutPrefix = useMemo(() => location.pathname.replace(/^\/s\/[^/]+/, ''), [location.pathname])
  const isPublic = useMemo(() => PUBLIC_PATHS.some((p) => pathWithoutPrefix.startsWith(p)), [pathWithoutPrefix])
  const [checkingAuth, setCheckingAuth] = useState(!isPublic && !isAuthenticated)

  useEffect(() => {
    // Skip re-check if already authenticated or already checked.
    // Public pages do not need an auth request before they can render.
    if (isPublic || hasChecked.current || isAuthenticated) {
      setCheckingAuth(false)
      return
    }
    hasChecked.current = true
    void checkAuth().finally(() => setCheckingAuth(false))
  }, [checkAuth, isAuthenticated, isPublic])

  // Public paths: render children directly, no sidebar/layout.
  // Console uses its own auth system, so don't redirect console paths.
  if (isPublic) {
    const isConsolePath = pathWithoutPrefix.startsWith('/console')
    if (isAuthenticated && !isConsolePath) {
      return <Navigate to="/workspace" />
    }
    return <>{children}</>
  }

  if (checkingAuth || isLoading) {
    return <AuthLoadingScreen />
  }

  // Protected paths: require auth.
  if (!isAuthenticated) {
    return <Navigate to="/workspace/auth/login" />
  }

  return <>{children}</>
}
