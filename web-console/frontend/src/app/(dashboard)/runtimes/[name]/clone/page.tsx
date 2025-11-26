'use client'

import { useRuntime, useCreateRuntime } from '@/lib/hooks/useRuntimes'
import { useParams, useRouter } from 'next/navigation'
import { RuntimeForm } from '@/components/forms/runtime'
import { ErrorState } from '@/components/ui/ErrorState'
import { useMemo } from 'react'
import type { ClusterServingRuntime } from '@/lib/types/runtime'

export default function CloneRuntimePage() {
  const params = useParams()
  const router = useRouter()
  const sourceName = params.name as string
  const { data: runtime, isLoading: isLoadingRuntime } = useRuntime(sourceName)
  const createRuntime = useCreateRuntime()

  // Prepare cloned data - clear the name so user must enter a new one
  const clonedData = useMemo((): ClusterServingRuntime | undefined => {
    if (!runtime) return undefined

    return {
      ...runtime,
      metadata: {
        ...runtime.metadata,
        name: '', // Clear name for user to enter new one
        // Remove system-managed fields
        uid: undefined,
        resourceVersion: undefined,
        creationTimestamp: undefined,
      },
    }
  }, [runtime])

  const handleSubmit = async (data: any) => {
    await createRuntime.mutateAsync(data)
    router.push('/runtimes')
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
      mode="create"
      initialData={clonedData}
      onSubmit={handleSubmit}
      isLoading={isLoadingRuntime}
      backLink={`/runtimes/${sourceName}`}
      backLinkText={`Back to ${sourceName}`}
      cloneFrom={sourceName}
    />
  )
}
