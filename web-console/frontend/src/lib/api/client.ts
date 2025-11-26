import axios, { AxiosError } from 'axios'
import { transformAxiosError } from '../types/api'

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

export const apiClient = axios.create({
  baseURL: `${API_URL}/api/v1`,
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 30000,
})

// Response interceptor with standardized error transformation
apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const apiError = transformAxiosError(error)

    if (process.env.NODE_ENV === 'development') {
      console.error('API Error:', {
        status: apiError.status,
        code: apiError.code,
        message: apiError.message,
      })
    }

    return Promise.reject(apiError)
  }
)
