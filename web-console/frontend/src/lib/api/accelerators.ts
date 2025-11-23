import { apiClient } from './client'
import { AcceleratorClass } from '../types/accelerator'

export interface AcceleratorListResponse {
  items: AcceleratorClass[]
  total: number
}

export const acceleratorsApi = {
  list: async (): Promise<AcceleratorListResponse> => {
    const response = await apiClient.get<AcceleratorListResponse>('/accelerators')
    return response.data
  },

  get: async (name: string): Promise<AcceleratorClass> => {
    const response = await apiClient.get<AcceleratorClass>(`/accelerators/${name}`)
    return response.data
  },
}
