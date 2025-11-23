import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { servicesApi } from '../api/services'
import { InferenceService } from '../types/service'

export function useServices(namespace?: string) {
  return useQuery({
    queryKey: ['services', namespace],
    queryFn: () => servicesApi.list(namespace),
  })
}

export function useService(name: string, namespace?: string) {
  return useQuery({
    queryKey: ['services', name, namespace],
    queryFn: () => servicesApi.get(name, namespace),
    enabled: !!name,
  })
}

export function useCreateService() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (service: Partial<InferenceService>) => servicesApi.create(service),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['services'] })
    },
  })
}

export function useUpdateService() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ name, service }: { name: string; service: Partial<InferenceService> }) =>
      servicesApi.update(name, service),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['services'] })
    },
  })
}

export function useDeleteService() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (name: string) => servicesApi.delete(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['services'] })
    },
  })
}
