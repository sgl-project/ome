import { apiClient } from './client'
import { InferenceService } from '../types/service'

export interface ServiceListResponse {
  items: InferenceService[]
  total: number
}

export const servicesApi = {
  list: async (namespace?: string): Promise<ServiceListResponse> => {
    const params = namespace ? { namespace } : {}
    const response = await apiClient.get<ServiceListResponse>('/services', { params })
    return response.data
  },

  get: async (name: string, namespace?: string): Promise<InferenceService> => {
    const params = namespace ? { namespace } : {}
    const response = await apiClient.get<InferenceService>(`/services/${name}`, { params })
    return response.data
  },

  create: async (service: Partial<InferenceService>): Promise<InferenceService> => {
    const response = await apiClient.post<InferenceService>('/services', service)
    return response.data
  },

  update: async (name: string, service: Partial<InferenceService>): Promise<InferenceService> => {
    const response = await apiClient.put<InferenceService>(`/services/${name}`, service)
    return response.data
  },

  delete: async (name: string): Promise<void> => {
    await apiClient.delete(`/services/${name}`)
  },

  getStatus: async (name: string, namespace?: string): Promise<any> => {
    const params = namespace ? { namespace } : {}
    const response = await apiClient.get(`/services/${name}/status`, { params })
    return response.data
  },
}
