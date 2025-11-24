import { useQuery } from '@tanstack/react-query'
import { namespacesApi } from '../api/namespaces'

export function useNamespaces() {
  return useQuery({
    queryKey: ['namespaces'],
    queryFn: () => namespacesApi.list(),
  })
}
