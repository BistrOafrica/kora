import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { AlertCircle, CheckCircle2, KeyRound, Loader2, Mail, ShieldCheck, Sparkles } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { LogoMark } from '@/components/ui/LogoMark'
import { useAuthStore } from '@/lib/auth-store'
import type { AuthProvider } from '@/types/api'

type AuthMode = 'password' | 'magic'

const defaultProviders: AuthProvider[] = [
  { name: 'password', label: 'Email & Password' },
  { name: 'magic_link', label: 'Magic Link' },
]

export default function LoginPage() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [mode, setMode] = useState<AuthMode>('magic')
  const [magicSentTo, setMagicSentTo] = useState('')
  const [isVerifyingToken, setIsVerifyingToken] = useState(false)
  const [magicTokenHandled, setMagicTokenHandled] = useState(false)
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

  if (isVerifyingToken) {
    return (
      <div className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(16,185,129,0.12),_transparent_32%),linear-gradient(180deg,#fff_0%,#f8fafc_100%)] px-4 py-10">
        <div className="mx-auto flex min-h-[70vh] max-w-md items-center justify-center">
          <Card className="w-full border-border/70 shadow-2xl shadow-emerald-950/5">
            <CardContent className="flex flex-col items-center gap-4 py-12 text-center">
              <div className="rounded-full border border-emerald-200 bg-emerald-50 p-3 text-emerald-600">
                <Loader2 className="h-6 w-6 animate-spin" />
              </div>
              <div className="space-y-1">
                <h1 className="text-xl font-semibold tracking-tight">Verifying sign-in link</h1>
                <p className="text-sm text-muted-foreground">
                  This should only take a moment.
                </p>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(16,185,129,0.12),_transparent_32%),linear-gradient(180deg,#fff_0%,#f8fafc_100%)] px-4 py-6 sm:py-10">
      <div className="mx-auto grid min-h-[calc(100vh-3rem)] max-w-6xl items-center gap-8 lg:grid-cols-[1.05fr_0.95fr]">
        <section className="hidden h-full flex-col justify-between rounded-[28px] border border-border/70 bg-slate-950 p-8 text-slate-50 shadow-2xl shadow-slate-950/20 lg:flex">
          <div className="space-y-8">
            <div className="flex items-center gap-3">
              <div className="rounded-2xl border border-white/10 bg-white/5 p-2">
                <LogoMark size={30} />
              </div>
              <div>
                <div className="text-sm font-medium text-slate-300">Kora</div>
                <div className="text-xs text-slate-400">Enterprise workspace access</div>
              </div>
            </div>

            <div className="space-y-4">
              <div className="inline-flex items-center gap-2 rounded-full border border-emerald-400/20 bg-emerald-400/10 px-3 py-1 text-xs font-semibold text-emerald-200">
                <Sparkles className="h-3.5 w-3.5" />
                Fast, secure entry
              </div>
              <h1 className="max-w-lg text-4xl font-semibold tracking-tight">
                Sign in the way your team works best.
              </h1>
              <p className="max-w-xl text-base leading-7 text-slate-300">
                Use a magic link for quick access or a password for existing accounts.
                The login flow stays simple on mobile and predictable for non-technical users.
              </p>
            </div>
          </div>

          <div className="grid gap-3 text-sm text-slate-300">
            <div className="flex items-start gap-3 rounded-2xl border border-white/10 bg-white/5 p-4">
              <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-emerald-300" />
              <div>
                <div className="font-medium text-slate-50">One-time link by default</div>
                <div className="mt-0.5 text-slate-400">New signups avoid password friction and reduce support load.</div>
              </div>
            </div>
            <div className="flex items-start gap-3 rounded-2xl border border-white/10 bg-white/5 p-4">
              <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-300" />
              <div>
                <div className="font-medium text-slate-50">Password fallback for existing teams</div>
                <div className="mt-0.5 text-slate-400">Keep legacy access working while you transition users gradually.</div>
              </div>
            </div>
            <div className="flex items-start gap-3 rounded-2xl border border-white/10 bg-white/5 p-4">
              <Mail className="mt-0.5 h-4 w-4 shrink-0 text-emerald-300" />
              <div>
                <div className="font-medium text-slate-50">Clean mobile handoff</div>
                <div className="mt-0.5 text-slate-400">The same link works on desktop, WhatsApp-assisted flows, or mobile email.</div>
              </div>
            </div>
          </div>
        </section>

        <section className="mx-auto w-full max-w-md">
          <Card className="border-border/70 bg-white/95 shadow-2xl shadow-slate-950/10 backdrop-blur">
            <CardHeader className="space-y-4 pb-5 text-center">
              <div className="mx-auto">
                <LogoMark size={34} />
              </div>
              <div className="space-y-1">
                <CardTitle className="text-2xl tracking-tight">Welcome back</CardTitle>
                <CardDescription>Choose the sign-in method that fits your account</CardDescription>
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
                  />
                </div>

                {mode === 'password' && hasPassword ? (
                  <div className="space-y-2">
                    <Label htmlFor="password">Password</Label>
                    <Input
                      id="password"
                      type="password"
                      placeholder="••••••••"
                      value={password}
                      onChange={(e) => {
                        setPassword(e.target.value)
                        if (error) clearError()
                      }}
                      required
                      className="h-11"
                    />
                  </div>
                ) : (
                  <div className="rounded-2xl border border-dashed border-emerald-200 bg-emerald-50/70 p-4 text-sm text-emerald-950">
                    <div className="font-medium">{primaryProviderLabel} is one-time and passwordless.</div>
                    <div className="mt-1 leading-6 text-emerald-900/80">
                      It expires quickly and does not require a password.
                    </div>
                  </div>
                )}

                {magicSentTo && mode === 'magic' && !error && (
                  <div className="flex items-start gap-3 rounded-2xl border border-emerald-500/30 bg-emerald-500/10 px-4 py-3 text-sm">
                    <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />
                    <div>
                      <div className="font-medium text-emerald-900">Check your inbox</div>
                      <p className="text-emerald-800">
                        We sent a sign-in link to <span className="font-semibold">{magicSentTo}</span>.
                        If it doesn’t arrive, check spam or switch to password sign-in.
                      </p>
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
                  {mode === 'password' && hasPassword ? 'Sign in' : 'Send sign-in link'}
                </Button>

                <p className="text-center text-xs leading-5 text-muted-foreground">
                  New accounts use magic links. Existing teams can keep passwords as a fallback.
                </p>
              </form>
            </CardContent>
          </Card>
        </section>
      </div>
    </div>
  )
}
