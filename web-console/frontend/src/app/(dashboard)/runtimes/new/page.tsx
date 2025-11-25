'use client'

import { useCreateRuntime } from '@/lib/hooks/useRuntimes'
import { useRouter } from 'next/navigation'
import { RuntimeForm } from '@/components/forms/runtime'

export default function CreateRuntimePage() {
  const router = useRouter()
  const createRuntime = useCreateRuntime()

  const handleSubmit = async (data: any) => {
    await createRuntime.mutateAsync(data)
    router.push('/runtimes')
  }

  return (
    <RuntimeForm
      mode="create"
      onSubmit={handleSubmit}
      backLink="/runtimes"
      backLinkText="Back to Runtimes"
    />
  )
}
