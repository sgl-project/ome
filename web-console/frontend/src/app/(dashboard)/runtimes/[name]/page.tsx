'use client'

import { useRuntime, useDeleteRuntime } from '@/lib/hooks/useRuntimes'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { useState } from 'react'
import { ConfirmDeleteModal } from '@/components/ui/Modal'

export default function RuntimeDetailPage() {
  const params = useParams()
  const router = useRouter()
  const name = params.name as string
  const { data: runtime, isLoading, error } = useRuntime(name)
  const deleteRuntime = useDeleteRuntime()
  const [showDeleteModal, setShowDeleteModal] = useState(false)

  const handleDelete = async () => {
    try {
      await deleteRuntime.mutateAsync(name)
      router.push('/runtimes')
    } catch (err) {
      console.error('Failed to delete runtime:', err)
    }
  }

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-lg">Loading runtime details...</div>
      </div>
    )
  }

  if (error || !runtime) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <div className="text-lg text-red-600 mb-4">
            Error: {error instanceof Error ? error.message : 'Runtime not found'}
          </div>
          <Link href="/runtimes" className="text-purple-600 hover:text-purple-800">
            ← Back to Runtimes
          </Link>
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
              <Link href="/runtimes" className="text-sm text-purple-600 hover:text-purple-800 mb-2 inline-block">
                ← Back to Runtimes
              </Link>
              <h1 className="text-3xl font-bold text-gray-900">{runtime.metadata.name}</h1>
              <p className="mt-1 text-sm text-gray-500">
                ClusterServingRuntime Details
              </p>
            </div>
            <div className="flex gap-3">
              <button
                onClick={() => router.push(`/runtimes/${name}/edit`)}
                className="rounded-lg bg-purple-600 px-4 py-2 text-sm font-medium text-white hover:bg-purple-700"
              >
                Edit Runtime
              </button>
              <button
                onClick={() => setShowDeleteModal(true)}
                className="rounded-lg border border-red-600 px-4 py-2 text-sm font-medium text-red-600 hover:bg-red-50"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        {/* Status */}
        <div className="mb-6 rounded-lg bg-white p-6 shadow">
          <h2 className="mb-4 text-lg font-medium text-gray-900">Status</h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div>
              <dt className="text-sm font-medium text-gray-500">State</dt>
              <dd className="mt-1">
                <span
                  className={`inline-flex rounded-full px-3 py-1 text-sm font-semibold ${
                    runtime.spec.disabled
                      ? 'bg-gray-100 text-gray-800'
                      : 'bg-green-100 text-green-800'
                  }`}
                >
                  {runtime.spec.disabled ? 'Disabled' : 'Active'}
                </span>
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Created</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {runtime.metadata.creationTimestamp
                  ? new Date(runtime.metadata.creationTimestamp).toLocaleString()
                  : 'Unknown'}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Multi-Model</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {runtime.spec.multiModel ? 'Yes' : 'No'}
              </dd>
            </div>
          </div>
        </div>

        {/* Supported Model Formats */}
        {runtime.spec.supportedModelFormats && runtime.spec.supportedModelFormats.length > 0 && (
          <div className="mb-6 rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Supported Model Formats</h2>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      Format Name
                    </th>
                    <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      Version
                    </th>
                    <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      Auto Select
                    </th>
                    <th className="px-4 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      Priority
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200 bg-white">
                  {runtime.spec.supportedModelFormats.map((format, idx) => (
                    <tr key={idx}>
                      <td className="whitespace-nowrap px-4 py-2 text-sm text-gray-900">
                        {format.name}
                      </td>
                      <td className="whitespace-nowrap px-4 py-2 text-sm text-gray-500">
                        {format.version || '-'}
                      </td>
                      <td className="whitespace-nowrap px-4 py-2 text-sm text-gray-500">
                        {format.autoSelect ? 'Yes' : 'No'}
                      </td>
                      <td className="whitespace-nowrap px-4 py-2 text-sm text-gray-500">
                        {format.priority ?? '-'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* Containers */}
        {runtime.spec.containers && runtime.spec.containers.length > 0 && (
          <div className="mb-6 rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Containers</h2>
            <div className="space-y-4">
              {runtime.spec.containers.map((container, idx) => (
                <div key={idx} className="rounded-lg border border-gray-200 p-4">
                  <h3 className="mb-2 font-medium text-gray-900">{container.name}</h3>
                  <dl className="grid grid-cols-1 gap-x-4 gap-y-2 sm:grid-cols-2">
                    <div>
                      <dt className="text-sm font-medium text-gray-500">Image</dt>
                      <dd className="mt-1 text-sm text-gray-900 break-all">{container.image}</dd>
                    </div>
                    {container.command && container.command.length > 0 && (
                      <div>
                        <dt className="text-sm font-medium text-gray-500">Command</dt>
                        <dd className="mt-1 text-sm text-gray-900">
                          {container.command.join(' ')}
                        </dd>
                      </div>
                    )}
                    {container.args && container.args.length > 0 && (
                      <div className="sm:col-span-2">
                        <dt className="text-sm font-medium text-gray-500">Arguments</dt>
                        <dd className="mt-1 text-sm text-gray-900">
                          {container.args.join(' ')}
                        </dd>
                      </div>
                    )}
                    {container.env && container.env.length > 0 && (
                      <div className="sm:col-span-2">
                        <dt className="text-sm font-medium text-gray-500">Environment Variables</dt>
                        <dd className="mt-1">
                          <div className="max-h-40 overflow-y-auto">
                            {container.env.map((env, envIdx) => (
                              <div key={envIdx} className="text-sm text-gray-900">
                                <span className="font-medium">{env.name}</span>: {env.value || '-'}
                              </div>
                            ))}
                          </div>
                        </dd>
                      </div>
                    )}
                  </dl>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Protocol Versions */}
        {runtime.spec.protocolVersions && runtime.spec.protocolVersions.length > 0 && (
          <div className="mb-6 rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Protocol Versions</h2>
            <div className="flex flex-wrap gap-2">
              {runtime.spec.protocolVersions.map((version, idx) => (
                <span
                  key={idx}
                  className="inline-flex rounded-full bg-purple-100 px-3 py-1 text-sm font-semibold text-purple-800"
                >
                  {version}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Built-in Adapter */}
        {runtime.spec.builtInAdapter && (
          <div className="mb-6 rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Built-in Adapter</h2>
            <dl className="grid grid-cols-1 gap-x-4 gap-y-4 sm:grid-cols-2">
              {runtime.spec.builtInAdapter.serverType && (
                <div>
                  <dt className="text-sm font-medium text-gray-500">Server Type</dt>
                  <dd className="mt-1 text-sm text-gray-900">
                    {runtime.spec.builtInAdapter.serverType}
                  </dd>
                </div>
              )}
              {runtime.spec.builtInAdapter.runtimeManagementPort && (
                <div>
                  <dt className="text-sm font-medium text-gray-500">Management Port</dt>
                  <dd className="mt-1 text-sm text-gray-900">
                    {runtime.spec.builtInAdapter.runtimeManagementPort}
                  </dd>
                </div>
              )}
              {runtime.spec.builtInAdapter.memBufferBytes && (
                <div>
                  <dt className="text-sm font-medium text-gray-500">Memory Buffer</dt>
                  <dd className="mt-1 text-sm text-gray-900">
                    {runtime.spec.builtInAdapter.memBufferBytes} bytes
                  </dd>
                </div>
              )}
              {runtime.spec.builtInAdapter.modelLoadingTimeoutMillis && (
                <div>
                  <dt className="text-sm font-medium text-gray-500">Loading Timeout</dt>
                  <dd className="mt-1 text-sm text-gray-900">
                    {runtime.spec.builtInAdapter.modelLoadingTimeoutMillis} ms
                  </dd>
                </div>
              )}
            </dl>
          </div>
        )}

        {/* Raw Specification */}
        <div className="rounded-lg bg-white p-6 shadow">
          <h2 className="mb-4 text-lg font-medium text-gray-900">Raw Specification</h2>
          <pre className="overflow-x-auto rounded bg-gray-50 p-4 text-sm text-gray-800">
            {JSON.stringify(runtime, null, 2)}
          </pre>
        </div>
      </main>

      {/* Delete Confirmation Modal */}
      <ConfirmDeleteModal
        isOpen={showDeleteModal}
        onClose={() => setShowDeleteModal(false)}
        onConfirm={handleDelete}
        resourceName={runtime.metadata.name}
        resourceType="runtime"
        isDeleting={deleteRuntime.isPending}
      />
    </div>
  )
}
