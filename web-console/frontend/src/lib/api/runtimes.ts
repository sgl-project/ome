import { apiClient } from './client'
import { ClusterServingRuntime } from '../types/runtime'

export interface RuntimeListResponse {
  items: ClusterServingRuntime[]
  total: number
}

export const runtimesApi = {
  list: async (): Promise<RuntimeListResponse> => {
    const response = await apiClient.get<RuntimeListResponse>('/runtimes')
    return response.data
  },

  get: async (name: string): Promise<ClusterServingRuntime> => {
    const response = await apiClient.get<ClusterServingRuntime>(`/runtimes/${name}`)
    return response.data
  },

  create: async (runtime: Partial<ClusterServingRuntime>): Promise<ClusterServingRuntime> => {
    const response = await apiClient.post<ClusterServingRuntime>('/runtimes', runtime)
    return response.data
  },

  update: async (name: string, runtime: Partial<ClusterServingRuntime>): Promise<ClusterServingRuntime> => {
    const response = await apiClient.put<ClusterServingRuntime>(`/runtimes/${name}`, runtime)
    return response.data
  },

  delete: async (name: string): Promise<void> => {
    await apiClient.delete(`/runtimes/${name}`)
  },
}
