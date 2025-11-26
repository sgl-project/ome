'use client'

import { useRuntime, useUpdateRuntime } from '@/lib/hooks/useRuntimes'
import { useParams, useRouter } from 'next/navigation'
import { RuntimeForm } from '@/components/forms/runtime'
import { ErrorState } from '@/components/ui/ErrorState'

export default function EditRuntimePage() {
  const params = useParams()
  const router = useRouter()
  const name = params.name as string
  const { data: runtime, isLoading: isLoadingRuntime } = useRuntime(name)
  const updateRuntime = useUpdateRuntime()

  const handleSubmit = async (data: any) => {
    await updateRuntime.mutateAsync({ name, runtime: data })
    router.push(`/runtimes/${name}`)
  }

  if (!isLoadingRuntime && !runtime) {
    return (
      <ErrorState
        error={new Error('Runtime not found')}
        backLink={{ href: '/runtimes', label: 'Back to Runtimes' }}
      />
    )
  }

  return (
    <RuntimeForm
      mode="edit"
      initialData={runtime}
      onSubmit={handleSubmit}
      isLoading={isLoadingRuntime}
      backLink={`/runtimes/${name}`}
      backLinkText="Back to Runtime Details"
    />
  )
}
