import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { servicesApi } from '../api/services'
import { InferenceService } from '../types/service'
import { DEFAULT_QUERY_CONFIG, queryKeys, createResourceHooks } from './createResourceHooks'

const RESOURCE_KEY = 'services'

// Create base CRUD hooks using the factory
const serviceHooks = createResourceHooks<
  InferenceService,
  Partial<InferenceService>,
  Partial<InferenceService>
>(servicesApi, {
  resourceKey: RESOURCE_KEY,
})

// Export standard list hook from factory
export const useServices = serviceHooks.useList

// Export create and delete from factory
export const useCreateService = serviceHooks.useCreate
export const useDeleteService = serviceHooks.useDelete

// Custom useService to support namespace parameter in get
export function useService(name: string, namespace?: string) {
  return useQuery({
    queryKey: queryKeys.detail(RESOURCE_KEY, name, namespace),
    queryFn: () => servicesApi.get(name, namespace),
    enabled: !!name,
    staleTime: DEFAULT_QUERY_CONFIG.staleTime,
    gcTime: DEFAULT_QUERY_CONFIG.gcTime,
    retry: DEFAULT_QUERY_CONFIG.retry,
    retryDelay: DEFAULT_QUERY_CONFIG.retryDelay,
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
      queryClient.invalidateQueries({ queryKey: queryKeys.all(RESOURCE_KEY) })
    },
  })
}
