import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { modelsApi, ModelEventsResponse, ModelProgressResponse } from '../api/models'
import { ClusterBaseModel } from '../types/model'
import { DEFAULT_QUERY_CONFIG, queryKeys } from './createResourceHooks'

/**
 * Models hooks are intentionally NOT using createResourceHooks factory because:
 * - useCreateModel has a custom signature: { model: Partial<ClusterBaseModel>; huggingfaceToken?: string }
 *   This differs from the factory's standard create(data: T) pattern
 * - The HuggingFace token handling is specific to model imports
 *
 * However, these hooks follow the same query key patterns and default configurations
 * as factory-generated hooks for consistency.
 *
 * See useRuntimes.ts and useServices.ts for examples of factory usage.
 */

const RESOURCE_KEY = 'models'

export function useModels(namespace?: string) {
  return useQuery({
    queryKey: queryKeys.list(RESOURCE_KEY, namespace),
    queryFn: () => modelsApi.list(namespace),
    staleTime: DEFAULT_QUERY_CONFIG.staleTime,
    gcTime: DEFAULT_QUERY_CONFIG.gcTime,
    retry: DEFAULT_QUERY_CONFIG.retry,
    retryDelay: DEFAULT_QUERY_CONFIG.retryDelay,
  })
}

export function useModel(name: string) {
  return useQuery({
    queryKey: queryKeys.detail(RESOURCE_KEY, name),
    queryFn: () => modelsApi.get(name),
    enabled: !!name,
    staleTime: DEFAULT_QUERY_CONFIG.staleTime,
    gcTime: DEFAULT_QUERY_CONFIG.gcTime,
    retry: DEFAULT_QUERY_CONFIG.retry,
    retryDelay: DEFAULT_QUERY_CONFIG.retryDelay,
  })
}

export function useCreateModel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (requestBody: { model: Partial<ClusterBaseModel>; huggingfaceToken?: string }) =>
      modelsApi.create(requestBody),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.all(RESOURCE_KEY) })
    },
  })
}

export function useUpdateModel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ name, model }: { name: string; model: Partial<ClusterBaseModel> }) =>
      modelsApi.update(name, model),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.all(RESOURCE_KEY) })
    },
  })
}

export function useDeleteModel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (name: string) => modelsApi.delete(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.all(RESOURCE_KEY) })
    },
  })
}

/**
 * Hook to fetch K8s events for a model.
 * Used to display download progress and other events.
 * Polls every 5 seconds when the model is in a downloading state.
 */
export function useModelEvents(name: string, enabled = true, refetchInterval?: number) {
  return useQuery<ModelEventsResponse>({
    queryKey: [...queryKeys.detail(RESOURCE_KEY, name), 'events'],
    queryFn: () => modelsApi.getEvents(name),
    enabled: !!name && enabled,
    staleTime: 2000, // Events can change frequently during download
    gcTime: DEFAULT_QUERY_CONFIG.gcTime,
    retry: DEFAULT_QUERY_CONFIG.retry,
    retryDelay: DEFAULT_QUERY_CONFIG.retryDelay,
    refetchInterval: refetchInterval, // Allow caller to set polling interval
  })
}

/**
 * Hook to fetch real-time download progress from ConfigMaps.
 * This reads directly from ConfigMaps written by model-agent daemonsets,
 * providing more reliable progress updates than K8s events.
 * Polls every 2 seconds when enabled.
 */
export function useModelProgress(name: string, enabled = true, refetchInterval = 2000) {
  return useQuery<ModelProgressResponse>({
    queryKey: [...queryKeys.detail(RESOURCE_KEY, name), 'progress'],
    queryFn: () => modelsApi.getProgress(name),
    enabled: !!name && enabled,
    staleTime: 1000, // Progress changes frequently during download
    gcTime: DEFAULT_QUERY_CONFIG.gcTime,
    retry: DEFAULT_QUERY_CONFIG.retry,
    retryDelay: DEFAULT_QUERY_CONFIG.retryDelay,
    refetchInterval: enabled ? refetchInterval : undefined,
  })
}
