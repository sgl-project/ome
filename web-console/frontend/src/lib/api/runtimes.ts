import { apiClient } from './client'
import { ClusterServingRuntime, RuntimeMatch, CompatibilityCheck, RuntimeValidationResult } from '../types/runtime'

export interface RuntimeListResponse {
  items: ClusterServingRuntime[]
  total: number
}

export interface CompatibleRuntimesResponse {
  matches: RuntimeMatch[]
  total: number
}

export const runtimesApi = {
  list: async (namespace?: string): Promise<RuntimeListResponse> => {
    const params = namespace ? { namespace } : {}
    const response = await apiClient.get<RuntimeListResponse>('/runtimes', { params })
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

  // Intelligence features
  findCompatible: async (format: string, framework?: string): Promise<CompatibleRuntimesResponse> => {
    const params: Record<string, string> = { format }
    if (framework) params.framework = framework
    const response = await apiClient.get<CompatibleRuntimesResponse>('/runtimes/compatible', { params })
    return response.data
  },

  checkCompatibility: async (name: string, format: string, framework?: string): Promise<CompatibilityCheck> => {
    const params: Record<string, string> = { format }
    if (framework) params.framework = framework
    const response = await apiClient.get<CompatibilityCheck>(`/runtimes/${name}/compatibility`, { params })
    return response.data
  },

  getRecommendation: async (format: string, framework?: string): Promise<RuntimeMatch> => {
    const params: Record<string, string> = { format }
    if (framework) params.framework = framework
    const response = await apiClient.get<RuntimeMatch>('/runtimes/recommend', { params })
    return response.data
  },

  validate: async (runtime: Partial<ClusterServingRuntime>): Promise<RuntimeValidationResult> => {
    const response = await apiClient.post<RuntimeValidationResult>('/runtimes/validate', runtime)
    return response.data
  },

  clone: async (name: string, newName: string): Promise<ClusterServingRuntime> => {
    const response = await apiClient.post<ClusterServingRuntime>(`/runtimes/${name}/clone`, { newName })
    return response.data
  },
}
