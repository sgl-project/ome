'use client'

import { useModels } from '@/lib/hooks/useModels'
import { useServerEvents } from '@/hooks/useServerEvents'
import { useQueryClient } from '@tanstack/react-query'
import Link from 'next/link'

export default function ModelsPage() {
  const { data, isLoading, error } = useModels()
  const queryClient = useQueryClient()

  // Connect to SSE for real-time updates
  useServerEvents({
    onEvent: (event) => {
      if (event.resource === 'models') {
        // Invalidate models query to trigger refetch
        queryClient.invalidateQueries({ queryKey: ['models'] })
      }
    },
    onConnected: () => {
      console.log('Connected to real-time updates')
    },
  })

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-lg">Loading models...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-lg text-red-600">
          Error: {error instanceof Error ? error.message : 'Failed to load models'}
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-3xl font-bold text-gray-900">Models</h1>
              <p className="mt-1 text-sm text-gray-500">
                Manage your ClusterBaseModel and BaseModel resources
              </p>
            </div>
            <div className="flex gap-3">
              <Link
                href="/models/import"
                className="rounded-lg border border-blue-600 px-4 py-2 text-sm font-medium text-blue-600 hover:bg-blue-50"
              >
                Import from HuggingFace
              </Link>
              <Link
                href="/models/new"
                className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
              >
                Create Model
              </Link>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        {/* Stats */}
        <div className="mb-6 grid grid-cols-1 gap-6 sm:grid-cols-4">
          <div className="overflow-hidden rounded-lg bg-white shadow">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-gray-500">
                    Total Models
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-gray-900">
                    {data?.total || 0}
                  </dd>
                </div>
              </div>
            </div>
          </div>

          <div className="overflow-hidden rounded-lg bg-white shadow">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-gray-500">
                    Ready
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-green-600">
                    {data?.items.filter((m) => m.status?.state === 'Ready').length || 0}
                  </dd>
                </div>
              </div>
            </div>
          </div>

          <div className="overflow-hidden rounded-lg bg-white shadow">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-gray-500">
                    In Transit
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-yellow-600">
                    {data?.items.filter((m) => m.status?.state === 'In_Transit').length || 0}
                  </dd>
                </div>
              </div>
            </div>
          </div>

          <div className="overflow-hidden rounded-lg bg-white shadow">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-gray-500">
                    Failed
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-red-600">
                    {data?.items.filter((m) => m.status?.state === 'Failed').length || 0}
                  </dd>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Models Table */}
        <div className="overflow-hidden rounded-lg bg-white shadow">
          <div className="border-b border-gray-200 px-4 py-5 sm:px-6">
            <h3 className="text-lg font-medium leading-6 text-gray-900">
              All Models
            </h3>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    Name
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    Vendor
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    Framework
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    Size
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    Status
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                    Created
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 bg-white">
                {data?.items.map((model) => (
                  <tr key={model.metadata.name} className="hover:bg-gray-50">
                    <td className="whitespace-nowrap px-6 py-4 text-sm font-medium">
                      <Link
                        href={`/models/${model.metadata.name}`}
                        className="text-blue-600 hover:text-blue-900"
                      >
                        {model.metadata.name}
                      </Link>
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                      {model.spec.vendor || '-'}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                      {model.spec.modelFramework?.name || '-'}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                      {model.spec.modelParameterSize || '-'}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4">
                      <span
                        className={`inline-flex rounded-full px-2 text-xs font-semibold leading-5 ${
                          model.status?.state === 'Ready'
                            ? 'bg-green-100 text-green-800'
                            : model.status?.state === 'Failed'
                            ? 'bg-red-100 text-red-800'
                            : model.status?.state === 'In_Transit'
                            ? 'bg-yellow-100 text-yellow-800'
                            : 'bg-gray-100 text-gray-800'
                        }`}
                      >
                        {model.status?.state || 'Unknown'}
                      </span>
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                      {model.metadata.creationTimestamp
                        ? new Date(model.metadata.creationTimestamp).toLocaleDateString()
                        : '-'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </main>
    </div>
  )
}
