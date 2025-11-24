import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { runtimesApi } from '../api/runtimes'
import { ClusterServingRuntime } from '../types/runtime'

export function useRuntimes(namespace?: string) {
  return useQuery({
    queryKey: namespace ? ['runtimes', { namespace }] : ['runtimes'],
    queryFn: () => runtimesApi.list(namespace),
  })
}

export function useRuntime(name: string) {
  return useQuery({
    queryKey: ['runtimes', name],
    queryFn: () => runtimesApi.get(name),
    enabled: !!name,
  })
}

export function useCreateRuntime() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (runtime: Partial<ClusterServingRuntime>) => runtimesApi.create(runtime),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['runtimes'] })
    },
  })
}

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

export function useDeleteRuntime() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (name: string) => runtimesApi.delete(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['runtimes'] })
    },
  })
}

// Runtime Intelligence Hooks

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

export function useValidateRuntime() {
  return useMutation({
    mutationFn: (runtime: Partial<ClusterServingRuntime>) => runtimesApi.validate(runtime),
  })
}

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
