import type { OperationEnvelope, OperationError, OperationStatus } from '@/types/api'
import { api, KoraApiError } from '@/lib/api/client'

export function toOperationError(error: unknown): OperationError {
  if (error instanceof KoraApiError) {
    return {
      code: error.type || 'error',
      message: error.message,
      field: error.field,
      retryable: error.status >= 500,
    }
  }
  if (error instanceof Error) {
    return {
      code: 'error',
      message: error.message,
      retryable: false,
    }
  }
  return {
    code: 'unknown',
    message: 'Unknown error',
    retryable: false,
  }
}

export function toCompletedOperation<T>(data: T, correlationId?: string): OperationEnvelope<T> {
  return {
    operation_id: crypto.randomUUID(),
    correlation_id: correlationId,
    status: 'completed',
    data,
  }
}

export function toFailedOperation<T = never>(error: unknown, correlationId?: string): OperationEnvelope<T> {
  return {
    operation_id: crypto.randomUUID(),
    correlation_id: correlationId,
    status: 'failed',
    error: toOperationError(error),
  }
}

export async function executeOperation<T>(
  method: 'POST' | 'PUT' | 'DELETE',
  path: string,
  body?: unknown,
  params?: Record<string, string | number | undefined>,
): Promise<OperationEnvelope<T>> {
  try {
    const data = method === 'DELETE'
      ? await api.delete<T>(path)
      : method === 'PUT'
        ? await api.put<T>(path, body)
        : await api.post<T>(path, body)
    return toCompletedOperation(data as T)
  } catch (error) {
    return toFailedOperation<T>(error)
  }
}

export function isTerminalOperationStatus(status: OperationStatus): boolean {
  return status === 'completed' || status === 'rejected' || status === 'conflict' || status === 'failed'
}
