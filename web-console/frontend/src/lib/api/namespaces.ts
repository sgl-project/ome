import { apiClient } from './client'

export interface NamespaceListResponse {
  items: string[]
  total: number
}

export const namespacesApi = {
  list: async (): Promise<NamespaceListResponse> => {
    const response = await apiClient.get<NamespaceListResponse>('/namespaces')
    return response.data
  },
}
