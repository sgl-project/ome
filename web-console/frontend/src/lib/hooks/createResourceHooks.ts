import { useQuery, useMutation, useQueryClient, QueryKey } from '@tanstack/react-query'
import { ListResponse } from '../types/common'

/**
 * Generic API interface for standard resource operations.
 * APIs may have additional methods beyond these core operations.
 *
 * This interface is intentionally flexible to accommodate different API patterns:
 * - Cluster-scoped resources (no namespace in get/update/delete)
 * - Namespace-scoped resources (namespace required or optional)
 */
export interface ResourceApi<T, CreateInput = Partial<T>, UpdateInput = Partial<T>> {
  list: (namespace?: string) => Promise<ListResponse<T>>
  get: (name: string, namespace?: string) => Promise<T>
  create: (data: CreateInput) => Promise<T>
  update: (name: string, data: UpdateInput) => Promise<T>
  delete: (name: string) => Promise<void>
}

export interface ResourceHooksOptions {
  /** Query key prefix, e.g., 'models', 'services', 'runtimes' */
  resourceKey: string
  /** Default stale time in ms (default: 30000) */
  staleTime?: number
  /** Whether list queries should include namespace in query key */
  namespaceInListKey?: boolean
}

/**
 * Creates a set of standard React Query hooks for a resource type.
 *
 * This factory eliminates duplicate hook code across useModels, useServices,
 * useRuntimes, etc. Each resource gets:
 * - useList: Fetch list with optional namespace filter
 * - useGet: Fetch single resource by name
 * - useCreate: Create new resource
 * - useUpdate: Update existing resource
 * - useDelete: Delete resource
 *
 * @example
 * ```ts
 * // Create hooks for models
 * const modelHooks = createResourceHooks<ClusterBaseModel>(modelsApi, {
 *   resourceKey: 'models',
 * })
 *
 * export const useModels = modelHooks.useList
 * export const useModel = modelHooks.useGet
 * export const useCreateModel = modelHooks.useCreate
 * export const useUpdateModel = modelHooks.useUpdate
 * export const useDeleteModel = modelHooks.useDelete
 * ```
 */
export function createResourceHooks<T, CreateInput = Partial<T>, UpdateInput = Partial<T>>(
  api: ResourceApi<T, CreateInput, UpdateInput>,
  options: ResourceHooksOptions
) {
  const { resourceKey, staleTime = 30000, namespaceInListKey = true } = options

  /**
   * Build a query key for the resource list
   */
  function getListQueryKey(namespace?: string): QueryKey {
    if (namespaceInListKey && namespace) {
      return [resourceKey, { namespace }]
    }
    return [resourceKey]
  }

  /**
   * Build a query key for a single resource
   */
  function getItemQueryKey(name: string): QueryKey {
    return [resourceKey, name]
  }

  return {
    /**
     * Fetch list of resources, optionally filtered by namespace
     */
    useList: (namespace?: string) => {
      return useQuery({
        queryKey: getListQueryKey(namespace),
        queryFn: () => api.list(namespace),
        staleTime,
      })
    },

    /**
     * Fetch a single resource by name
     */
    useGet: (name: string) => {
      return useQuery({
        queryKey: getItemQueryKey(name),
        queryFn: () => api.get(name),
        enabled: !!name,
        staleTime,
      })
    },

    /**
     * Create a new resource
     */
    useCreate: () => {
      const queryClient = useQueryClient()
      return useMutation({
        mutationFn: (data: CreateInput) => api.create(data),
        onSuccess: () => {
          queryClient.invalidateQueries({ queryKey: [resourceKey] })
        },
      })
    },

    /**
     * Update an existing resource
     */
    useUpdate: () => {
      const queryClient = useQueryClient()
      return useMutation({
        mutationFn: ({ name, data }: { name: string; data: UpdateInput }) => api.update(name, data),
        onSuccess: () => {
          queryClient.invalidateQueries({ queryKey: [resourceKey] })
        },
      })
    },

    /**
     * Delete a resource
     */
    useDelete: () => {
      const queryClient = useQueryClient()
      return useMutation({
        mutationFn: (name: string) => api.delete(name),
        onSuccess: () => {
          queryClient.invalidateQueries({ queryKey: [resourceKey] })
        },
      })
    },

    /**
     * Get the query key builders for advanced use cases
     * (e.g., prefetching, direct cache manipulation)
     */
    getListQueryKey,
    getItemQueryKey,

    /**
     * Invalidate all queries for this resource type
     */
    useInvalidate: () => {
      const queryClient = useQueryClient()
      return () => queryClient.invalidateQueries({ queryKey: [resourceKey] })
    },
  }
}

/**
 * Extended factory for resources with additional specialized operations
 * Can be combined with createResourceHooks
 */
export function createResourceMutation<TData, TVariables>(
  mutationFn: (variables: TVariables) => Promise<TData>,
  invalidateKeys: string[]
) {
  return function useMutationHook() {
    const queryClient = useQueryClient()
    return useMutation({
      mutationFn,
      onSuccess: () => {
        invalidateKeys.forEach((key) => {
          queryClient.invalidateQueries({ queryKey: [key] })
        })
      },
    })
  }
}
