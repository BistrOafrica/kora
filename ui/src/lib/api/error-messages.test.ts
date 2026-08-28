import { describe, expect, it } from 'vitest'
import { getApiErrorCode, getApiErrorMessage, mapApiErrorMessage } from './error-messages'

describe('api error messages', () => {
  it('prefers explicit error codes', () => {
    expect(getApiErrorCode({ code: 'auth.invalid_credentials', message: 'Invalid credentials' })).toBe('auth.invalid_credentials')
  })

  it('falls back to legacy type fields', () => {
    expect(getApiErrorCode({ type: 'csrf.token_mismatch', message: 'CSRF token mismatch' })).toBe('csrf.token_mismatch')
  })

  it('maps backend codes to translated user-facing messages', () => {
    expect(mapApiErrorMessage('auth.invalid_credentials')).toBe('The email or password is incorrect.')
    expect(getApiErrorMessage({ code: 'validation.failed', message: 'Validation failed' })).toBe('Please fix the highlighted fields and try again.')
  })
})
