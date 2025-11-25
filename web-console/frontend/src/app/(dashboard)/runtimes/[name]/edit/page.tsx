'use client'

import { useRuntime, useUpdateRuntime } from '@/lib/hooks/useRuntimes'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { RuntimeForm } from '@/components/forms/runtime'

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
      <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-50 to-slate-100">
        <div className="text-center">
          <div className="mb-4 text-xl font-semibold text-red-600">Runtime not found</div>
          <Link
            href="/runtimes"
            className="text-purple-600 hover:text-purple-800 transition-colors"
          >
            ← Back to Runtimes
          </Link>
        </div>
      </div>
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
