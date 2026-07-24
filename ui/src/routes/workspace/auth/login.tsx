import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { LogoMark } from '@/components/ui/LogoMark'
import { useAuthStore } from '@/lib/auth-store'
import { LoginForm } from '@/components/login-form'

function detectSiteLabel(): string {
  if (typeof window === 'undefined') return 'your workspace'
  const pathMatch = window.location.pathname.match(/^\/s\/([^/]+)/)
  if (pathMatch?.[1]) return decodeURIComponent(pathMatch[1])
  const cookieMatch = document.cookie.match(/(?:^|;\s*)kora_site=([^;]+)/)
  if (cookieMatch?.[1]) return decodeURIComponent(cookieMatch[1])
  const host = window.location.hostname
  if (!host || host === 'localhost') return 'your workspace'
  const first = host.split('.')[0]
  if (!first || first === 'app') return 'your workspace'
  return first
}

export default function LoginPage() {
  const [isVerifyingToken, setIsVerifyingToken] = useState(false)
  const [magicTokenHandled, setMagicTokenHandled] = useState(false)
  const [siteLabel] = useState(() => detectSiteLabel())
  const { verifyMagicLink } = useAuthStore()
  const navigate = useNavigate()

  const magicToken = useMemo(() => {
    if (typeof window === 'undefined') return ''
    return new URLSearchParams(window.location.search).get('magic_token') || ''
  }, [])

  useEffect(() => {
    if (!magicToken || magicTokenHandled) return
    setMagicTokenHandled(true)
    setIsVerifyingToken(true)
    void (async () => {
      try {
        await verifyMagicLink(magicToken)
        navigate({ to: '/workspace' })
      } catch {
        // Token invalid, stay on login
      } finally {
        setIsVerifyingToken(false)
      }
    })()
  }, [magicToken, magicTokenHandled, navigate, verifyMagicLink])

  // Token verification loading state
  if (isVerifyingToken) {
    return (
      <div className="flex min-h-svh items-center justify-center bg-background px-4">
        <Card className="w-full max-w-md">
          <CardContent className="flex flex-col items-center gap-4 py-12 text-center">
            <div className="rounded-full border bg-muted p-3">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
            <div className="space-y-1">
              <h1 className="text-xl font-semibold tracking-tight">Verifying sign-in link</h1>
              <p className="text-sm text-muted-foreground">
                Connecting you to {siteLabel}. This should only take a moment.
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      {/* Left: Form */}
      <div className="flex flex-col gap-4 p-6 md:p-10">
        <div className="flex justify-center gap-2 md:justify-start">
          <a href="/workspace" className="flex items-center gap-2 font-medium">
            <div className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <LogoMark size={18} />
            </div>
            <span className="text-sm font-semibold">Kora</span>
            <span className="hidden text-sm text-muted-foreground sm:inline">· {siteLabel}</span>
          </a>
        </div>
        <div className="flex flex-1 items-center justify-center">
          <div className="w-full max-w-sm">
            <LoginForm
              siteLabel={siteLabel}
              onSuccess={() => navigate({ to: '/workspace' })}
            />
          </div>
        </div>
      </div>

      {/* Right: Brand Panel */}
      <div className="relative hidden bg-muted lg:block">
        <div className="absolute inset-0 bg-gradient-to-br from-emerald-950/90 via-emerald-900/70 to-slate-950/90" />
        <div className="absolute inset-0 flex flex-col justify-between p-12 text-white">
          <div className="flex items-center gap-3">
            <div className="rounded-2xl border border-white/10 bg-white/5 p-2">
              <LogoMark size={28} />
            </div>
            <div>
              <div className="text-sm font-medium text-slate-300">Kora</div>
              <div className="text-xs text-slate-400">{siteLabel}</div>
            </div>
          </div>
          <div className="space-y-6">
            <div className="space-y-0.5">
              <p className="text-5xl font-semibold tracking-tight text-white">One place for<br />all your work.</p>
            </div>
            <div className="space-y-4">
              <div className="flex items-center gap-3 text-sm text-slate-300">
                <svg className="h-5 w-5 shrink-0 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                <span>No spreadsheets. No paper. Just what you need.</span>
              </div>
              <div className="flex items-center gap-3 text-sm text-slate-300">
                <svg className="h-5 w-5 shrink-0 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                <span>Your whole team sees the same numbers. No confusion.</span>
              </div>
              <div className="flex items-center gap-3 text-sm text-slate-300">
                <svg className="h-5 w-5 shrink-0 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                <span>Works on your phone, tablet, or computer. Nothing to install.</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
