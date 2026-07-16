import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { AlertCircle, ArrowLeft, CheckCircle2, KeyRound, Loader2, Mail, ShieldCheck, Sparkles } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/ui/password-input'
import { Label } from '@/components/ui/label'
import { LogoMark } from '@/components/ui/LogoMark'
import { useAuthStore } from '@/lib/auth-store'
import type { AuthProvider } from '@/types/api'

type AuthMode = 'password' | 'magic'

const defaultProviders: AuthProvider[] = [
  { name: 'password', label: 'Email & Password' },
  { name: 'magic_link', label: 'Magic Link' },
]

function detectSiteLabel(): string {
  if (typeof window === 'undefined') return 'your workspace'
  const pathMatch = window.location.pathname.match(/^\/s\/([^/]+)/)
  if (pathMatch?.[1]) {
    return pathMatch[1]
  }
  const host = window.location.hostname
  if (!host || host === 'localhost') return 'your workspace'
  const first = host.split('.')[0]
  if (!first || first === 'app') return 'your workspace'
  return first
}

export default function LoginPage() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [mode, setMode] = useState<AuthMode>('magic')
  const [magicSentTo, setMagicSentTo] = useState('')
  const [isVerifyingToken, setIsVerifyingToken] = useState(false)
  const [magicTokenHandled, setMagicTokenHandled] = useState(false)
  const [siteLabel] = useState(() => detectSiteLabel())
  const { login, fetchProviders, requestMagicLink, requestEmailVerification, verifyMagicLink, providers, isLoading, error, errorType, clearError } = useAuthStore()
  const navigate = useNavigate()

  const magicToken = useMemo(() => {
    if (typeof window === 'undefined') return ''
    return new URLSearchParams(window.location.search).get('magic_token') || ''
  }, [])

  const availableProviders = providers.length > 0 ? providers : defaultProviders
  const hasPassword = availableProviders.some(p => p.name === 'password')
  const hasMagic = availableProviders.some(p => p.name === 'magic_link')

  useEffect(() => {
    void fetchProviders()
  }, [fetchProviders])

  useEffect(() => {
    if (!hasMagic && hasPassword) {
      setMode('password')
    } else if (hasMagic && !hasPassword) {
      setMode('magic')
    }
  }, [hasMagic, hasPassword])

  useEffect(() => {
    if (!magicToken || magicTokenHandled) return
    setMagicTokenHandled(true)
    setIsVerifyingToken(true)

    void (async () => {
      try {
        await verifyMagicLink(magicToken)
        navigate({ to: '/workspace' })
      } catch {
        setMode('magic')
      } finally {
        setIsVerifyingToken(false)
      }
    })()
  }, [magicToken, magicTokenHandled, navigate, verifyMagicLink])

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    try {
      if (mode === 'password' && hasPassword) {
        await login(email, password)
        navigate({ to: '/workspace' })
        return
      }

      if (!hasMagic) return
      await requestMagicLink(email)
      setMagicSentTo(email.trim())
    } catch {
      // Store already contains the error state.
    }
  }

  const switchMode = (nextMode: AuthMode) => {
    if (nextMode === 'password' && !hasPassword) return
    if (nextMode === 'magic' && !hasMagic) return
    setMode(nextMode)
    setMagicSentTo('')
    if (error) clearError()
  }

  const primaryProviderLabel = mode === 'password'
    ? (availableProviders.find(p => p.name === 'password')?.label || 'Email & Password')
    : (availableProviders.find(p => p.name === 'magic_link')?.label || 'Magic Link')

  const helperTitle = mode === 'password' && hasPassword
    ? 'Use your existing password'
    : `${primaryProviderLabel} is one-time and passwordless`
  const helperBody = mode === 'password' && hasPassword
    ? 'Best for existing team members who already have a saved password.'
    : 'We will email a short-lived sign-in link. Open it on this device to finish signing in.'

  if (isVerifyingToken) {
    return (
      <div className="min-h-screen bg-[linear-gradient(180deg,#f5f7f4_0%,#eef5ef_35%,#ffffff_100%)] px-4 py-10">
        <div className="mx-auto flex min-h-[70vh] max-w-md items-center justify-center">
          <Card className="w-full overflow-hidden border-emerald-100 bg-white/95 shadow-2xl shadow-emerald-950/10">
            <CardContent className="flex flex-col items-center gap-4 py-12 text-center">
              <div className="rounded-full border border-emerald-200 bg-emerald-50 p-3 text-emerald-600">
                <Loader2 className="h-6 w-6 animate-spin" />
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
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-[linear-gradient(180deg,#f5f7f4_0%,#eef5ef_35%,#ffffff_100%)] px-4 py-6 sm:py-10">
      <div className="mx-auto grid min-h-[calc(100vh-3rem)] max-w-6xl items-center gap-8 lg:grid-cols-[1.08fr_0.92fr]">
        <section className="hidden h-full flex-col justify-between overflow-hidden rounded-[32px] border border-black/5 bg-[linear-gradient(180deg,#10231a_0%,#153325_35%,#0d1712_100%)] p-8 text-slate-50 shadow-2xl shadow-emerald-950/15 lg:flex">
          <div className="space-y-8">
            <div className="flex items-center gap-3">
              <div className="rounded-2xl border border-white/10 bg-white/5 p-2">
                <LogoMark size={30} />
              </div>
              <div>
                <div className="text-sm font-medium text-slate-300">Kora</div>
                <div className="text-xs text-slate-400">Workspace access for {siteLabel}</div>
              </div>
            </div>

            <div className="space-y-4">
              <div className="inline-flex items-center gap-2 rounded-full border border-emerald-400/20 bg-emerald-400/10 px-3 py-1 text-xs font-semibold text-emerald-200">
                <Sparkles className="h-3.5 w-3.5" />
                Fast, secure entry
              </div>
              <h1 className="max-w-lg text-4xl font-semibold tracking-tight">
                Clear sign-in flow, even when the user is on mobile or coming from email.
              </h1>
              <p className="max-w-xl text-base leading-7 text-slate-300">
                This page is tuned for the two real cases that matter: quick one-time access through email and stable password sign-in for existing teams.
              </p>
            </div>
          </div>

          <div className="grid gap-3 text-sm text-slate-300">
            <div className="rounded-3xl border border-white/10 bg-white/5 p-5">
              <div className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-200/80">What users should expect</div>
              <div className="mt-4 space-y-4">
                <div className="flex items-start gap-3">
                  <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-emerald-300" />
                  <div>
                    <div className="font-medium text-slate-50">Short-lived links</div>
                    <div className="mt-0.5 text-slate-400">Good for first-time access and lower support overhead.</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-300" />
                  <div>
                    <div className="font-medium text-slate-50">Password fallback stays available</div>
                    <div className="mt-0.5 text-slate-400">Existing users do not need to relearn the flow.</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <Mail className="mt-0.5 h-4 w-4 shrink-0 text-emerald-300" />
                  <div>
                    <div className="font-medium text-slate-50">Works with assisted sign-in flows</div>
                    <div className="mt-0.5 text-slate-400">Email links remain usable from WhatsApp or mobile handoff.</div>
                  </div>
                </div>
              </div>
            </div>
            <div className="flex items-start gap-3 rounded-2xl border border-white/10 bg-white/5 p-4">
              <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-emerald-300" />
              <div>
                <div className="font-medium text-slate-50">Workspace</div>
                <div className="mt-0.5 text-slate-400">{siteLabel}</div>
              </div>
            </div>
          </div>
        </section>

        <section className="mx-auto w-full max-w-md">
          <div className="mb-4 flex items-center justify-between lg:hidden">
            <div className="flex items-center gap-3">
              <div className="rounded-2xl border border-black/5 bg-white p-2 shadow-sm">
                <LogoMark size={28} />
              </div>
              <div>
                <div className="text-sm font-medium text-slate-900">Kora</div>
                <div className="text-xs text-slate-500">{siteLabel}</div>
              </div>
            </div>
          </div>

          <Card className="overflow-hidden border-black/5 bg-white/95 shadow-2xl shadow-slate-950/10 backdrop-blur">
            <CardHeader className="space-y-4 pb-5 text-center">
              <div className="mx-auto rounded-2xl border border-emerald-100 bg-emerald-50 p-3 text-emerald-700">
                <LogoMark size={34} />
              </div>
              <div className="space-y-1">
                <CardTitle className="text-2xl tracking-tight">Sign in to {siteLabel}</CardTitle>
                <CardDescription>Choose the fastest safe path into your workspace</CardDescription>
              </div>

              <div className={`grid gap-2 rounded-2xl bg-muted p-1 ${hasPassword && hasMagic ? 'grid-cols-2' : 'grid-cols-1'}`}>
                {hasMagic && (
                  <button
                    type="button"
                    aria-pressed={mode === 'magic'}
                    onClick={() => switchMode('magic')}
                    className={`flex items-center justify-center gap-2 rounded-xl px-3 py-2 text-sm font-medium transition ${
                      mode === 'magic' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    <Mail className="h-4 w-4" />
                    {availableProviders.find(p => p.name === 'magic_link')?.label || 'Magic link'}
                  </button>
                )}
                {hasPassword && (
                  <button
                    type="button"
                    aria-pressed={mode === 'password'}
                    onClick={() => switchMode('password')}
                    className={`flex items-center justify-center gap-2 rounded-xl px-3 py-2 text-sm font-medium transition ${
                      mode === 'password' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    <KeyRound className="h-4 w-4" />
                    {availableProviders.find(p => p.name === 'password')?.label || 'Email & Password'}
                  </button>
                )}
              </div>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleSubmit} className="space-y-4">
                {magicSentTo && mode === 'magic' && !error ? (
                  <div className="rounded-2xl border border-emerald-200 bg-emerald-50/80 p-4 text-left text-sm">
                    <div className="flex items-start gap-3">
                      <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />
                      <div className="space-y-2">
                        <div className="font-medium text-emerald-950">Check your inbox</div>
                        <p className="text-emerald-900/90">
                          We sent a sign-in link to <span className="font-semibold">{magicSentTo}</span>.
                          Open it on this device to finish signing in.
                        </p>
                        <div className="flex flex-wrap gap-2">
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            className="border-emerald-300 bg-white text-emerald-900 hover:bg-emerald-100"
                            onClick={() => {
                              setMagicSentTo('')
                              clearError()
                            }}
                          >
                            <ArrowLeft className="mr-1 h-3.5 w-3.5" />
                            Use another email
                          </Button>
                        </div>
                      </div>
                    </div>
                  </div>
                ) : null}

                <div className="space-y-2">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    type="email"
                    placeholder="admin@example.com"
                    value={email}
                    onChange={(e) => {
                      setEmail(e.target.value)
                      if (error) clearError()
                    }}
                    required
                    autoFocus
                    className="h-11"
                    autoComplete="email"
                  />
                </div>

                {mode === 'password' && hasPassword ? (
                  <div className="space-y-2">
                    <Label htmlFor="password">Password</Label>
                    <PasswordInput
                      id="password"
                      placeholder="Password"
                      value={password}
                      onChange={(e) => {
                        setPassword(e.target.value)
                        if (error) clearError()
                      }}
                      required
                      className="h-11 pr-11"
                      autoComplete="current-password"
                    />
                  </div>
                ) : (
                  <div className="rounded-2xl border border-dashed border-emerald-200 bg-emerald-50/70 p-4 text-sm text-emerald-950">
                    <div className="font-medium">{helperTitle}</div>
                    <div className="mt-1 leading-6 text-emerald-900/80">
                      {helperBody}
                    </div>
                  </div>
                )}

                {error && (
                  <div className="flex items-start gap-3 rounded-2xl border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm">
                    <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
                    <div className="space-y-3">
                      <p className="text-destructive">{error}</p>
                      {errorType === 'email_verification_required' && (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={async () => {
                            try {
                              await requestEmailVerification(email)
                              setMagicSentTo(email.trim())
                              clearError()
                            } catch {
                              // store carries the error
                            }
                          }}
                          className="border-destructive/30 text-destructive hover:bg-destructive/10"
                        >
                          Resend verification email
                        </Button>
                      )}
                    </div>
                  </div>
                )}

                <Button type="submit" className="h-11 w-full" disabled={isLoading}>
                  {isLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                  {mode === 'password' && hasPassword ? 'Sign in' : magicSentTo ? 'Send another sign-in link' : 'Send sign-in link'}
                </Button>

                <div className="rounded-2xl bg-slate-50 px-4 py-3 text-xs leading-5 text-slate-600">
                  <div className="font-medium text-slate-800">Before you continue</div>
                  <p className="mt-1">
                    New accounts usually start with magic links. Existing teams can keep password sign-in enabled as a fallback.
                  </p>
                </div>
              </form>
            </CardContent>
          </Card>
        </section>
      </div>
    </div>
  )
}
