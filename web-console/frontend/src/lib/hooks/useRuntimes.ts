import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { runtimesApi } from '../api/runtimes'
import { ClusterServingRuntime } from '../types/runtime'

export function useRuntimes() {
  return useQuery({
    queryKey: ['runtimes'],
    queryFn: runtimesApi.list,
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
