'use client'

import { useState, useMemo } from 'react'
import { useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useModels } from '@/lib/hooks/useModels'
import { useNamespaces } from '@/lib/hooks/useNamespaces'
import { useRuntimes, useCompatibleRuntimes, useRuntimeRecommendation } from '@/lib/hooks/useRuntimes'
import { useCreateService } from '@/lib/hooks/useServices'
import { Button, ButtonIcons } from '@/components/ui/Button'
import { LoadingState } from '@/components/ui/LoadingState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import Link from 'next/link'
import type { ClusterBaseModel } from '@/lib/types/model'
import type { RuntimeMatch } from '@/lib/types/runtime'

const deploySchema = z.object({
  name: z.string().min(1, 'Name is required').regex(/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/, 'Must be a valid Kubernetes name'),
  namespace: z.string().min(1, 'Namespace is required'),
  model: z.string().min(1, 'Model is required'),
  runtime: z.string().min(1, 'Runtime is required'),
  replicas: z.number().min(1).max(100),
  minReplicas: z.number().min(0).max(100).optional(),
  maxReplicas: z.number().min(1).max(100).optional(),
})

type DeployFormData = {
  name: string
  namespace: string
  model: string
  runtime: string
  replicas: number
  minReplicas?: number
  maxReplicas?: number
}

// Score badge component
function ScoreBadge({ score }: { score: number }) {
  const getScoreColor = (s: number) => {
    if (s >= 80) return 'bg-success/10 text-success border-success/20'
    if (s >= 60) return 'bg-warning/10 text-warning border-warning/20'
    return 'bg-muted text-muted-foreground border-border'
  }

  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium border ${getScoreColor(score)}`}>
      <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
        <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
      </svg>
      {score}%
    </span>
  )
}

// Runtime card component
function RuntimeCard({
  match,
  isSelected,
  isRecommended,
  onSelect,
}: {
  match: RuntimeMatch
  isSelected: boolean
  isRecommended: boolean
  onSelect: () => void
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={`w-full text-left p-4 rounded-xl border-2 transition-all duration-200 ${
        isSelected
          ? 'border-primary bg-primary/5 shadow-md'
          : 'border-border hover:border-primary/30 hover:bg-muted/30'
      }`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <span className="font-medium text-foreground truncate">
              {match.runtime.metadata.name}
            </span>
            {isRecommended && (
              <span className="inline-flex items-center gap-1 rounded-full bg-accent/10 text-accent px-2 py-0.5 text-xs font-medium">
                <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                Recommended
              </span>
            )}
          </div>
          <p className="text-sm text-muted-foreground line-clamp-2">
            {match.recommendation}
          </p>
        </div>
        <ScoreBadge score={match.score} />
      </div>

      {/* Reasons */}
      {match.reasons.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1">
          {match.reasons.slice(0, 3).map((reason, idx) => (
            <span
              key={idx}
              className="inline-flex items-center rounded-md bg-success/10 text-success px-2 py-0.5 text-xs"
            >
              <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
              </svg>
              {reason}
            </span>
          ))}
        </div>
      )}

      {/* Warnings */}
      {match.warnings && match.warnings.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          {match.warnings.map((warning, idx) => (
            <span
              key={idx}
              className="inline-flex items-center rounded-md bg-warning/10 text-warning px-2 py-0.5 text-xs"
            >
              <svg className="w-3 h-3 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              {warning}
            </span>
          ))}
        </div>
      )}

      {/* Selection indicator */}
      <div className={`mt-3 flex items-center justify-end ${isSelected ? 'text-primary' : 'text-muted-foreground'}`}>
        {isSelected ? (
          <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
            <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
          </svg>
        ) : (
          <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <circle cx="12" cy="12" r="10" />
          </svg>
        )}
      </div>
    </button>
  )
}

export default function DeployServicePage() {
  const router = useRouter()
  const [selectedModel, setSelectedModel] = useState<ClusterBaseModel | null>(null)

  const { data: modelsData, isLoading: modelsLoading } = useModels()
  const { data: namespacesData } = useNamespaces()
  const { data: runtimesData } = useRuntimes()
  const createService = useCreateService()

  // Extract model format and framework for smart selection
  const modelFormat = selectedModel?.spec.modelFormat?.name
  const modelFramework = selectedModel?.spec.modelFramework?.name

  // Smart selection queries - only enabled when a model is selected
  const { data: compatibleData, isLoading: compatibleLoading } = useCompatibleRuntimes(modelFormat, modelFramework)
  const { data: recommendedRuntime } = useRuntimeRecommendation(modelFormat, modelFramework)

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<DeployFormData>({
    resolver: zodResolver(deploySchema),
    defaultValues: {
      replicas: 1,
      namespace: 'default',
    },
  })

  const watchedRuntime = watch('runtime')
  const watchedModel = watch('model')

  // Handle model selection change
  const handleModelChange = (modelName: string) => {
    const model = modelsData?.items.find((m) => m.metadata.name === modelName) || null
    setSelectedModel(model)
    setValue('model', modelName)
    // Reset runtime selection when model changes
    setValue('runtime', '')
  }

  // Get all runtimes (compatible ones first, then others)
  const sortedRuntimes = useMemo(() => {
    if (!runtimesData?.items) return []
    if (!compatibleData?.matches) return runtimesData.items.map((r) => ({
      runtime: r,
      score: 0,
      compatibleWith: [],
      reasons: [],
      warnings: [],
      recommendation: 'Select a model to see compatibility',
    }))

    const compatibleNames = new Set(compatibleData.matches.map((m) => m.runtime.metadata.name))
    const incompatible = runtimesData.items
      .filter((r) => !compatibleNames.has(r.metadata.name))
      .map((r) => ({
        runtime: r,
        score: 0,
        compatibleWith: [],
        reasons: [],
        warnings: ['Not compatible with selected model'],
        recommendation: 'This runtime does not support the selected model format',
      }))

    return [...compatibleData.matches, ...incompatible]
  }, [runtimesData, compatibleData])

  const onSubmit = async (data: DeployFormData) => {
    try {
      await createService.mutateAsync({
        apiVersion: 'serving.ome.io/v1beta1',
        kind: 'InferenceService',
        metadata: {
          name: data.name,
          namespace: data.namespace,
        },
        spec: {
          predictor: {
            model: data.model,
            runtime: data.runtime,
            replicas: data.replicas,
            minReplicas: data.minReplicas,
            maxReplicas: data.maxReplicas,
          },
        },
      })
      router.push('/services')
    } catch (error) {
      console.error('Failed to deploy service:', error)
    }
  }

  if (modelsLoading) {
    return <LoadingState message="Loading models..." />
  }

  return (
    <div className="min-h-screen pb-12">
      {/* Header */}
      <header className="border-b border-border bg-gradient-to-r from-primary/5 via-transparent to-accent/5">
        <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
          <div className="flex items-center gap-4 mb-4">
            <Link
              href="/services"
              className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
              </svg>
              Back to Services
            </Link>
          </div>
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-primary to-accent shadow-lg shadow-primary/25">
              <svg className="w-6 h-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M5.636 5.636a9 9 0 1012.728 0M12 3v9" />
              </svg>
            </div>
            <div>
              <h1 className="text-2xl font-semibold tracking-tight text-foreground">
                Deploy Inference Service
              </h1>
              <p className="text-sm text-muted-foreground">
                Create a new inference service with smart runtime selection
              </p>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-8">
          {/* Basic Information */}
          <section className="rounded-xl border border-border bg-card shadow-sm overflow-hidden">
            <div className="border-b border-border bg-muted/30 px-6 py-4">
              <h2 className="text-lg font-semibold">Basic Information</h2>
            </div>
            <div className="p-6 space-y-6">
              <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                <div>
                  <label htmlFor="name" className="block text-sm font-medium mb-2">
                    Service Name <span className="text-destructive">*</span>
                  </label>
                  <input
                    id="name"
                    type="text"
                    {...register('name')}
                    placeholder="my-inference-service"
                    className="w-full rounded-lg border border-border bg-background px-4 py-2.5 text-sm shadow-sm transition-colors focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20"
                  />
                  {errors.name && (
                    <p className="mt-1 text-sm text-destructive">{errors.name.message}</p>
                  )}
                </div>

                <div>
                  <label htmlFor="namespace" className="block text-sm font-medium mb-2">
                    Namespace <span className="text-destructive">*</span>
                  </label>
                  <select
                    id="namespace"
                    {...register('namespace')}
                    className="w-full rounded-lg border border-border bg-background px-4 py-2.5 text-sm shadow-sm transition-colors focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20"
                  >
                    <option value="default">default</option>
                    {namespacesData?.items.map((ns) => (
                      <option key={ns} value={ns}>
                        {ns}
                      </option>
                    ))}
                  </select>
                  {errors.namespace && (
                    <p className="mt-1 text-sm text-destructive">{errors.namespace.message}</p>
                  )}
                </div>
              </div>
            </div>
          </section>

          {/* Model Selection */}
          <section className="rounded-xl border border-border bg-card shadow-sm overflow-hidden">
            <div className="border-b border-border bg-muted/30 px-6 py-4">
              <h2 className="text-lg font-semibold">Model Selection</h2>
              <p className="text-sm text-muted-foreground mt-1">
                Select a model to see compatible runtimes
              </p>
            </div>
            <div className="p-6">
              <div>
                <label htmlFor="model" className="block text-sm font-medium mb-2">
                  Model <span className="text-destructive">*</span>
                </label>
                <select
                  id="model"
                  value={watchedModel || ''}
                  onChange={(e) => handleModelChange(e.target.value)}
                  className="w-full rounded-lg border border-border bg-background px-4 py-2.5 text-sm shadow-sm transition-colors focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20"
                >
                  <option value="">Select a model...</option>
                  {modelsData?.items.map((model) => (
                    <option key={model.metadata.name} value={model.metadata.name}>
                      {model.metadata.name} ({model.spec.modelFormat?.name || 'Unknown format'})
                    </option>
                  ))}
                </select>
                {errors.model && (
                  <p className="mt-1 text-sm text-destructive">{errors.model.message}</p>
                )}
              </div>

              {/* Selected Model Info */}
              {selectedModel && (
                <div className="mt-4 p-4 rounded-lg bg-muted/30 border border-border">
                  <div className="flex items-start gap-4">
                    <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
                      <svg className="w-5 h-5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M21 7.5l-9-5.25L3 7.5m18 0l-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
                      </svg>
                    </div>
                    <div className="flex-1">
                      <h4 className="font-medium">{selectedModel.metadata.name}</h4>
                      <div className="mt-2 flex flex-wrap gap-2">
                        {selectedModel.spec.modelFormat?.name && (
                          <span className="inline-flex items-center rounded-md bg-primary/10 text-primary px-2 py-0.5 text-xs font-medium">
                            Format: {selectedModel.spec.modelFormat.name}
                          </span>
                        )}
                        {selectedModel.spec.modelFramework?.name && (
                          <span className="inline-flex items-center rounded-md bg-accent/10 text-accent px-2 py-0.5 text-xs font-medium">
                            Framework: {selectedModel.spec.modelFramework.name}
                          </span>
                        )}
                        {selectedModel.spec.modelParameterSize && (
                          <span className="inline-flex items-center rounded-md bg-muted text-muted-foreground px-2 py-0.5 text-xs font-medium">
                            Size: {selectedModel.spec.modelParameterSize}
                          </span>
                        )}
                        {selectedModel.status?.state && (
                          <StatusBadge state={selectedModel.status.state} size="sm" />
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </section>

          {/* Smart Runtime Selection */}
          <section className="rounded-xl border border-border bg-card shadow-sm overflow-hidden">
            <div className="border-b border-border bg-muted/30 px-6 py-4">
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-lg font-semibold flex items-center gap-2">
                    Runtime Selection
                    {selectedModel && (
                      <span className="inline-flex items-center gap-1 rounded-full bg-accent/10 text-accent px-2 py-0.5 text-xs font-medium">
                        <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
                        </svg>
                        Smart Selection Active
                      </span>
                    )}
                  </h2>
                  <p className="text-sm text-muted-foreground mt-1">
                    {selectedModel
                      ? 'Runtimes are sorted by compatibility with your selected model'
                      : 'Select a model above to enable smart runtime recommendations'}
                  </p>
                </div>
                {compatibleData?.matches && (
                  <div className="text-sm text-muted-foreground">
                    {compatibleData.matches.length} compatible runtime{compatibleData.matches.length !== 1 ? 's' : ''}
                  </div>
                )}
              </div>
            </div>
            <div className="p-6">
              {compatibleLoading ? (
                <div className="flex items-center justify-center py-12">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
                </div>
              ) : sortedRuntimes.length === 0 ? (
                <div className="text-center py-12">
                  <svg className="mx-auto h-12 w-12 text-muted-foreground/40" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z" />
                  </svg>
                  <p className="mt-4 text-sm text-muted-foreground">No runtimes available</p>
                  <Button href="/runtimes/new" variant="outline" size="sm" className="mt-4" icon={ButtonIcons.plus}>
                    Create Runtime
                  </Button>
                </div>
              ) : (
                <div className="space-y-3">
                  {sortedRuntimes.map((match) => (
                    <RuntimeCard
                      key={match.runtime.metadata.name}
                      match={match}
                      isSelected={watchedRuntime === match.runtime.metadata.name}
                      isRecommended={recommendedRuntime?.runtime.metadata.name === match.runtime.metadata.name}
                      onSelect={() => setValue('runtime', match.runtime.metadata.name)}
                    />
                  ))}
                </div>
              )}
              {errors.runtime && (
                <p className="mt-2 text-sm text-destructive">{errors.runtime.message}</p>
              )}
            </div>
          </section>

          {/* Scaling Configuration */}
          <section className="rounded-xl border border-border bg-card shadow-sm overflow-hidden">
            <div className="border-b border-border bg-muted/30 px-6 py-4">
              <h2 className="text-lg font-semibold">Scaling Configuration</h2>
            </div>
            <div className="p-6">
              <div className="grid grid-cols-1 gap-6 sm:grid-cols-3">
                <div>
                  <label htmlFor="replicas" className="block text-sm font-medium mb-2">
                    Replicas
                  </label>
                  <input
                    id="replicas"
                    type="number"
                    {...register('replicas', { valueAsNumber: true })}
                    min={1}
                    max={100}
                    className="w-full rounded-lg border border-border bg-background px-4 py-2.5 text-sm shadow-sm transition-colors focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20"
                  />
                  {errors.replicas && (
                    <p className="mt-1 text-sm text-destructive">{errors.replicas.message}</p>
                  )}
                </div>

                <div>
                  <label htmlFor="minReplicas" className="block text-sm font-medium mb-2">
                    Min Replicas (Auto-scale)
                  </label>
                  <input
                    id="minReplicas"
                    type="number"
                    {...register('minReplicas', { valueAsNumber: true })}
                    min={0}
                    max={100}
                    placeholder="0"
                    className="w-full rounded-lg border border-border bg-background px-4 py-2.5 text-sm shadow-sm transition-colors focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20"
                  />
                </div>

                <div>
                  <label htmlFor="maxReplicas" className="block text-sm font-medium mb-2">
                    Max Replicas (Auto-scale)
                  </label>
                  <input
                    id="maxReplicas"
                    type="number"
                    {...register('maxReplicas', { valueAsNumber: true })}
                    min={1}
                    max={100}
                    placeholder="10"
                    className="w-full rounded-lg border border-border bg-background px-4 py-2.5 text-sm shadow-sm transition-colors focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20"
                  />
                </div>
              </div>
              <p className="mt-4 text-sm text-muted-foreground">
                Set min/max replicas to enable auto-scaling. Leave empty to use fixed replica count.
              </p>
            </div>
          </section>

          {/* Actions */}
          <div className="flex items-center justify-end gap-3 pt-4">
            <Button href="/services" variant="outline">
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={isSubmitting || createService.isPending}
              icon={isSubmitting || createService.isPending ? undefined : ButtonIcons.check}
            >
              {isSubmitting || createService.isPending ? 'Deploying...' : 'Deploy Service'}
            </Button>
          </div>
        </form>
      </main>
    </div>
  )
}
