import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { runtimesApi } from '../api/runtimes'
import { ClusterServingRuntime } from '../types/runtime'
import {
  createResourceHooks,
  createResourceMutation,
  DEFAULT_QUERY_CONFIG,
  queryKeys,
} from './createResourceHooks'

const RESOURCE_KEY = 'runtimes'

// Create base CRUD hooks using the factory
const runtimeHooks = createResourceHooks<
  ClusterServingRuntime,
  Partial<ClusterServingRuntime>,
  Partial<ClusterServingRuntime>
>(runtimesApi, {
  resourceKey: RESOURCE_KEY,
})

// Export standard CRUD hooks from factory
export const useRuntimes = runtimeHooks.useList
export const useRuntime = runtimeHooks.useGet
export const useCreateRuntime = runtimeHooks.useCreate
export const useDeleteRuntime = runtimeHooks.useDelete

// Custom useUpdateRuntime to match existing API signature
// (existing code passes { name, runtime } instead of { name, data })
export function useUpdateRuntime() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ name, runtime }: { name: string; runtime: Partial<ClusterServingRuntime> }) =>
      runtimesApi.update(name, runtime),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.all(RESOURCE_KEY) })
    },
  })
}

// Runtime Intelligence Hooks (specialized operations not covered by factory)

export function useCompatibleRuntimes(format?: string, framework?: string) {
  return useQuery({
    queryKey: queryKeys.related(RESOURCE_KEY, 'compatible', 'search', format, framework),
    queryFn: () => runtimesApi.findCompatible(format!, framework),
    enabled: !!format,
    staleTime: DEFAULT_QUERY_CONFIG.staleTime,
    gcTime: DEFAULT_QUERY_CONFIG.gcTime,
    retry: DEFAULT_QUERY_CONFIG.retry,
    retryDelay: DEFAULT_QUERY_CONFIG.retryDelay,
  })
}

export function useRuntimeCompatibility(name?: string, format?: string, framework?: string) {
  return useQuery({
    queryKey: queryKeys.related(RESOURCE_KEY, name || '', 'compatibility', format, framework),
    queryFn: () => runtimesApi.checkCompatibility(name!, format!, framework),
    enabled: !!name && !!format,
    staleTime: DEFAULT_QUERY_CONFIG.staleTime,
    gcTime: DEFAULT_QUERY_CONFIG.gcTime,
    retry: DEFAULT_QUERY_CONFIG.retry,
    retryDelay: DEFAULT_QUERY_CONFIG.retryDelay,
  })
}

export function useRuntimeRecommendation(format?: string, framework?: string) {
  return useQuery({
    queryKey: queryKeys.related(RESOURCE_KEY, 'recommend', 'search', format, framework),
    queryFn: () => runtimesApi.getRecommendation(format!, framework),
    enabled: !!format,
    staleTime: DEFAULT_QUERY_CONFIG.staleTime,
    gcTime: DEFAULT_QUERY_CONFIG.gcTime,
    retry: DEFAULT_QUERY_CONFIG.retry,
    retryDelay: DEFAULT_QUERY_CONFIG.retryDelay,
  })
}

export const useValidateRuntime = createResourceMutation(
  (runtime: Partial<ClusterServingRuntime>) => runtimesApi.validate(runtime),
  [] // No invalidation needed for validation
)

export function useCloneRuntime() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ name, newName }: { name: string; newName: string }) =>
      runtimesApi.clone(name, newName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.all(RESOURCE_KEY) })
    },
  })
}
