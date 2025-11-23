import { useQuery } from '@tanstack/react-query'
import { namespacesApi } from '../api/namespaces'

export function useNamespaces() {
  return useQuery({
    queryKey: ['namespaces'],
    queryFn: namespacesApi.list,
  })
}

export function useNamespace(name: string) {
  return useQuery({
    queryKey: ['namespaces', name],
    queryFn: () => namespacesApi.get(name),
    enabled: !!name,
  })
}
