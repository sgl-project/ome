import { apiClient } from './client'

export interface NamespacesListResponse {
  items: string[]
  total: number
}

export const namespacesApi = {
  list: async (): Promise<NamespacesListResponse> => {
    const response = await apiClient.get<NamespacesListResponse>('/namespaces')
    return response.data
  },

  get: async (name: string): Promise<any> => {
    const response = await apiClient.get(`/namespaces/${name}`)
    return response.data
  },
}
