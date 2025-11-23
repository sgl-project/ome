'use client'

import { useModel, useDeleteModel } from '@/lib/hooks/useModels'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { useState } from 'react'
import { ConfirmDeleteModal } from '@/components/ui/Modal'

export default function ModelDetailPage() {
  const params = useParams()
  const router = useRouter()
  const name = params.name as string
  const { data: model, isLoading, error } = useModel(name)
  const deleteModel = useDeleteModel()
  const [showDeleteModal, setShowDeleteModal] = useState(false)

  const handleDelete = async () => {
    try {
      await deleteModel.mutateAsync(name)
      router.push('/models')
    } catch (err) {
      console.error('Failed to delete model:', err)
    }
  }

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-lg">Loading model details...</div>
      </div>
    )
  }

  if (error || !model) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <div className="text-lg text-red-600 mb-4">
            Error: {error instanceof Error ? error.message : 'Model not found'}
          </div>
          <Link href="/models" className="text-blue-600 hover:text-blue-800">
            ← Back to Models
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
              <Link href="/models" className="text-sm text-blue-600 hover:text-blue-800 mb-2 inline-block">
                ← Back to Models
              </Link>
              <h1 className="text-3xl font-bold text-gray-900">{model.metadata.name}</h1>
              <p className="mt-1 text-sm text-gray-500">
                ClusterBaseModel Details
              </p>
            </div>
            <div className="flex gap-3">
              <button
                onClick={() => router.push(`/models/${name}/edit`)}
                className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
              >
                Edit Model
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
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Created</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {model.metadata.creationTimestamp
                  ? new Date(model.metadata.creationTimestamp).toLocaleString()
                  : 'Unknown'}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Namespace</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {model.metadata.namespace || 'default'}
              </dd>
            </div>
          </div>
        </div>

        {/* Model Specification */}
        <div className="mb-6 rounded-lg bg-white p-6 shadow">
          <h2 className="mb-4 text-lg font-medium text-gray-900">Model Specification</h2>
          <dl className="grid grid-cols-1 gap-x-4 gap-y-6 sm:grid-cols-2">
            <div>
              <dt className="text-sm font-medium text-gray-500">Vendor</dt>
              <dd className="mt-1 text-sm text-gray-900">{model.spec.vendor || '-'}</dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Parameter Size</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {model.spec.modelParameterSize || '-'}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Framework</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {model.spec.modelFramework?.name || '-'}
                {model.spec.modelFramework?.version && ` (${model.spec.modelFramework.version})`}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Model Format</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {model.spec.modelFormat?.name || '-'}
                {model.spec.modelFormat?.version && ` v${model.spec.modelFormat.version}`}
              </dd>
            </div>
          </dl>
        </div>

        {/* Storage Configuration */}
        {model.spec.storage && (
          <div className="mb-6 rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Storage Configuration</h2>
            <dl className="grid grid-cols-1 gap-x-4 gap-y-6 sm:grid-cols-2">
              <div>
                <dt className="text-sm font-medium text-gray-500">Storage URI</dt>
                <dd className="mt-1 text-sm text-gray-900 break-all">
                  {model.spec.storage.storageUri || '-'}
                </dd>
              </div>
              {model.spec.storage.path && (
                <div>
                  <dt className="text-sm font-medium text-gray-500">Path</dt>
                  <dd className="mt-1 text-sm text-gray-900">{model.spec.storage.path}</dd>
                </div>
              )}
            </dl>
          </div>
        )}

        {/* Resource Requirements */}
        {model.spec.resources && (
          <div className="mb-6 rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Resource Requirements</h2>
            <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
              {model.spec.resources.requests && (
                <div>
                  <h3 className="mb-2 text-sm font-medium text-gray-700">Requests</h3>
                  <dl className="space-y-2">
                    {Object.entries(model.spec.resources.requests).map(([key, value]) => (
                      <div key={key} className="flex justify-between">
                        <dt className="text-sm text-gray-500">{key}:</dt>
                        <dd className="text-sm text-gray-900">{value}</dd>
                      </div>
                    ))}
                  </dl>
                </div>
              )}
              {model.spec.resources.limits && (
                <div>
                  <h3 className="mb-2 text-sm font-medium text-gray-700">Limits</h3>
                  <dl className="space-y-2">
                    {Object.entries(model.spec.resources.limits).map(([key, value]) => (
                      <div key={key} className="flex justify-between">
                        <dt className="text-sm text-gray-500">{key}:</dt>
                        <dd className="text-sm text-gray-900">{value}</dd>
                      </div>
                    ))}
                  </dl>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Raw YAML */}
        <div className="rounded-lg bg-white p-6 shadow">
          <h2 className="mb-4 text-lg font-medium text-gray-900">Raw Specification</h2>
          <pre className="overflow-x-auto rounded bg-gray-50 p-4 text-sm text-gray-800">
            {JSON.stringify(model, null, 2)}
          </pre>
        </div>
      </main>

      {/* Delete Confirmation Modal */}
      <ConfirmDeleteModal
        isOpen={showDeleteModal}
        onClose={() => setShowDeleteModal(false)}
        onConfirm={handleDelete}
        resourceName={model.metadata.name}
        resourceType="model"
        isDeleting={deleteModel.isPending}
      />
    </div>
  )
}
