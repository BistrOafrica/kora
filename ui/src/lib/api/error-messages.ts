import type { ApiError } from '@/types/api'

const DEFAULT_MESSAGE = 'The server returned an error.'

const ERROR_MESSAGES: Record<string, string> = {
  'auth.authentication_required': 'Please sign in to continue.',
  'auth.invalid_credentials': 'The email or password is incorrect.',
  'auth.session_invalid': 'Your session expired. Please sign in again.',
  'csrf.token_required': 'Your session needs a refresh. Reload the page and try again.',
  'csrf.token_mismatch': 'Your session needs a refresh. Reload the page and try again.',
  'doctype.already_exists': 'A record with that name already exists.',
  'resource.doctype_not_found': 'The selected record type was not found.',
  'resource.document_not_found': 'The record could not be found.',
  'validation.failed': 'Please fix the highlighted fields and try again.',
  'validation.invalid_json': 'The submitted data could not be read.',
  'validation.required_field': 'Please complete the required fields.',
  'validation.invalid_child_table': 'One of the nested rows is invalid.',
  'kernel.kernel.code_permission_denied': 'You do not have permission to perform this action.',
  'kernel.kernel.code_unauthenticated': 'Please sign in to continue.',
  'kernel.kernel.code_validation_failed': 'Please fix the highlighted fields and try again.',
  'kernel.kernel.code_not_found': 'The requested item was not found.',
  'kernel.kernel.code_conflict': 'This change conflicts with a newer version.',
}

function normalizeCode(error: Partial<ApiError> & { code?: string; type?: string }): string {
  return (error.code || error.type || 'error').trim()
}

export function getApiErrorCode(error: Partial<ApiError> & { code?: string; type?: string }): string {
  return normalizeCode(error)
}

export function getApiErrorMessage(error: Partial<ApiError> & { code?: string; type?: string; message?: string }): string {
  const code = normalizeCode(error)
  return ERROR_MESSAGES[code] || error.message || DEFAULT_MESSAGE
}

export function mapApiErrorMessage(code: string, fallback?: string): string {
  return ERROR_MESSAGES[code] || fallback || DEFAULT_MESSAGE
}
