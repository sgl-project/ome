'use client'

import { useModel, useDeleteModel, useModelProgress } from '@/lib/hooks/useModels'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { useState } from 'react'
import { ConfirmDeleteModal } from '@/components/ui/Modal'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import { ResourceRequirements } from '@/components/ui/ResourceRequirements'
import { KeyValueList } from '@/components/ui/KeyValueList'
import { NodeList } from '@/components/ui/NodeList'
import { SpecCard } from '@/components/ui/SpecCard'
import { Icons } from '@/components/ui/Icons'
import { exportAsYaml } from '@/lib/utils'

// Helper to format bytes
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// Helper to format time duration
function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.ceil(seconds)}s`
  if (seconds < 3600) {
    const mins = Math.floor(seconds / 60)
    const secs = Math.ceil(seconds % 60)
    return `${mins}m ${secs}s`
  }
  const hours = Math.floor(seconds / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  return `${hours}h ${mins}m`
}

export default function ModelDetailPage() {
  const params = useParams()
  const router = useRouter()
  const name = params.name as string
  const { data: model, isLoading, error } = useModel(name)
  const deleteModel = useDeleteModel()
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [showRawSpec, setShowRawSpec] = useState(false)

  // Determine if we should poll for progress (only when model is downloading)
  const isDownloading =
    model?.status?.state === 'In_Transit' || model?.status?.state === 'Importing'

  // Use ConfigMap-based progress API for real-time updates
  const { data: progressData } = useModelProgress(
    name,
    isDownloading // Only poll when downloading
  )

  const handleDelete = async () => {
    try {
      await deleteModel.mutateAsync(name)
      router.push('/models')
    } catch (err) {
      console.error('Failed to delete model:', err)
    }
  }

  const handleExportYaml = () => {
    if (model) {
      exportAsYaml(model, `${model.metadata.name}.yaml`)
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
                onClick={handleExportYaml}
                className="inline-flex items-center gap-2 rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
              >
                <Icons.downloadFile size="sm" />
                Export YAML
              </button>
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

          {/* Download Progress - shown when downloading */}
          {isDownloading &&
            progressData &&
            progressData.progress.length > 0 &&
            (() => {
              // Calculate aggregate stats
              const avgPercentage =
                progressData.progress.reduce((sum, p) => sum + p.percentage, 0) /
                progressData.progress.length
              const minPercentage = Math.min(...progressData.progress.map((p) => p.percentage))
              const totalSpeed = progressData.progress.reduce((sum, p) => sum + p.bytesPerSecond, 0)
              const maxEta = Math.max(...progressData.progress.map((p) => p.remainingTime))
              const firstProgress = progressData.progress[0]

              return (
                <div className="mt-6 pt-6 border-t border-gray-200">
                  <div className="flex items-center justify-between mb-4">
                    <h3 className="text-sm font-medium text-gray-900">Download Progress</h3>
                    <div className="flex items-center gap-4 text-xs text-gray-500">
                      <span>{progressData.progress.length} nodes</span>
                      <span className="text-blue-600 font-medium">
                        {formatBytes(totalSpeed)}/s total
                      </span>
                      {maxEta > 0 && <span>ETA: {formatDuration(maxEta)}</span>}
                    </div>
                  </div>

                  {/* Aggregate progress bar */}
                  <div className="mb-4 p-3 bg-gray-50 rounded-lg">
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-sm font-medium text-gray-700">Overall Progress</span>
                      <span className="text-sm font-semibold text-blue-600">
                        {avgPercentage.toFixed(1)}%
                      </span>
                    </div>
                    <div className="relative h-3 bg-gray-200 rounded-full overflow-hidden">
                      <div
                        className="absolute inset-y-0 left-0 bg-gradient-to-r from-blue-500 to-blue-600 rounded-full transition-all duration-500"
                        style={{ width: `${Math.min(avgPercentage, 100)}%` }}
                      />
                      {/* Min progress indicator */}
                      <div
                        className="absolute inset-y-0 w-0.5 bg-blue-800 opacity-50"
                        style={{ left: `${Math.min(minPercentage, 100)}%` }}
                        title={`Slowest node: ${minPercentage.toFixed(1)}%`}
                      />
                    </div>
                    <div className="flex justify-between mt-2 text-xs text-gray-500">
                      <span>
                        {formatBytes(firstProgress?.completedBytes || 0)} /{' '}
                        {formatBytes(firstProgress?.totalBytes || 0)} per node
                      </span>
                      <span>Min: {minPercentage.toFixed(1)}%</span>
                    </div>
                  </div>

                  {/* Per-node progress */}
                  <div className="space-y-2 max-h-64 overflow-y-auto">
                    {progressData.progress
                      .sort((a, b) => a.percentage - b.percentage) // Show slowest first
                      .map((progress) => {
                        const pct = Math.min(progress.percentage, 100)
                        const isSlower = progress.percentage < avgPercentage - 5

                        return (
                          <div
                            key={progress.node}
                            className={`flex items-center gap-3 p-2 rounded-lg text-sm ${isSlower ? 'bg-amber-50' : 'hover:bg-gray-50'}`}
                          >
                            <span
                              className={`w-28 truncate font-mono text-xs ${isSlower ? 'text-amber-700' : 'text-gray-600'}`}
                            >
                              {progress.node}
                            </span>
                            <div className="flex-1 relative">
                              <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                                <div
                                  className={`h-full rounded-full transition-all duration-300 ${
                                    isSlower ? 'bg-amber-500' : 'bg-blue-500'
                                  }`}
                                  style={{ width: `${pct}%` }}
                                />
                              </div>
                            </div>
                            <span
                              className={`w-14 text-right font-medium ${isSlower ? 'text-amber-700' : 'text-gray-700'}`}
                            >
                              {progress.percentage.toFixed(1)}%
                            </span>
                            <span className="text-gray-500 w-24 text-right text-xs">
                              {formatBytes(progress.bytesPerSecond)}/s
                            </span>
                            <span className="text-gray-400 w-16 text-right text-xs">
                              {progress.remainingTime > 0
                                ? formatDuration(progress.remainingTime)
                                : '-'}
                            </span>
                          </div>
                        )
                      })}
                  </div>
                </div>
              )
            })()}

          {/* Nodes Ready/Failed */}
          {model.status?.nodesReady && (
            <NodeList title="Ready on Nodes" nodes={model.status.nodesReady} variant="success" />
          )}
          {model.status?.nodesFailed && (
            <NodeList title="Failed on Nodes" nodes={model.status.nodesFailed} variant="error" />
          )}

          {/* Labels & Annotations */}
          {((model.metadata.labels && Object.keys(model.metadata.labels).length > 0) ||
            (model.metadata.annotations && Object.keys(model.metadata.annotations).length > 0)) && (
            <div className="mt-6 pt-6 border-t border-gray-200">
              <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
                {model.metadata.labels && (
                  <KeyValueList title="Labels" items={model.metadata.labels} />
                )}
                {model.metadata.annotations && (
                  <KeyValueList title="Annotations" items={model.metadata.annotations} truncate />
                )}
              </div>
            </div>
          )}
        </div>

        {/* Model Specification */}
        <div className="mb-6 rounded-lg bg-white p-6 shadow">
          <h2 className="mb-4 text-lg font-medium text-gray-900">Model Specification</h2>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
            <SpecCard label="Vendor">{model.spec.vendor || '-'}</SpecCard>
            <SpecCard label="Parameter Size">{model.spec.modelParameterSize || '-'}</SpecCard>
            <SpecCard label="Framework">
              {model.spec.modelFramework?.name || '-'}
              {model.spec.modelFramework?.version && (
                <span className="text-gray-500 font-normal">
                  {' '}
                  ({model.spec.modelFramework.version})
                </span>
              )}
            </SpecCard>
            <SpecCard label="Format">
              {model.spec.modelFormat?.name || '-'}
              {model.spec.modelFormat?.version && (
                <span className="text-gray-500 font-normal">
                  {' '}
                  v{model.spec.modelFormat.version}
                </span>
              )}
            </SpecCard>

            {/* Model Configuration fields */}
            {model.spec.modelConfiguration?.architecture && (
              <SpecCard label="Architecture" title={model.spec.modelConfiguration.architecture}>
                {model.spec.modelConfiguration.architecture}
              </SpecCard>
            )}
            {model.spec.modelConfiguration?.model_type && (
              <SpecCard label="Model Type">{model.spec.modelConfiguration.model_type}</SpecCard>
            )}
            {model.spec.modelConfiguration?.context_length && (
              <SpecCard label="Context Length">
                {model.spec.modelConfiguration.context_length.toLocaleString()}
              </SpecCard>
            )}
            {model.spec.modelConfiguration?.torch_dtype && (
              <SpecCard label="Data Type">{model.spec.modelConfiguration.torch_dtype}</SpecCard>
            )}
            {model.spec.modelConfiguration?.transformers_version && (
              <SpecCard label="Transformers">
                v{model.spec.modelConfiguration.transformers_version}
              </SpecCard>
            )}
            {model.spec.modelConfiguration?.has_vision !== undefined && (
              <SpecCard label="Vision Support">
                {model.spec.modelConfiguration.has_vision ? (
                  <span className="text-green-600">Yes</span>
                ) : (
                  <span>No</span>
                )}
              </SpecCard>
            )}
          </div>
        </div>

        {/* Storage Configuration */}
        {model.spec.storage && (
          <div className="mb-6 rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Storage Configuration</h2>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <SpecCard label="Storage URI" copyValue={model.spec.storage.storageUri}>
                <span className="font-mono">{model.spec.storage.storageUri || '-'}</span>
              </SpecCard>
              {model.spec.storage.path && (
                <SpecCard label="Local Path" copyValue={model.spec.storage.path}>
                  <span className="font-mono">{model.spec.storage.path}</span>
                </SpecCard>
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
            <Icons.chevronDown
              size="md"
              className={`text-gray-500 transition-transform ${showRawSpec ? 'rotate-180' : ''}`}
            />
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
