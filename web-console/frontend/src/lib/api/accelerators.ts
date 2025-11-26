import { apiClient } from './client'
import { AcceleratorClass } from '../types/accelerator'
import { ListResponse } from '../types/common'

export const acceleratorsApi = {
  list: async (): Promise<ListResponse<AcceleratorClass>> => {
    const response = await apiClient.get<ListResponse<AcceleratorClass>>('/accelerators')
    return response.data
  },

  get: async (name: string): Promise<AcceleratorClass> => {
    const response = await apiClient.get<AcceleratorClass>(`/accelerators/${name}`)
    return response.data
  },
}
