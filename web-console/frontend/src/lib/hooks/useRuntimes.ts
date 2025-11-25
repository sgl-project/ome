import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { runtimesApi } from '../api/runtimes'
import { ClusterServingRuntime } from '../types/runtime'
import { createResourceHooks, createResourceMutation } from './createResourceHooks'

// Create base CRUD hooks using the factory
const runtimeHooks = createResourceHooks<
  ClusterServingRuntime,
  Partial<ClusterServingRuntime>,
  Partial<ClusterServingRuntime>
>(runtimesApi, {
  resourceKey: 'runtimes',
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
      queryClient.invalidateQueries({ queryKey: ['runtimes'] })
    },
  })
}

// Runtime Intelligence Hooks (specialized operations not covered by factory)

export function useCompatibleRuntimes(format?: string, framework?: string) {
  return useQuery({
    queryKey: ['runtimes', 'compatible', format, framework],
    queryFn: () => runtimesApi.findCompatible(format!, framework),
    enabled: !!format,
  })
}

export function useRuntimeCompatibility(name?: string, format?: string, framework?: string) {
  return useQuery({
    queryKey: ['runtimes', name, 'compatibility', format, framework],
    queryFn: () => runtimesApi.checkCompatibility(name!, format!, framework),
    enabled: !!name && !!format,
  })
}

export function useRuntimeRecommendation(format?: string, framework?: string) {
  return useQuery({
    queryKey: ['runtimes', 'recommend', format, framework],
    queryFn: () => runtimesApi.getRecommendation(format!, framework),
    enabled: !!format,
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
      queryClient.invalidateQueries({ queryKey: ['runtimes'] })
    },
  })
}
