import { useQuery } from '@tanstack/react-query'
import { acceleratorsApi } from '../api/accelerators'

export function useAccelerators() {
  return useQuery({
    queryKey: ['accelerators'],
    queryFn: acceleratorsApi.list,
  })
}

export function useAccelerator(name: string) {
  return useQuery({
    queryKey: ['accelerators', name],
    queryFn: () => acceleratorsApi.get(name),
    enabled: !!name,
  })
}
