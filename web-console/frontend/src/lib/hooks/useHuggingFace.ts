import { useQuery } from '@tanstack/react-query'
import type {
  HuggingFaceModelSearchResult,
  HuggingFaceModelInfoResponse,
  HuggingFaceModelConfig,
  HuggingFaceSearchParams,
} from '../types/model'

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

// Search HuggingFace models
export function useHuggingFaceSearch(params: HuggingFaceSearchParams) {
  return useQuery({
    queryKey: ['huggingface', 'search', params],
    queryFn: async () => {
      const searchParams = new URLSearchParams()

      if (params.q) searchParams.append('q', params.q)
      if (params.author) searchParams.append('author', params.author)
      if (params.filter) searchParams.append('filter', params.filter)
      if (params.sort) searchParams.append('sort', params.sort)
      if (params.direction) searchParams.append('direction', params.direction)
      if (params.limit) searchParams.append('limit', params.limit.toString())
      if (params.tags) {
        params.tags.forEach((tag) => searchParams.append('tags', tag))
      }

      const url = `${API_BASE_URL}/api/v1/huggingface/models/search?${searchParams.toString()}`
      const response = await fetch(url)

      if (!response.ok) {
        throw new Error('Failed to search HuggingFace models')
      }

      const data = await response.json()
      return data.items as HuggingFaceModelSearchResult[]
    },
    enabled: Boolean(params.q || params.author || params.filter || params.tags?.length),
  })
}

// Get HuggingFace model info
export function useHuggingFaceModelInfo(modelId: string | null) {
  return useQuery({
    queryKey: ['huggingface', 'model', modelId],
    queryFn: async () => {
      if (!modelId) throw new Error('Model ID is required')

      const url = `${API_BASE_URL}/api/v1/huggingface/models/${encodeURIComponent(modelId)}/info`
      const response = await fetch(url)

      if (!response.ok) {
        throw new Error('Failed to fetch model info')
      }

      return response.json() as Promise<HuggingFaceModelInfoResponse>
    },
    enabled: Boolean(modelId),
  })
}

// Get HuggingFace model config
export function useHuggingFaceModelConfig(modelId: string | null) {
  return useQuery({
    queryKey: ['huggingface', 'config', modelId],
    queryFn: async () => {
      if (!modelId) throw new Error('Model ID is required')

      const url = `${API_BASE_URL}/api/v1/huggingface/models/${encodeURIComponent(modelId)}/config`
      const response = await fetch(url)

      if (!response.ok) {
        throw new Error('Failed to fetch model config')
      }

      const data = await response.json()
      return data.config as HuggingFaceModelConfig
    },
    enabled: Boolean(modelId),
  })
}
