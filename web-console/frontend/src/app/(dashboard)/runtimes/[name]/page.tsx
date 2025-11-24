'use client'

import { useRuntime, useDeleteRuntime } from '@/lib/hooks/useRuntimes'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { useState } from 'react'
import { ConfirmDeleteModal } from '@/components/ui/Modal'
import { ModelFormatsDisplay } from '@/components/ui/ModelFormatsDisplay'
import { MetadataCollapsible } from '@/components/ui/MetadataCollapsible'

// Reusable component for displaying K8s resources
function ResourceDisplay({ resources }: { resources: any }) {
  if (!resources) return null

  return (
    <div className="mt-2 bg-gray-50 border border-gray-200 rounded p-2">
      <p className="text-xs font-semibold text-gray-700 mb-1.5">Resources</p>
      <div className="space-y-1.5 text-xs">
        {resources.requests && (
          <div>
            <span className="font-medium text-gray-600">Requests:</span>
            <div className="ml-3 space-y-0.5">
              {Object.entries(resources.requests).map(([key, value]: [string, any]) => (
                <div key={key} className="flex">
                  <span className="text-gray-500 w-32">{key}:</span>
                  <span className="font-mono text-gray-900">{value}</span>
                </div>
              ))}
            </div>
          </div>
        )}
        {resources.limits && (
          <div>
            <span className="font-medium text-gray-600">Limits:</span>
            <div className="ml-3 space-y-0.5">
              {Object.entries(resources.limits).map(([key, value]: [string, any]) => (
                <div key={key} className="flex">
                  <span className="text-gray-500 w-32">{key}:</span>
                  <span className="font-mono text-gray-900">{value}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

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
    <div className="min-h-screen pb-12">
      {/* Header */}
      <header className="relative border-b border-border/50 bg-card/50 backdrop-blur-sm animate-in">
        <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          <Link
            href="/runtimes"
            className="group inline-flex items-center gap-2 text-sm font-medium text-primary hover:text-primary/80 transition-colors mb-4"
          >
            <svg className="w-4 h-4 transition-transform group-hover:-translate-x-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            Back to Runtimes
          </Link>
          <div className="flex items-start justify-between">
            <div>
              <h1 className="text-4xl font-bold tracking-tight">{runtime.metadata.name}</h1>
              <p className="mt-2 text-muted-foreground max-w-2xl">
                Runtime configuration details
              </p>
            </div>
            <div className="flex gap-3">
              <button
                onClick={() => router.push(`/runtimes/${name}/edit`)}
                className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white hover:bg-primary/90 transition-colors"
              >
                Edit Runtime
              </button>
              <button
                onClick={() => setShowDeleteModal(true)}
                className="rounded-lg border border-destructive px-4 py-2 text-sm font-medium text-destructive hover:bg-destructive/10 transition-colors"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
        {/* Runtime Type Badge */}
        <div className="mb-6">
          {runtime.spec?.engineConfig && runtime.spec?.decoderConfig && (
            <span className="inline-flex items-center rounded-md bg-purple-100 px-3 py-1.5 text-xs font-bold text-purple-900 ring-1 ring-inset ring-purple-600/20">
              🚀 PREFILL-DECODE DISAGGREGATED (PDD)
            </span>
          )}
          {runtime.spec?.engineConfig && !runtime.spec?.decoderConfig && (
            <span className="inline-flex items-center rounded-md bg-accent/20 px-3 py-1.5 text-xs font-bold text-blue-900 ring-1 ring-inset ring-blue-600/20">
              ENGINE ONLY
            </span>
          )}
          {!runtime.spec?.engineConfig && runtime.spec?.decoderConfig && (
            <span className="inline-flex items-center rounded-md bg-accent/20 px-3 py-1.5 text-xs font-bold text-orange-900 ring-1 ring-inset ring-orange-600/20">
              DECODER ONLY
            </span>
          )}
          {runtime.spec?.containers && !runtime.spec?.engineConfig && !runtime.spec?.decoderConfig && (
            <span className="inline-flex items-center rounded-md bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-900 ring-1 ring-inset ring-gray-600/20">
              STANDARD RUNTIME
            </span>
          )}
        </div>

        {/* Runtime Details */}
        <div className="rounded-xl border border-border bg-card shadow-lg animate-in-delay-1">
          <div className="px-4 py-3 bg-gray-50 border-b border-gray-200">
            <h3 className="text-base font-medium text-gray-900">Runtime Configuration</h3>
          </div>

          <div className="p-6 space-y-6">
            {/* Basic Info */}
            <div>
              <h4 className="text-sm font-medium text-gray-900 mb-3">Basic Information</h4>
              <dl className="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2">
                <div>
                  <dt className="text-xs font-medium text-gray-500">Name</dt>
                  <dd className="mt-1 text-sm text-gray-900">{runtime.metadata?.name}</dd>
                </div>
                <div>
                  <dt className="text-xs font-medium text-gray-500">Kind</dt>
                  <dd className="mt-1 text-sm text-gray-900">{runtime.kind}</dd>
                </div>
                <div>
                  <dt className="text-xs font-medium text-gray-500">API Version</dt>
                  <dd className="mt-1 text-sm text-gray-900">{runtime.apiVersion}</dd>
                </div>
                {runtime.spec?.disabled !== undefined && (
                  <div>
                    <dt className="text-xs font-medium text-gray-500">Status</dt>
                    <dd className="mt-1">
                      <span className={`inline-flex rounded-full px-2 text-xs font-semibold leading-5 ${
                        runtime.spec.disabled ? 'bg-gray-100 text-gray-800' : 'bg-green-100 text-green-800'
                      }`}>
                        {runtime.spec.disabled ? 'Disabled' : 'Active'}
                      </span>
                    </dd>
                  </div>
                )}
                {runtime.spec?.multiModel !== undefined && (
                  <div>
                    <dt className="text-xs font-medium text-gray-500">Multi-Model</dt>
                    <dd className="mt-1">
                      <span className={`inline-flex rounded-full px-2 text-xs font-semibold leading-5 ${
                        runtime.spec.multiModel ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                      }`}>
                        {runtime.spec.multiModel ? 'Enabled' : 'Disabled'}
                      </span>
                    </dd>
                  </div>
                )}
                {runtime.metadata?.creationTimestamp && (
                  <div>
                    <dt className="text-xs font-medium text-gray-500">Created</dt>
                    <dd className="mt-1 text-sm text-gray-900">
                      {new Date(runtime.metadata.creationTimestamp).toLocaleString()}
                    </dd>
                  </div>
                )}
              </dl>
            </div>

            {/* Metadata - Labels and Annotations */}
            <MetadataCollapsible
              labels={runtime.metadata?.labels}
              annotations={runtime.metadata?.annotations}
            />

            {/* Supported Model Formats */}
            <ModelFormatsDisplay formats={runtime.spec?.supportedModelFormats || []} />

            {/* Protocol Versions */}
            {runtime.spec?.protocolVersions && runtime.spec.protocolVersions.length > 0 && (
              <div>
                <h4 className="text-sm font-medium text-gray-900 mb-3">Protocol Versions</h4>
                <div className="flex flex-wrap gap-2">
                  {runtime.spec.protocolVersions.map((version: string, idx: number) => (
                    <span
                      key={idx}
                      className="inline-flex items-center rounded-md bg-blue-50 px-3 py-1 text-sm font-medium text-accent ring-1 ring-inset ring-blue-700/10"
                    >
                      {version}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {/* Router Config */}
            {runtime.spec?.routerConfig && (
              <div className="border-l-4 border-accent bg-accent/10 p-4">
                <h4 className="text-sm font-medium text-accent mb-3">🔀 Router Configuration</h4>
                <div className="space-y-3">
                  {runtime.spec.routerConfig.runner && (
                    <div className="bg-white rounded-lg p-3 border border-accent/20">
                      <p className="text-xs font-medium text-gray-700 mb-2">Container: {runtime.spec.routerConfig.runner.name || 'router-container'}</p>
                      <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {runtime.spec.routerConfig.runner.image}</p>
                      <ResourceDisplay resources={runtime.spec.routerConfig.runner.resources} />
                      {runtime.spec.routerConfig.runner.env && runtime.spec.routerConfig.runner.env.length > 0 && (
                        <details className="mt-2">
                          <summary className="text-xs font-medium text-accent cursor-pointer">Environment Variables ({runtime.spec.routerConfig.runner.env.length})</summary>
                          <div className="mt-1 space-y-1 pl-2">
                            {runtime.spec.routerConfig.runner.env.map((env: any, i: number) => (
                              <p key={i} className="text-xs text-gray-600 font-mono">{env.name}={env.value}</p>
                            ))}
                          </div>
                        </details>
                      )}
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Engine Config */}
            {runtime.spec?.engineConfig && (
              <div className="border-l-4 border-accent bg-accent/10 p-4">
                <h4 className="text-sm font-medium text-accent mb-3">⚡ Engine Configuration (Prefill)</h4>
                <div className="space-y-3">
                  {/* Single-node: Direct runner */}
                  {runtime.spec.engineConfig.runner && !runtime.spec.engineConfig.leader && (
                    <div className="bg-white rounded-lg p-3 border border-accent/20">
                      <p className="text-xs font-bold text-accent/90 mb-2">🖥️ Single Node</p>
                      <p className="text-xs font-medium text-gray-700 mb-1">Container: {runtime.spec.engineConfig.runner.name || 'N/A'}</p>
                      <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {runtime.spec.engineConfig.runner.image || 'N/A'}</p>
                      <ResourceDisplay resources={runtime.spec.engineConfig.runner.resources} />
                      {runtime.spec.engineConfig.runner.env && Array.isArray(runtime.spec.engineConfig.runner.env) && runtime.spec.engineConfig.runner.env.length > 0 && (
                        <details className="mt-2">
                          <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({runtime.spec.engineConfig.runner.env.length})</summary>
                          <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                            {runtime.spec.engineConfig.runner.env.map((env: any, i: number) => (
                              <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || (env.valueFrom ? '[from field]' : '')}</p>
                            ))}
                          </div>
                        </details>
                      )}
                      {runtime.spec.engineConfig.runner.command && Array.isArray(runtime.spec.engineConfig.runner.command) && (
                        <details className="mt-2">
                          <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({runtime.spec.engineConfig.runner.command.length} args)</summary>
                          <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{runtime.spec.engineConfig.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                        </details>
                      )}
                      {runtime.spec.engineConfig.runner.args && Array.isArray(runtime.spec.engineConfig.runner.args) && runtime.spec.engineConfig.runner.args.length > 0 && (
                        <details className="mt-2">
                          <summary className="text-xs font-medium text-accent cursor-pointer">📝 Args ({runtime.spec.engineConfig.runner.args.length})</summary>
                          <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{runtime.spec.engineConfig.runner.args.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                        </details>
                      )}
                    </div>
                  )}

                  {/* Multi-node: Leader */}
                  {runtime.spec.engineConfig.leader && (
                    <div className="bg-white rounded-lg p-3 border border-accent/20">
                      <p className="text-xs font-bold text-accent/90 mb-2">👑 Leader Node</p>
                      {runtime.spec.engineConfig.leader.runner ? (
                        <>
                          <p className="text-xs font-medium text-gray-700 mb-1">Container: {runtime.spec.engineConfig.leader.runner.name || 'N/A'}</p>
                          <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {runtime.spec.engineConfig.leader.runner.image || 'N/A'}</p>
                          <ResourceDisplay resources={runtime.spec.engineConfig.leader.runner.resources} />
                          {runtime.spec.engineConfig.leader.runner.env && Array.isArray(runtime.spec.engineConfig.leader.runner.env) && runtime.spec.engineConfig.leader.runner.env.length > 0 && (
                            <details className="mt-2">
                              <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({runtime.spec.engineConfig.leader.runner.env.length})</summary>
                              <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                                {runtime.spec.engineConfig.leader.runner.env.map((env: any, i: number) => (
                                  <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || env.valueFrom ? '[from field]' : ''}</p>
                                ))}
                              </div>
                            </details>
                          )}
                          {runtime.spec.engineConfig.leader.runner.command && Array.isArray(runtime.spec.engineConfig.leader.runner.command) && (
                            <details className="mt-2">
                              <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({runtime.spec.engineConfig.leader.runner.command.length} args)</summary>
                              <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{runtime.spec.engineConfig.leader.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                            </details>
                          )}
                        </>
                      ) : (
                        <p className="text-xs text-gray-500 italic">No runner configuration</p>
                      )}
                      {runtime.spec.engineConfig.leader.nodeSelector && Object.keys(runtime.spec.engineConfig.leader.nodeSelector).length > 0 && (
                        <details className="mt-2">
                          <summary className="text-xs font-medium text-accent cursor-pointer">🎯 Node Selector ({Object.keys(runtime.spec.engineConfig.leader.nodeSelector).length} rules)</summary>
                          <div className="mt-1 space-y-1 pl-2">
                            {Object.entries(runtime.spec.engineConfig.leader.nodeSelector).map(([key, value]: [string, any], i: number) => (
                              <p key={i} className="text-xs text-gray-600 font-mono break-all">{key}: {String(value)}</p>
                            ))}
                          </div>
                        </details>
                      )}
                    </div>
                  )}

                  {/* Worker */}
                  {runtime.spec.engineConfig.worker && (
                    <div className="bg-white rounded-lg p-3 border border-accent/20">
                      <div className="flex items-center justify-between mb-2">
                        <p className="text-xs font-bold text-accent/90">👥 Worker Nodes</p>
                        <span className="inline-flex items-center rounded-md bg-accent/20 px-2 py-1 text-xs font-medium text-accent">
                          {runtime.spec.engineConfig.worker.size || 1} node{(runtime.spec.engineConfig.worker.size || 1) > 1 ? 's' : ''}
                        </span>
                      </div>
                      {runtime.spec.engineConfig.worker.runner ? (
                        <>
                          <p className="text-xs font-medium text-gray-700 mb-1">Container: {runtime.spec.engineConfig.worker.runner.name || 'N/A'}</p>
                          <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {runtime.spec.engineConfig.worker.runner.image || 'N/A'}</p>
                          <ResourceDisplay resources={runtime.spec.engineConfig.worker.runner.resources} />
                          {runtime.spec.engineConfig.worker.runner.env && Array.isArray(runtime.spec.engineConfig.worker.runner.env) && runtime.spec.engineConfig.worker.runner.env.length > 0 && (
                            <details className="mt-2">
                              <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({runtime.spec.engineConfig.worker.runner.env.length})</summary>
                              <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                                {runtime.spec.engineConfig.worker.runner.env.map((env: any, i: number) => (
                                  <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || env.valueFrom ? '[from field]' : ''}</p>
                                ))}
                              </div>
                            </details>
                          )}
                          {runtime.spec.engineConfig.worker.runner.command && Array.isArray(runtime.spec.engineConfig.worker.runner.command) && (
                            <details className="mt-2">
                              <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({runtime.spec.engineConfig.worker.runner.command.length} args)</summary>
                              <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{runtime.spec.engineConfig.worker.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                            </details>
                          )}
                        </>
                      ) : (
                        <p className="text-xs text-gray-500 italic">No runner configuration</p>
                      )}
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Decoder Config */}
            {runtime.spec?.decoderConfig && (
              <div className="border-l-4 border-accent bg-accent/10 p-4">
                <h4 className="text-sm font-medium text-accent mb-3">🔄 Decoder Configuration (Decode)</h4>
                <div className="space-y-3">
                  {/* Single-node: Direct runner */}
                  {runtime.spec.decoderConfig.runner && !runtime.spec.decoderConfig.leader && (
                    <div className="bg-white rounded-lg p-3 border border-accent/20">
                      <p className="text-xs font-bold text-accent/90 mb-2">🖥️ Single Node</p>
                      <p className="text-xs font-medium text-gray-700 mb-1">Container: {runtime.spec.decoderConfig.runner.name || 'N/A'}</p>
                      <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {runtime.spec.decoderConfig.runner.image || 'N/A'}</p>
                      <ResourceDisplay resources={runtime.spec.decoderConfig.runner.resources} />
                      {runtime.spec.decoderConfig.runner.env && Array.isArray(runtime.spec.decoderConfig.runner.env) && runtime.spec.decoderConfig.runner.env.length > 0 && (
                        <details className="mt-2">
                          <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({runtime.spec.decoderConfig.runner.env.length})</summary>
                          <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                            {runtime.spec.decoderConfig.runner.env.map((env: any, i: number) => (
                              <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || (env.valueFrom ? '[from field]' : '')}</p>
                            ))}
                          </div>
                        </details>
                      )}
                      {runtime.spec.decoderConfig.runner.command && Array.isArray(runtime.spec.decoderConfig.runner.command) && (
                        <details className="mt-2">
                          <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({runtime.spec.decoderConfig.runner.command.length} args)</summary>
                          <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{runtime.spec.decoderConfig.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                        </details>
                      )}
                      {runtime.spec.decoderConfig.runner.args && Array.isArray(runtime.spec.decoderConfig.runner.args) && runtime.spec.decoderConfig.runner.args.length > 0 && (
                        <details className="mt-2">
                          <summary className="text-xs font-medium text-accent cursor-pointer">📝 Args ({runtime.spec.decoderConfig.runner.args.length})</summary>
                          <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-2 rounded overflow-x-auto max-h-60">{runtime.spec.decoderConfig.runner.args.join(' ')}</pre>
                        </details>
                      )}
                    </div>
                  )}

                  {/* Multi-node: Leader */}
                  {runtime.spec.decoderConfig.leader && (
                    <div className="bg-white rounded-lg p-3 border border-accent/20">
                      <p className="text-xs font-bold text-accent/90 mb-2">👑 Leader Node</p>
                      {runtime.spec.decoderConfig.leader.runner ? (
                        <>
                          <p className="text-xs font-medium text-gray-700 mb-1">Container: {runtime.spec.decoderConfig.leader.runner.name || 'N/A'}</p>
                          <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {runtime.spec.decoderConfig.leader.runner.image || 'N/A'}</p>
                          <ResourceDisplay resources={runtime.spec.decoderConfig.leader.runner.resources} />
                          {runtime.spec.decoderConfig.leader.runner.env && Array.isArray(runtime.spec.decoderConfig.leader.runner.env) && runtime.spec.decoderConfig.leader.runner.env.length > 0 && (
                            <details className="mt-2">
                              <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({runtime.spec.decoderConfig.leader.runner.env.length})</summary>
                              <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                                {runtime.spec.decoderConfig.leader.runner.env.map((env: any, i: number) => (
                                  <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || env.valueFrom ? '[from field]' : ''}</p>
                                ))}
                              </div>
                            </details>
                          )}
                          {runtime.spec.decoderConfig.leader.runner.command && Array.isArray(runtime.spec.decoderConfig.leader.runner.command) && (
                            <details className="mt-2">
                              <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({runtime.spec.decoderConfig.leader.runner.command.length} args)</summary>
                              <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{runtime.spec.decoderConfig.leader.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                            </details>
                          )}
                        </>
                      ) : (
                        <p className="text-xs text-gray-500 italic">No runner configuration</p>
                      )}
                      {runtime.spec.decoderConfig.leader.nodeSelector && Object.keys(runtime.spec.decoderConfig.leader.nodeSelector).length > 0 && (
                        <details className="mt-2">
                          <summary className="text-xs font-medium text-accent cursor-pointer">🎯 Node Selector ({Object.keys(runtime.spec.decoderConfig.leader.nodeSelector).length} rules)</summary>
                          <div className="mt-1 space-y-1 pl-2">
                            {Object.entries(runtime.spec.decoderConfig.leader.nodeSelector).map(([key, value]: [string, any], i: number) => (
                              <p key={i} className="text-xs text-gray-600 font-mono break-all">{key}: {String(value)}</p>
                            ))}
                          </div>
                        </details>
                      )}
                    </div>
                  )}

                  {/* Worker */}
                  {runtime.spec.decoderConfig.worker && (
                    <div className="bg-white rounded-lg p-3 border border-accent/20">
                      <div className="flex items-center justify-between mb-2">
                        <p className="text-xs font-bold text-accent/90">👥 Worker Nodes</p>
                        <span className="inline-flex items-center rounded-md bg-accent/20 px-2 py-1 text-xs font-medium text-accent">
                          {runtime.spec.decoderConfig.worker.size || 1} node{(runtime.spec.decoderConfig.worker.size || 1) > 1 ? 's' : ''}
                        </span>
                      </div>
                      {runtime.spec.decoderConfig.worker.runner ? (
                        <>
                          <p className="text-xs font-medium text-gray-700 mb-1">Container: {runtime.spec.decoderConfig.worker.runner.name || 'N/A'}</p>
                          <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {runtime.spec.decoderConfig.worker.runner.image || 'N/A'}</p>
                          <ResourceDisplay resources={runtime.spec.decoderConfig.worker.runner.resources} />
                          {runtime.spec.decoderConfig.worker.runner.env && Array.isArray(runtime.spec.decoderConfig.worker.runner.env) && runtime.spec.decoderConfig.worker.runner.env.length > 0 && (
                            <details className="mt-2">
                              <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({runtime.spec.decoderConfig.worker.runner.env.length})</summary>
                              <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                                {runtime.spec.decoderConfig.worker.runner.env.map((env: any, i: number) => (
                                  <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || env.valueFrom ? '[from field]' : ''}</p>
                                ))}
                              </div>
                            </details>
                          )}
                          {runtime.spec.decoderConfig.worker.runner.command && Array.isArray(runtime.spec.decoderConfig.worker.runner.command) && (
                            <details className="mt-2">
                              <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({runtime.spec.decoderConfig.worker.runner.command.length} args)</summary>
                              <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{runtime.spec.decoderConfig.worker.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                            </details>
                          )}
                        </>
                      ) : (
                        <p className="text-xs text-gray-500 italic">No runner configuration</p>
                      )}
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Standard Containers (if no engine/decoder config) */}
            {runtime.spec?.containers && !runtime.spec?.engineConfig && !runtime.spec?.decoderConfig && (
              <div className="border-l-4 border-gray-500 bg-gray-50 p-4">
                <h4 className="text-sm font-medium text-gray-900 mb-3">📦 Containers ({runtime.spec.containers.length})</h4>
                <div className="space-y-3">
                  {runtime.spec.containers.map((container: any, idx: number) => (
                    <div key={idx} className="bg-white rounded-lg p-3 border border-gray-200">
                      <p className="text-xs font-medium text-gray-700 mb-1">{container.name}</p>
                      <p className="text-xs text-gray-600 font-mono break-all mb-2">{container.image}</p>
                      <ResourceDisplay resources={container.resources} />
                      {container.env && container.env.length > 0 && (
                        <details className="mt-2">
                          <summary className="text-xs font-medium text-gray-700 cursor-pointer">🔧 Environment Variables ({container.env.length})</summary>
                          <div className="mt-1 space-y-1 pl-2">
                            {container.env.map((env: any, i: number) => (
                              <p key={i} className="text-xs text-gray-600 font-mono">{env.name}={env.value}</p>
                            ))}
                          </div>
                        </details>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
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
