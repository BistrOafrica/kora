import { useState, type FormEvent } from 'react'
import { AlertCircle, CheckCircle2, Loader2, Mail, KeyRound, ArrowLeft } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/ui/password-input'
import { Label } from '@/components/ui/label'
import { Field, FieldGroup, FieldLabel, FieldDescription, FieldSeparator } from '@/components/ui/field'
import { useAuthStore } from '@/lib/auth-store'
import type { AuthProvider } from '@/types/api'

type AuthMode = 'password' | 'magic'

const defaultProviders: AuthProvider[] = [
  { name: 'password', label: 'Email & Password' },
  { name: 'magic_link', label: 'Magic Link' },
]

interface LoginFormProps {
  siteLabel: string
  onSuccess?: () => void
  className?: string
}

export function LoginForm({ siteLabel, onSuccess, className }: LoginFormProps) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [mode, setMode] = useState<AuthMode>('magic')
  const [magicSentTo, setMagicSentTo] = useState('')
  const { login, fetchProviders, requestMagicLink, requestEmailVerification, providers, isLoading, error, errorType, clearError } = useAuthStore()

  const availableProviders = providers.length > 0 ? providers : defaultProviders
  const hasPassword = availableProviders.some(p => p.name === 'password')
  const hasMagic = availableProviders.some(p => p.name === 'magic_link')

  // Auto-select available mode
  useState(() => {
    if (!hasMagic && hasPassword) setMode('password')
    else if (hasMagic && !hasPassword) setMode('magic')
  })

  // Fetch providers on mount
  useState(() => {
    void fetchProviders()
  })

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    try {
      if (mode === 'password' && hasPassword) {
        await login(email, password)
        onSuccess?.()
        return
      }
      if (!hasMagic) return
      await requestMagicLink(email)
      setMagicSentTo(email.trim())
    } catch {
      // error is in auth store
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

  return (
    <form onSubmit={handleSubmit} className={cn('flex flex-col gap-6', className)}>
      <FieldGroup>
        <div className="flex flex-col items-center gap-1 text-center">
          <h1 className="text-2xl font-bold text-card-foreground">Sign in to {siteLabel}</h1>
          <p className="text-sm text-balance text-muted-foreground">
            Your team, your data, your way.
          </p>
        </div>

        {/* Mode Tabs */}
        <div className={`grid gap-2 rounded-2xl bg-muted p-1 ${hasPassword && hasMagic ? 'grid-cols-2' : 'grid-cols-1'}`}>
          {hasMagic && (
            <button
              type="button"
              aria-pressed={mode === 'magic'}
              onClick={() => switchMode('magic')}
              className={cn(
                'flex items-center justify-center gap-2 rounded-xl px-3 py-2 text-sm font-medium transition',
                mode === 'magic' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
              )}
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
              className={cn(
                'flex items-center justify-center gap-2 rounded-xl px-3 py-2 text-sm font-medium transition',
                mode === 'password' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <KeyRound className="h-4 w-4" />
              {availableProviders.find(p => p.name === 'password')?.label || 'Email & Password'}
            </button>
          )}
        </div>

        {/* Magic Link Sent Confirmation */}
        {magicSentTo && mode === 'magic' && !error ? (
          <Field>
            <div className="rounded-2xl border border-emerald-200 bg-emerald-50/80 p-4 text-left text-sm">
              <div className="flex items-start gap-3">
                <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />
                <div className="space-y-2">
                  <div className="font-medium text-emerald-950">Check your inbox</div>
                  <p className="text-emerald-900/90">
                    We sent a sign-in link to <span className="font-semibold">{magicSentTo}</span>.
                    Open it on this device to finish signing in.
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="border-emerald-300 bg-white text-emerald-900 hover:bg-emerald-100"
                    onClick={() => { setMagicSentTo(''); clearError() }}
                  >
                    <ArrowLeft className="mr-1 h-3.5 w-3.5" />
                    Use another email
                  </Button>
                </div>
              </div>
            </div>
          </Field>
        ) : null}

        {/* Email Field */}
        <Field>
          <FieldLabel htmlFor="email">Email</FieldLabel>
          <Input
            id="email"
            type="email"
            placeholder="admin@example.com"
            value={email}
            onChange={(e) => { setEmail(e.target.value); if (error) clearError() }}
            required
            autoFocus
            className="h-11"
            autoComplete="email"
          />
        </Field>

        {/* Password Field */}
        {mode === 'password' && hasPassword ? (
          <Field>
            <div className="flex items-center">
              <FieldLabel htmlFor="password">Password</FieldLabel>
            </div>
            <PasswordInput
              id="password"
              placeholder="Password"
              value={password}
              onChange={(e) => { setPassword(e.target.value); if (error) clearError() }}
              required
              className="h-11 pr-11"
              autoComplete="current-password"
            />
          </Field>
        ) : !magicSentTo ? (
          <Field>
            <div className="rounded-2xl border border-dashed border-emerald-200 bg-emerald-50/70 p-4 text-sm text-emerald-950">
              <div className="font-medium">{helperTitle}</div>
              <div className="mt-1 leading-6 text-emerald-900/80">{helperBody}</div>
            </div>
          </Field>
        ) : null}

        {/* Error */}
        {error && (
          <Field>
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
                      } catch { /* store carries error */ }
                    }}
                    className="border-destructive/30 text-destructive hover:bg-destructive/10"
                  >
                    Resend verification email
                  </Button>
                )}
              </div>
            </div>
          </Field>
        )}

        {/* Submit */}
        <Field>
          <Button type="submit" className="h-11 w-full" disabled={isLoading}>
            {isLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
            {mode === 'password' && hasPassword ? 'Sign in' : magicSentTo ? 'Send another sign-in link' : 'Send sign-in link'}
          </Button>
        </Field>
      </FieldGroup>

      <FieldDescription className="text-center text-xs">
        First time here? Magic link is the quickest way in. Existing teams can use passwords too.
      </FieldDescription>

      <div className="mt-4 text-center text-sm">
        Don&apos;t have an account?{' '}
        <a href="https://kora.mradiafrica.com/onboard" className="font-medium text-primary underline underline-offset-4 hover:opacity-80">
          Sign up
        </a>
      </div>
    </form>
  )
}
