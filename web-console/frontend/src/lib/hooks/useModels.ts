import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { modelsApi } from '../api/models'
import { ClusterBaseModel } from '../types/model'

export function useModels(namespace?: string) {
  return useQuery({
    queryKey: namespace ? ['models', { namespace }] : ['models'],
    queryFn: () => modelsApi.list(namespace),
  })
}

export function useModel(name: string) {
  return useQuery({
    queryKey: ['models', name],
    queryFn: () => modelsApi.get(name),
    enabled: !!name,
  })
}

export function useCreateModel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (requestBody: { model: Partial<ClusterBaseModel>; huggingfaceToken?: string }) =>
      modelsApi.create(requestBody),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
    },
  })
}

export function useUpdateModel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ name, model }: { name: string; model: Partial<ClusterBaseModel> }) =>
      modelsApi.update(name, model),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
    },
  })
}

export function useDeleteModel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (name: string) => modelsApi.delete(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['models'] })
    },
  })
}
