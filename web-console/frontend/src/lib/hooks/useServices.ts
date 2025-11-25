import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { servicesApi } from '../api/services'
import { InferenceService } from '../types/service'
import { createResourceHooks } from './createResourceHooks'

// Create base CRUD hooks using the factory
const serviceHooks = createResourceHooks<
  InferenceService,
  Partial<InferenceService>,
  Partial<InferenceService>
>(servicesApi, {
  resourceKey: 'services',
  namespaceInListKey: false, // services uses ['services', namespace] not ['services', { namespace }]
})

// Export hooks that match the factory signatures
export const useServices = (namespace?: string) => {
  // Custom query key format to match existing behavior
  return useQuery({
    queryKey: ['services', namespace],
    queryFn: () => servicesApi.list(namespace),
  })
}

export const useCreateService = serviceHooks.useCreate
export const useDeleteService = serviceHooks.useDelete

// Custom useService to support namespace parameter in get
export function useService(name: string, namespace?: string) {
  return useQuery({
    queryKey: ['services', name, namespace],
    queryFn: () => servicesApi.get(name, namespace),
    enabled: !!name,
  })
}

// Custom useUpdateService to match existing API signature
// (existing code passes { name, service } instead of { name, data })
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
