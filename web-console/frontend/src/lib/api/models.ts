import { apiClient } from './client'
import { ClusterBaseModel, BaseModel } from '../types/model'

export interface ModelListResponse {
  items: ClusterBaseModel[]
  total: number
}

export interface BaseModelListResponse {
  items: BaseModel[]
  total: number
}

// ClusterBaseModel API (cluster-scoped)
export const modelsApi = {
  list: async (): Promise<ModelListResponse> => {
    const response = await apiClient.get<ModelListResponse>('/models')
    return response.data
  },

  get: async (name: string): Promise<ClusterBaseModel> => {
    const response = await apiClient.get<ClusterBaseModel>(`/models/${name}`)
    return response.data
  },

  create: async (requestBody: { model: Partial<ClusterBaseModel>; huggingfaceToken?: string }): Promise<ClusterBaseModel> => {
    const response = await apiClient.post<ClusterBaseModel>('/models', requestBody)
    return response.data
  },

  update: async (name: string, model: Partial<ClusterBaseModel>): Promise<ClusterBaseModel> => {
    const response = await apiClient.put<ClusterBaseModel>(`/models/${name}`, model)
    return response.data
  },

  delete: async (name: string): Promise<void> => {
    await apiClient.delete(`/models/${name}`)
  },

  getStatus: async (name: string): Promise<any> => {
    const response = await apiClient.get(`/models/${name}/status`)
    return response.data
  },
}

// BaseModel API (namespace-scoped)
export const baseModelsApi = {
  list: async (namespace: string): Promise<BaseModelListResponse> => {
    const response = await apiClient.get<BaseModelListResponse>(`/namespaces/${namespace}/models`)
    return response.data
  },

  get: async (namespace: string, name: string): Promise<BaseModel> => {
    const response = await apiClient.get<BaseModel>(`/namespaces/${namespace}/models/${name}`)
    return response.data
  },

  create: async (namespace: string, requestBody: { model: Partial<BaseModel>; huggingfaceToken?: string }): Promise<BaseModel> => {
    const response = await apiClient.post<BaseModel>(`/namespaces/${namespace}/models`, requestBody)
    return response.data
  },

  update: async (namespace: string, name: string, model: Partial<BaseModel>): Promise<BaseModel> => {
    const response = await apiClient.put<BaseModel>(`/namespaces/${namespace}/models/${name}`, model)
    return response.data
  },

  delete: async (namespace: string, name: string): Promise<void> => {
    await apiClient.delete(`/namespaces/${namespace}/models/${name}`)
  },
}
