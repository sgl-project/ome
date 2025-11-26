import { useQuery, useMutation, useQueryClient, QueryKey } from '@tanstack/react-query'
import { ListResponse } from '../types/common'
import { isApiError } from '../types/api'

export const DEFAULT_QUERY_CONFIG = {
  staleTime: 30000,
  gcTime: 300000,
  retry: (failureCount: number, error: unknown): boolean => {
    if (isApiError(error) && error.status >= 400 && error.status < 500) return false
    return failureCount < 3
  },
  retryDelay: (attempt: number): number => Math.min(1000 * 2 ** attempt, 30000),
}

export const queryKeys = {
  list: (key: string, ns?: string): QueryKey => (ns ? [key, { namespace: ns }] : [key]),
  detail: (key: string, name: string, ns?: string): QueryKey =>
    ns ? [key, 'detail', name, { namespace: ns }] : [key, 'detail', name],
  all: (key: string): QueryKey => [key],
  related: (key: string, name: string, relation: string, ...args: unknown[]): QueryKey => [
    key,
    name,
    relation,
    ...args.filter((a) => a !== undefined),
  ],
}

export interface ResourceApi<T, C = Partial<T>, U = Partial<T>> {
  list: (namespace?: string) => Promise<ListResponse<T>>
  get: (name: string, namespace?: string) => Promise<T>
  create: (data: C) => Promise<T>
  update: (name: string, data: U) => Promise<T>
  delete: (name: string) => Promise<void>
}

export function createResourceHooks<T, C = Partial<T>, U = Partial<T>>(
  api: ResourceApi<T, C, U>,
  options: { resourceKey: string; staleTime?: number; gcTime?: number }
) {
  const {
    resourceKey,
    staleTime = DEFAULT_QUERY_CONFIG.staleTime,
    gcTime = DEFAULT_QUERY_CONFIG.gcTime,
  } = options

  return {
    useList: (namespace?: string) =>
      useQuery({
        queryKey: queryKeys.list(resourceKey, namespace),
        queryFn: () => api.list(namespace),
        staleTime,
        gcTime,
        retry: DEFAULT_QUERY_CONFIG.retry,
      }),

    useGet: (name: string, namespace?: string) =>
      useQuery({
        queryKey: queryKeys.detail(resourceKey, name, namespace),
        queryFn: () => api.get(name, namespace),
        enabled: !!name,
        staleTime,
        gcTime,
        retry: DEFAULT_QUERY_CONFIG.retry,
      }),

    useCreate: () => {
      const qc = useQueryClient()
      return useMutation({
        mutationFn: (data: C) => api.create(data),
        onSuccess: () => qc.invalidateQueries({ queryKey: [resourceKey] }),
      })
    },

    useUpdate: () => {
      const qc = useQueryClient()
      return useMutation({
        mutationFn: ({ name, data }: { name: string; data: U }) => api.update(name, data),
        onSuccess: () => qc.invalidateQueries({ queryKey: [resourceKey] }),
      })
    },

    useDelete: () => {
      const qc = useQueryClient()
      return useMutation({
        mutationFn: (name: string) => api.delete(name),
        onSuccess: () => qc.invalidateQueries({ queryKey: [resourceKey] }),
      })
    },
  }
}

export function createResourceMutation<T, V>(
  mutationFn: (v: V) => Promise<T>,
  invalidateKeys: string[]
) {
  return function useMutationHook() {
    const qc = useQueryClient()
    return useMutation({
      mutationFn,
      onSuccess: () => invalidateKeys.forEach((k) => qc.invalidateQueries({ queryKey: [k] })),
    })
  }
}
