'use client'

import { useModel, useDeleteModel } from '@/lib/hooks/useModels'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { useState } from 'react'
import { ConfirmDeleteModal } from '@/components/ui/Modal'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import { ResourceRequirements } from '@/components/ui/ResourceRequirements'

export default function ModelDetailPage() {
  const params = useParams()
  const router = useRouter()
  const name = params.name as string
  const { data: model, isLoading, error } = useModel(name)
  const deleteModel = useDeleteModel()
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [showRawSpec, setShowRawSpec] = useState(false)

  const handleDelete = async () => {
    try {
      await deleteModel.mutateAsync(name)
      router.push('/models')
    } catch (err) {
      console.error('Failed to delete model:', err)
    }
  }

  if (isLoading) {
    return <LoadingState message="Loading model details..." />
  }

  if (error || !model) {
    return (
      <ErrorState
        error={error || new Error('Model not found')}
        backLink={{ href: '/models', label: 'Back to Models' }}
      />
    )
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between">
            <div>
              <Link
                href="/models"
                className="text-sm text-blue-600 hover:text-blue-800 mb-2 inline-block"
              >
                ← Back to Models
              </Link>
              <h1 className="text-3xl font-bold text-gray-900">{model.metadata.name}</h1>
              <p className="mt-1 text-sm text-gray-500">
                {model.kind || 'ClusterBaseModel'} Details
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
                <StatusBadge state={model.status?.state} />
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
              <dt className="text-sm font-medium text-gray-500">Scope</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {model.kind === 'ClusterBaseModel' ? (
                  <span className="inline-flex items-center rounded-full bg-blue-100 px-3 py-1 text-xs font-semibold text-blue-800">
                    Cluster-scoped
                  </span>
                ) : (
                  <span>Namespace: {model.metadata.namespace || 'default'}</span>
                )}
              </dd>
            </div>
          </div>

          {/* Nodes Ready */}
          {model.status?.nodesReady && model.status.nodesReady.length > 0 && (
            <div className="mt-6">
              <dt className="text-sm font-medium text-gray-500 mb-2">
                Ready on Nodes ({model.status.nodesReady.length})
              </dt>
              <dd className="flex flex-wrap gap-2">
                {model.status.nodesReady.map((node: string, index: number) => (
                  <span
                    key={index}
                    className="inline-flex items-center rounded-full bg-green-100 px-3 py-1 text-xs font-medium text-green-800"
                  >
                    {node}
                  </span>
                ))}
              </dd>
            </div>
          )}
        </div>

        {/* Model Specification */}
        <div className="mb-6 rounded-lg bg-white p-6 shadow">
          <h2 className="mb-4 text-lg font-medium text-gray-900">Model Specification</h2>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
            <div className="rounded-lg bg-gray-50 p-3">
              <dt className="text-xs text-gray-500 mb-1">Vendor</dt>
              <dd className="text-sm font-medium text-gray-900">{model.spec.vendor || '-'}</dd>
            </div>
            <div className="rounded-lg bg-gray-50 p-3">
              <dt className="text-xs text-gray-500 mb-1">Parameter Size</dt>
              <dd className="text-sm font-medium text-gray-900">{model.spec.modelParameterSize || '-'}</dd>
            </div>
            <div className="rounded-lg bg-gray-50 p-3">
              <dt className="text-xs text-gray-500 mb-1">Framework</dt>
              <dd className="text-sm font-medium text-gray-900">
                {model.spec.modelFramework?.name || '-'}
                {model.spec.modelFramework?.version && (
                  <span className="text-gray-500 font-normal"> ({model.spec.modelFramework.version})</span>
                )}
              </dd>
            </div>
            <div className="rounded-lg bg-gray-50 p-3">
              <dt className="text-xs text-gray-500 mb-1">Format</dt>
              <dd className="text-sm font-medium text-gray-900">
                {model.spec.modelFormat?.name || '-'}
                {model.spec.modelFormat?.version && (
                  <span className="text-gray-500 font-normal"> v{model.spec.modelFormat.version}</span>
                )}
              </dd>
            </div>

            {/* Model Configuration fields */}
            {model.spec.modelConfiguration?.architecture && (
              <div className="rounded-lg bg-gray-50 p-3">
                <dt className="text-xs text-gray-500 mb-1">Architecture</dt>
                <dd className="text-sm font-medium text-gray-900 truncate" title={model.spec.modelConfiguration.architecture}>
                  {model.spec.modelConfiguration.architecture}
                </dd>
              </div>
            )}
            {model.spec.modelConfiguration?.model_type && (
              <div className="rounded-lg bg-gray-50 p-3">
                <dt className="text-xs text-gray-500 mb-1">Model Type</dt>
                <dd className="text-sm font-medium text-gray-900">{model.spec.modelConfiguration.model_type}</dd>
              </div>
            )}
            {model.spec.modelConfiguration?.context_length && (
              <div className="rounded-lg bg-gray-50 p-3">
                <dt className="text-xs text-gray-500 mb-1">Context Length</dt>
                <dd className="text-sm font-medium text-gray-900">
                  {model.spec.modelConfiguration.context_length.toLocaleString()}
                </dd>
              </div>
            )}
            {model.spec.modelConfiguration?.torch_dtype && (
              <div className="rounded-lg bg-gray-50 p-3">
                <dt className="text-xs text-gray-500 mb-1">Data Type</dt>
                <dd className="text-sm font-medium text-gray-900">{model.spec.modelConfiguration.torch_dtype}</dd>
              </div>
            )}
            {model.spec.modelConfiguration?.transformers_version && (
              <div className="rounded-lg bg-gray-50 p-3">
                <dt className="text-xs text-gray-500 mb-1">Transformers</dt>
                <dd className="text-sm font-medium text-gray-900">v{model.spec.modelConfiguration.transformers_version}</dd>
              </div>
            )}
            {model.spec.modelConfiguration?.has_vision !== undefined && (
              <div className="rounded-lg bg-gray-50 p-3">
                <dt className="text-xs text-gray-500 mb-1">Vision Support</dt>
                <dd className="text-sm font-medium text-gray-900">
                  {model.spec.modelConfiguration.has_vision ? (
                    <span className="text-green-600">Yes</span>
                  ) : (
                    <span>No</span>
                  )}
                </dd>
              </div>
            )}
          </div>
        </div>

        {/* Storage Configuration */}
        {model.spec.storage && (
          <div className="mb-6 rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Storage Configuration</h2>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="rounded-lg bg-gray-50 p-3">
                <dt className="text-xs text-gray-500 mb-1">Storage URI</dt>
                <dd className="text-sm font-mono font-medium text-gray-900 break-all">
                  {model.spec.storage.storageUri || '-'}
                </dd>
              </div>
              {model.spec.storage.path && (
                <div className="rounded-lg bg-gray-50 p-3">
                  <dt className="text-xs text-gray-500 mb-1">Local Path</dt>
                  <dd className="text-sm font-mono font-medium text-gray-900 break-all">
                    {model.spec.storage.path}
                  </dd>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Resource Requirements */}
        <ResourceRequirements resources={model.spec.resources} />

        {/* Raw Specification - Collapsible */}
        <div className="rounded-lg bg-white p-6 shadow">
          <button
            onClick={() => setShowRawSpec(!showRawSpec)}
            className="flex w-full items-center justify-between text-left"
          >
            <h2 className="text-lg font-medium text-gray-900">Raw Specification</h2>
            <svg
              className={`h-5 w-5 transform text-gray-500 transition-transform ${
                showRawSpec ? 'rotate-180' : ''
              }`}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 9l-7 7-7-7"
              />
            </svg>
          </button>
          {showRawSpec && (
            <pre className="mt-4 overflow-x-auto rounded bg-gray-50 p-4 text-sm text-gray-800">
              {JSON.stringify(model, null, 2)}
            </pre>
          )}
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
