/**
 * Standardized API error format
 */
export interface ApiError {
  status: number
  code: string
  message: string
  details?: unknown
}

export function isApiError(error: unknown): error is ApiError {
  return (
    typeof error === 'object' &&
    error !== null &&
    'status' in error &&
    'code' in error &&
    'message' in error
  )
}

export function transformAxiosError(error: unknown): ApiError {
  const err = error as {
    response?: {
      status: number
      data?: { message?: string; error?: string; code?: string; details?: unknown }
    }
    code?: string
    message?: string
  }
  if (!err.response) {
    return {
      status: 0,
      code: err.code === 'ECONNABORTED' ? 'TIMEOUT' : 'NETWORK_ERROR',
      message: err.code === 'ECONNABORTED' ? 'Request timed out' : 'Network error',
    }
  }

  const { status, data } = err.response
  return {
    status,
    code: data?.code || 'ERROR',
    message: data?.message || data?.error || err.message || 'An error occurred',
    details: data?.details,
  }
}
