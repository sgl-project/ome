'use client'

import { useForm, useFieldArray, FieldValues } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { clusterServingRuntimeSchema } from '@/lib/validation/runtime-schema'
import Link from 'next/link'
import { useState, useEffect } from 'react'
import { ContainerForm } from '@/components/forms/ContainerForm'
import { VolumeForm } from '@/components/forms/VolumeForm'
import type { ClusterServingRuntime } from '@/lib/types/runtime'

// Runtime form data type - more permissive for complex nested forms
type RuntimeFormData = FieldValues

interface RuntimeFormProps {
  mode: 'create' | 'edit'
  initialData?: ClusterServingRuntime
  onSubmit: (data: any) => Promise<void>
  isLoading?: boolean
  backLink: string
  backLinkText: string
}

const defaultValues = {
  apiVersion: 'ome.io/v1beta1',
  kind: 'ClusterServingRuntime',
  metadata: {
    name: '',
  },
  spec: {
    supportedModelFormats: [{ name: '' }],
    disabled: false,
    protocolVersions: [],
  },
}

export function RuntimeForm({
  mode,
  initialData,
  onSubmit,
  isLoading = false,
  backLink,
  backLinkText,
}: RuntimeFormProps) {
  const [error, setError] = useState<string | null>(null)
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({
    basic: true,
    'model-formats': mode === 'create',
    engine: false,
    decoder: false,
    router: false,
  })

  const [engineMultiNode, setEngineMultiNode] = useState(false)
  const [decoderMultiNode, setDecoderMultiNode] = useState(false)

  const {
    register,
    handleSubmit,
    control,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<RuntimeFormData>({
    resolver: zodResolver(clusterServingRuntimeSchema) as any,
    defaultValues,
  })

  // Field arrays
  const {
    fields: formatFields,
    append: appendFormat,
    remove: removeFormat,
  } = useFieldArray({ control, name: 'spec.supportedModelFormats' })

  const {
    fields: protocolFields,
    append: appendProtocol,
    remove: removeProtocol,
  } = useFieldArray({ control, name: 'spec.protocolVersions' })

  const {
    fields: engineInitContainerFields,
    append: appendEngineInitContainer,
    remove: removeEngineInitContainer,
  } = useFieldArray({ control, name: 'spec.engineConfig.initContainers' })

  const {
    fields: engineSidecarFields,
    append: appendEngineSidecar,
    remove: removeEngineSidecar,
  } = useFieldArray({ control, name: 'spec.engineConfig.sidecars' })

  const {
    fields: decoderInitContainerFields,
    append: appendDecoderInitContainer,
    remove: removeDecoderInitContainer,
  } = useFieldArray({ control, name: 'spec.decoderConfig.initContainers' })

  const {
    fields: decoderSidecarFields,
    append: appendDecoderSidecar,
    remove: removeDecoderSidecar,
  } = useFieldArray({ control, name: 'spec.decoderConfig.sidecars' })

  const {
    fields: routerInitContainerFields,
    append: appendRouterInitContainer,
    remove: removeRouterInitContainer,
  } = useFieldArray({ control, name: 'spec.routerConfig.initContainers' })

  const {
    fields: routerSidecarFields,
    append: appendRouterSidecar,
    remove: removeRouterSidecar,
  } = useFieldArray({ control, name: 'spec.routerConfig.sidecars' })

  // Pre-populate form when initial data is provided (edit mode)
  useEffect(() => {
    if (initialData) {
      reset({
        apiVersion: initialData.apiVersion || 'ome.io/v1beta1',
        kind: initialData.kind || 'ClusterServingRuntime',
        metadata: {
          name: initialData.metadata?.name || '',
          namespace: initialData.metadata?.namespace,
        },
        spec: {
          supportedModelFormats: initialData.spec?.supportedModelFormats?.length
            ? initialData.spec.supportedModelFormats
            : [{ name: '' }],
          disabled: initialData.spec?.disabled || false,
          modelSizeRange: initialData.spec?.modelSizeRange,
          routerConfig: initialData.spec?.routerConfig,
          engineConfig: initialData.spec?.engineConfig,
          decoderConfig: initialData.spec?.decoderConfig,
          protocolVersions: initialData.spec?.protocolVersions || [],
        },
      })

      if (initialData.spec?.engineConfig?.leader || initialData.spec?.engineConfig?.worker) {
        setEngineMultiNode(true)
      }
      if (initialData.spec?.decoderConfig?.leader || initialData.spec?.decoderConfig?.worker) {
        setDecoderMultiNode(true)
      }
    }
  }, [initialData, reset])

  const toggleSection = (sectionId: string) => {
    setExpandedSections((prev) => ({ ...prev, [sectionId]: !prev[sectionId] }))
  }

  const handleFormSubmit = async (data: any) => {
    try {
      setError(null)
      await onSubmit(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${mode} runtime`)
    }
  }

  const isEditMode = mode === 'edit'

  const AccordionSection = ({
    id,
    title,
    children,
  }: {
    id: string
    title: string
    children: React.ReactNode
  }) => {
    const isExpanded = expandedSections[id]
    return (
      <div className="section-card rounded-2xl overflow-hidden">
        <button
          type="button"
          onClick={() => toggleSection(id)}
          className="w-full flex items-center justify-between p-6 text-left hover:bg-slate-50/50 transition-colors"
        >
          <h2 className="text-xl font-display font-semibold text-slate-900">{title}</h2>
          <svg
            className={`h-6 w-6 text-slate-400 transition-transform duration-200 ${isExpanded ? 'rotate-180' : ''}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        {isExpanded && (
          <div className="border-t border-slate-200 p-6 animate-in slide-in-from-top-2 duration-300">
            {children}
          </div>
        )}
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-50 to-slate-100">
        <div className="flex flex-col items-center gap-3">
          <div className="h-10 w-10 animate-spin rounded-full border-4 border-slate-200 border-t-purple-600"></div>
          <p className="text-sm font-medium text-slate-600">Loading runtime configuration...</p>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 via-slate-100 to-slate-50">
      <style jsx global>{`
        @import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600&family=Space+Grotesk:wght@400;500;600;700&display=swap');
        .font-display {
          font-family: 'Space Grotesk', sans-serif;
        }
        .font-mono {
          font-family: 'JetBrains Mono', monospace;
        }
        .input-focus {
          transition: all 0.2s ease;
        }
        .input-focus:focus {
          transform: translateY(-1px);
          box-shadow: 0 4px 12px rgba(147, 51, 234, 0.15);
        }
        .section-card {
          background: linear-gradient(
            135deg,
            rgba(255, 255, 255, 0.9) 0%,
            rgba(255, 255, 255, 0.95) 100%
          );
          backdrop-filter: blur(10px);
          border: 1px solid rgba(148, 163, 184, 0.1);
          box-shadow:
            0 2px 8px rgba(0, 0, 0, 0.04),
            0 0 0 1px rgba(148, 163, 184, 0.05);
        }
        .field-label {
          font-weight: 500;
          letter-spacing: -0.01em;
        }
      `}</style>

      {/* Header */}
      <header className="border-b border-slate-200 bg-white/80 backdrop-blur-md shadow-sm sticky top-0 z-10">
        <div className="mx-auto max-w-7xl px-6 py-6">
          <Link
            href={backLink}
            className="group inline-flex items-center gap-2 text-sm font-medium text-slate-600 hover:text-purple-600 transition-colors mb-3"
          >
            <span className="transition-transform group-hover:-translate-x-1">←</span>
            <span>{backLinkText}</span>
          </Link>
          <h1 className="text-4xl font-display font-bold text-slate-900 tracking-tight">
            {isEditMode ? 'Edit Runtime' : 'Create New Runtime'}
          </h1>
          <p className="mt-2 text-sm text-slate-600 font-medium">
            {isEditMode ? (
              <>
                Configure{' '}
                <span className="font-mono text-purple-600">{initialData?.metadata?.name}</span>{' '}
                runtime settings
              </>
            ) : (
              <>
                Define a new{' '}
                <span className="font-mono text-purple-600">ClusterServingRuntime</span> resource
              </>
            )}
          </p>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-6 py-8">
        {error && (
          <div className="mb-6 rounded-xl bg-red-50 border border-red-200 p-4 animate-in slide-in-from-top-2">
            <p className="text-sm font-medium text-red-800">{error}</p>
          </div>
        )}

        <form onSubmit={handleSubmit(handleFormSubmit)} className="space-y-4">
          {/* Basic Information */}
          <AccordionSection id="basic" title="Basic Information">
            <div className="space-y-6">
              <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                <div>
                  <label htmlFor="name" className="field-label block text-sm text-slate-700 mb-2">
                    Name *
                  </label>
                  <input
                    type="text"
                    id="name"
                    {...register('metadata.name')}
                    disabled={isEditMode}
                    className={`input-focus w-full rounded-lg border px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20 ${
                      isEditMode
                        ? 'border-slate-200 bg-slate-50 text-slate-500 cursor-not-allowed'
                        : 'border-slate-300'
                    }`}
                    placeholder="my-runtime"
                  />
                  {isEditMode ? (
                    <p className="mt-1.5 text-xs text-slate-500">
                      Name cannot be changed after creation
                    </p>
                  ) : (
                    (errors.metadata as any)?.name && (
                      <p className="mt-1.5 text-xs text-red-600">
                        {(errors.metadata as any).name.message as string}
                      </p>
                    )
                  )}
                </div>

                <div>
                  <label
                    htmlFor="namespace"
                    className="field-label block text-sm text-slate-700 mb-2"
                  >
                    Namespace
                  </label>
                  <input
                    type="text"
                    id="namespace"
                    {...register('metadata.namespace')}
                    disabled={isEditMode}
                    className={`input-focus w-full rounded-lg border px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20 ${
                      isEditMode
                        ? 'border-slate-200 bg-slate-50 text-slate-500 cursor-not-allowed'
                        : 'border-slate-300'
                    }`}
                    placeholder={isEditMode ? 'default' : 'Leave empty for cluster-scoped'}
                  />
                  <p className="mt-1.5 text-xs text-slate-500">
                    {isEditMode
                      ? 'Namespace cannot be changed'
                      : 'Leave empty for ClusterServingRuntime (cluster-scoped)'}
                  </p>
                </div>

                <div className="flex items-center gap-3 p-4 rounded-lg bg-slate-50 border border-slate-200">
                  <input
                    type="checkbox"
                    id="disabled"
                    {...register('spec.disabled')}
                    className="h-5 w-5 rounded border-slate-300 text-purple-600 focus:ring-purple-500 focus:ring-offset-2 transition-all"
                  />
                  <label htmlFor="disabled" className="flex-1">
                    <span className="field-label text-sm text-slate-700 block">Disabled</span>
                    <span className="text-xs text-slate-500">
                      Disable this runtime from being selected
                    </span>
                  </label>
                </div>
              </div>

              <div>
                <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                  Model Size Range
                </h3>
                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Minimum Size
                    </label>
                    <input
                      type="text"
                      {...register('spec.modelSizeRange.min')}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                      placeholder="e.g., 100MB"
                    />
                  </div>
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Maximum Size
                    </label>
                    <input
                      type="text"
                      {...register('spec.modelSizeRange.max')}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                      placeholder="e.g., 10GB"
                    />
                  </div>
                </div>
              </div>

              <div>
                <div className="mb-4 flex items-center justify-between">
                  <h3 className="text-base font-display font-semibold text-slate-700">
                    Protocol Versions
                  </h3>
                  <button
                    type="button"
                    onClick={() => appendProtocol('')}
                    className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-sm font-medium text-white shadow-sm hover:shadow-md transition-all"
                  >
                    + Add Protocol
                  </button>
                </div>
                <div className="space-y-3">
                  {protocolFields.map((field, index) => (
                    <div key={field.id} className="flex items-center gap-3">
                      <input
                        type="text"
                        {...register(`spec.protocolVersions.${index}` as const)}
                        className="input-focus flex-1 rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                        placeholder="e.g., openai, cohere"
                      />
                      <button
                        type="button"
                        onClick={() => removeProtocol(index)}
                        className="rounded-lg border border-red-200 bg-red-50 px-4 py-2 text-sm font-medium text-red-600 hover:bg-red-100 transition-colors"
                      >
                        Remove
                      </button>
                    </div>
                  ))}
                  {protocolFields.length === 0 && (
                    <p className="text-sm text-slate-500 italic">No protocols defined</p>
                  )}
                </div>
              </div>
            </div>
          </AccordionSection>

          {/* Model Formats */}
          <AccordionSection id="model-formats" title="Supported Model Formats">
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-sm text-slate-600">
                  Define the model formats this runtime can execute
                </p>
                <button
                  type="button"
                  onClick={() => appendFormat({ name: '' })}
                  className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-sm font-medium text-white shadow-sm hover:shadow-md transition-all"
                >
                  + Add Format
                </button>
              </div>

              <div className="space-y-4">
                {formatFields.map((field, index) => (
                  <div
                    key={field.id}
                    className="rounded-xl border border-slate-200 bg-white/50 p-5"
                  >
                    <div className="mb-4 flex items-center justify-between">
                      <h4 className="text-sm font-display font-semibold text-slate-700">
                        Format {index + 1}
                      </h4>
                      {formatFields.length > 1 && (
                        <button
                          type="button"
                          onClick={() => removeFormat(index)}
                          className="text-sm font-medium text-red-600 hover:text-red-800 transition-colors"
                        >
                          Remove
                        </button>
                      )}
                    </div>

                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                      <div>
                        <label className="field-label block text-sm text-slate-700 mb-2">
                          Format Name *
                        </label>
                        <select
                          {...register(`spec.supportedModelFormats.${index}.name` as const)}
                          className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                        >
                          <option value="">Select format...</option>
                          <option value="safetensors">SafeTensors</option>
                          <option value="pytorch">PyTorch</option>
                          <option value="onnx">ONNX</option>
                          <option value="tensorflow">TensorFlow</option>
                          <option value="huggingface">HuggingFace</option>
                        </select>
                        {(errors.spec as any)?.supportedModelFormats?.[index]?.name && (
                          <p className="mt-1 text-xs text-red-600">
                            {
                              (errors.spec as any).supportedModelFormats[index]?.name
                                ?.message as string
                            }
                          </p>
                        )}
                      </div>

                      <div>
                        <label className="field-label block text-sm text-slate-700 mb-2">
                          Version
                        </label>
                        <input
                          type="text"
                          {...register(`spec.supportedModelFormats.${index}.version` as const)}
                          className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                          placeholder="1.0"
                        />
                      </div>

                      <div>
                        <label className="field-label block text-sm text-slate-700 mb-2">
                          Model Type
                        </label>
                        <input
                          type="text"
                          {...register(`spec.supportedModelFormats.${index}.modelType` as const)}
                          className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                          placeholder="text-generation"
                        />
                      </div>

                      <div>
                        <label className="field-label block text-sm text-slate-700 mb-2">
                          Model Architecture
                        </label>
                        <input
                          type="text"
                          {...register(
                            `spec.supportedModelFormats.${index}.modelArchitecture` as const
                          )}
                          className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                          placeholder="LlamaForCausalLM"
                        />
                      </div>

                      <div>
                        <label className="field-label block text-sm text-slate-700 mb-2">
                          Quantization
                        </label>
                        <select
                          {...register(`spec.supportedModelFormats.${index}.quantization` as const)}
                          className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                        >
                          <option value="">None</option>
                          <option value="fp8">FP8</option>
                          <option value="fbgemm_fp8">FBGEMM FP8</option>
                          <option value="int4">INT4</option>
                        </select>
                      </div>

                      <div>
                        <label className="field-label block text-sm text-slate-700 mb-2">
                          Priority
                        </label>
                        <input
                          type="number"
                          {...register(`spec.supportedModelFormats.${index}.priority` as const, {
                            valueAsNumber: true,
                          })}
                          className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                          placeholder="0"
                        />
                      </div>

                      <div className="flex items-center gap-3 p-3 rounded-lg bg-slate-50">
                        <input
                          type="checkbox"
                          {...register(`spec.supportedModelFormats.${index}.autoSelect` as const)}
                          className="h-4 w-4 rounded border-slate-300 text-purple-600 focus:ring-purple-500"
                        />
                        <label className="text-sm text-slate-700 font-medium">Auto Select</label>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </AccordionSection>

          {/* Engine Configuration */}
          <AccordionSection id="engine" title="Engine Configuration">
            <div className="space-y-8">
              <p className="text-sm text-slate-600">Configure the inference engine component</p>

              {/* Scaling Configuration */}
              <div>
                <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                  Scaling Configuration
                </h3>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Min Replicas
                    </label>
                    <input
                      type="number"
                      {...register('spec.engineConfig.minReplicas', { valueAsNumber: true })}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                      placeholder="0"
                    />
                  </div>
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Max Replicas
                    </label>
                    <input
                      type="number"
                      {...register('spec.engineConfig.maxReplicas', { valueAsNumber: true })}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                      placeholder="5"
                    />
                  </div>
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Scale Target
                    </label>
                    <input
                      type="number"
                      {...register('spec.engineConfig.scaleTarget', { valueAsNumber: true })}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                      placeholder="80"
                    />
                  </div>
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Scale Metric
                    </label>
                    <select
                      {...register('spec.engineConfig.scaleMetric')}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                    >
                      <option value="">Select metric...</option>
                      <option value="cpu">CPU</option>
                      <option value="memory">Memory</option>
                      <option value="concurrency">Concurrency</option>
                      <option value="rps">RPS</option>
                    </select>
                  </div>
                </div>
              </div>

              {/* Runner */}
              <div>
                <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                  Runner (Main Container)
                </h3>
                <ContainerForm
                  basePath="spec.engineConfig.runner"
                  register={register}
                  control={control}
                />
              </div>

              {/* Multi-Node Toggle */}
              <div className="rounded-lg bg-purple-50 border border-purple-200 p-4">
                <label className="flex items-center gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={engineMultiNode}
                    onChange={(e) => setEngineMultiNode(e.target.checked)}
                    className="h-5 w-5 rounded border-slate-300 text-purple-600 focus:ring-purple-500"
                  />
                  <div>
                    <span className="field-label text-sm text-slate-700 block">
                      Enable Multi-Node Deployment
                    </span>
                    <span className="text-xs text-slate-600">
                      Configure leader and worker nodes for distributed inference
                    </span>
                  </div>
                </label>
              </div>

              {engineMultiNode && (
                <>
                  <div>
                    <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                      Leader Node Configuration
                    </h3>
                    <p className="text-xs text-slate-500 mb-4">
                      Coordinates distributed inference across worker nodes
                    </p>
                    <ContainerForm
                      basePath="spec.engineConfig.leader.runner"
                      register={register}
                      control={control}
                    />
                  </div>
                  <div>
                    <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                      Worker Node Configuration
                    </h3>
                    <p className="text-xs text-slate-500 mb-4">
                      Performs distributed processing tasks
                    </p>
                    <div className="mb-4">
                      <label className="field-label block text-sm text-slate-700 mb-2">
                        Worker Size (Number of Pods)
                      </label>
                      <input
                        type="number"
                        {...register('spec.engineConfig.worker.size', { valueAsNumber: true })}
                        className="input-focus w-full max-w-xs rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                        placeholder="1"
                      />
                    </div>
                    <ContainerForm
                      basePath="spec.engineConfig.worker.runner"
                      register={register}
                      control={control}
                    />
                  </div>
                </>
              )}

              <VolumeForm
                basePath="spec.engineConfig.volumes"
                register={register}
                control={control}
              />

              {/* Init Containers */}
              <div>
                <div className="mb-3 flex items-center justify-between">
                  <div>
                    <h5 className="text-sm font-display font-semibold text-slate-700">
                      Init Containers
                    </h5>
                    <p className="text-xs text-slate-500 mt-1">
                      Containers that run before the main container starts
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => appendEngineInitContainer({ name: '', image: '' })}
                    className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
                  >
                    + Add Init Container
                  </button>
                </div>
                <div className="space-y-4">
                  {engineInitContainerFields.map((field, index) => (
                    <ContainerForm
                      key={field.id}
                      basePath={`spec.engineConfig.initContainers.${index}`}
                      register={register}
                      control={control}
                      showRemove={true}
                      onRemove={() => removeEngineInitContainer(index)}
                      title={`Init Container ${index + 1}`}
                    />
                  ))}
                  {engineInitContainerFields.length === 0 && (
                    <p className="text-xs text-slate-500 italic">No init containers defined</p>
                  )}
                </div>
              </div>

              {/* Sidecar Containers */}
              <div>
                <div className="mb-3 flex items-center justify-between">
                  <div>
                    <h5 className="text-sm font-display font-semibold text-slate-700">
                      Sidecar Containers
                    </h5>
                    <p className="text-xs text-slate-500 mt-1">
                      Containers that run alongside the main container
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => appendEngineSidecar({ name: '', image: '' })}
                    className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
                  >
                    + Add Sidecar
                  </button>
                </div>
                <div className="space-y-4">
                  {engineSidecarFields.map((field, index) => (
                    <ContainerForm
                      key={field.id}
                      basePath={`spec.engineConfig.sidecars.${index}`}
                      register={register}
                      control={control}
                      showRemove={true}
                      onRemove={() => removeEngineSidecar(index)}
                      title={`Sidecar ${index + 1}`}
                    />
                  ))}
                  {engineSidecarFields.length === 0 && (
                    <p className="text-xs text-slate-500 italic">No sidecar containers defined</p>
                  )}
                </div>
              </div>
            </div>
          </AccordionSection>

          {/* Decoder Configuration */}
          <AccordionSection id="decoder" title="Decoder Configuration">
            <div className="space-y-8">
              <p className="text-sm text-slate-600">
                Configure the decoder component for prefill-decode disaggregated deployments
              </p>

              <div>
                <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                  Scaling Configuration
                </h3>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Min Replicas
                    </label>
                    <input
                      type="number"
                      {...register('spec.decoderConfig.minReplicas', { valueAsNumber: true })}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                      placeholder="0"
                    />
                  </div>
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Max Replicas
                    </label>
                    <input
                      type="number"
                      {...register('spec.decoderConfig.maxReplicas', { valueAsNumber: true })}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                      placeholder="5"
                    />
                  </div>
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Scale Target
                    </label>
                    <input
                      type="number"
                      {...register('spec.decoderConfig.scaleTarget', { valueAsNumber: true })}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                      placeholder="80"
                    />
                  </div>
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Scale Metric
                    </label>
                    <select
                      {...register('spec.decoderConfig.scaleMetric')}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                    >
                      <option value="">Select metric...</option>
                      <option value="cpu">CPU</option>
                      <option value="memory">Memory</option>
                      <option value="concurrency">Concurrency</option>
                      <option value="rps">RPS</option>
                    </select>
                  </div>
                </div>
              </div>

              <div>
                <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                  Runner (Main Container)
                </h3>
                <ContainerForm
                  basePath="spec.decoderConfig.runner"
                  register={register}
                  control={control}
                />
              </div>

              <div className="rounded-lg bg-purple-50 border border-purple-200 p-4">
                <label className="flex items-center gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={decoderMultiNode}
                    onChange={(e) => setDecoderMultiNode(e.target.checked)}
                    className="h-5 w-5 rounded border-slate-300 text-purple-600 focus:ring-purple-500"
                  />
                  <div>
                    <span className="field-label text-sm text-slate-700 block">
                      Enable Multi-Node Deployment
                    </span>
                    <span className="text-xs text-slate-600">
                      Configure leader and worker nodes for distributed token generation
                    </span>
                  </div>
                </label>
              </div>

              {decoderMultiNode && (
                <>
                  <div>
                    <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                      Leader Node Configuration
                    </h3>
                    <p className="text-xs text-slate-500 mb-4">
                      Coordinates distributed token generation across worker nodes
                    </p>
                    <ContainerForm
                      basePath="spec.decoderConfig.leader.runner"
                      register={register}
                      control={control}
                    />
                  </div>
                  <div>
                    <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                      Worker Node Configuration
                    </h3>
                    <p className="text-xs text-slate-500 mb-4">
                      Performs distributed token generation tasks
                    </p>
                    <div className="mb-4">
                      <label className="field-label block text-sm text-slate-700 mb-2">
                        Worker Size (Number of Pods)
                      </label>
                      <input
                        type="number"
                        {...register('spec.decoderConfig.worker.size', { valueAsNumber: true })}
                        className="input-focus w-full max-w-xs rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                        placeholder="1"
                      />
                    </div>
                    <ContainerForm
                      basePath="spec.decoderConfig.worker.runner"
                      register={register}
                      control={control}
                    />
                  </div>
                </>
              )}

              <VolumeForm
                basePath="spec.decoderConfig.volumes"
                register={register}
                control={control}
              />

              <div>
                <div className="mb-3 flex items-center justify-between">
                  <div>
                    <h5 className="text-sm font-display font-semibold text-slate-700">
                      Init Containers
                    </h5>
                    <p className="text-xs text-slate-500 mt-1">
                      Containers that run before the main container starts
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => appendDecoderInitContainer({ name: '', image: '' })}
                    className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
                  >
                    + Add Init Container
                  </button>
                </div>
                <div className="space-y-4">
                  {decoderInitContainerFields.map((field, index) => (
                    <ContainerForm
                      key={field.id}
                      basePath={`spec.decoderConfig.initContainers.${index}`}
                      register={register}
                      control={control}
                      showRemove={true}
                      onRemove={() => removeDecoderInitContainer(index)}
                      title={`Init Container ${index + 1}`}
                    />
                  ))}
                  {decoderInitContainerFields.length === 0 && (
                    <p className="text-xs text-slate-500 italic">No init containers defined</p>
                  )}
                </div>
              </div>

              <div>
                <div className="mb-3 flex items-center justify-between">
                  <div>
                    <h5 className="text-sm font-display font-semibold text-slate-700">
                      Sidecar Containers
                    </h5>
                    <p className="text-xs text-slate-500 mt-1">
                      Containers that run alongside the main container
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => appendDecoderSidecar({ name: '', image: '' })}
                    className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
                  >
                    + Add Sidecar
                  </button>
                </div>
                <div className="space-y-4">
                  {decoderSidecarFields.map((field, index) => (
                    <ContainerForm
                      key={field.id}
                      basePath={`spec.decoderConfig.sidecars.${index}`}
                      register={register}
                      control={control}
                      showRemove={true}
                      onRemove={() => removeDecoderSidecar(index)}
                      title={`Sidecar ${index + 1}`}
                    />
                  ))}
                  {decoderSidecarFields.length === 0 && (
                    <p className="text-xs text-slate-500 italic">No sidecar containers defined</p>
                  )}
                </div>
              </div>
            </div>
          </AccordionSection>

          {/* Router Configuration */}
          <AccordionSection id="router" title="Router Configuration">
            <div className="space-y-8">
              <p className="text-sm text-slate-600">
                Configure the router component for request routing and load balancing
              </p>

              <div>
                <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                  Scaling Configuration
                </h3>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Min Replicas
                    </label>
                    <input
                      type="number"
                      {...register('spec.routerConfig.minReplicas', { valueAsNumber: true })}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                      placeholder="1"
                    />
                  </div>
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Max Replicas
                    </label>
                    <input
                      type="number"
                      {...register('spec.routerConfig.maxReplicas', { valueAsNumber: true })}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                      placeholder="5"
                    />
                  </div>
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Scale Target
                    </label>
                    <input
                      type="number"
                      {...register('spec.routerConfig.scaleTarget', { valueAsNumber: true })}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                      placeholder="80"
                    />
                  </div>
                  <div>
                    <label className="field-label block text-sm text-slate-700 mb-2">
                      Scale Metric
                    </label>
                    <select
                      {...register('spec.routerConfig.scaleMetric')}
                      className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20"
                    >
                      <option value="">Select metric...</option>
                      <option value="cpu">CPU</option>
                      <option value="memory">Memory</option>
                      <option value="concurrency">Concurrency</option>
                      <option value="rps">RPS</option>
                    </select>
                  </div>
                </div>
              </div>

              <div>
                <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                  Runner (Main Container)
                </h3>
                <ContainerForm
                  basePath="spec.routerConfig.runner"
                  register={register}
                  control={control}
                />
              </div>

              <div>
                <h3 className="text-base font-display font-semibold text-slate-700 mb-4">
                  Router Configuration Parameters
                </h3>
                <p className="text-xs text-slate-500 mb-3">
                  Additional configuration parameters as key-value pairs (JSON format)
                </p>
                <textarea
                  {...register('spec.routerConfig.config')}
                  className="input-focus w-full rounded-lg border border-slate-300 px-4 py-2.5 text-sm font-mono shadow-sm focus:border-purple-500 focus:outline-none focus:ring-2 focus:ring-purple-500/20 min-h-[100px]"
                  placeholder={`{\n  "timeout": "30s",\n  "maxConnections": "1000"\n}`}
                />
              </div>

              <VolumeForm
                basePath="spec.routerConfig.volumes"
                register={register}
                control={control}
              />

              <div>
                <div className="mb-3 flex items-center justify-between">
                  <div>
                    <h5 className="text-sm font-display font-semibold text-slate-700">
                      Init Containers
                    </h5>
                    <p className="text-xs text-slate-500 mt-1">
                      Containers that run before the main container starts
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => appendRouterInitContainer({ name: '', image: '' })}
                    className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
                  >
                    + Add Init Container
                  </button>
                </div>
                <div className="space-y-4">
                  {routerInitContainerFields.map((field, index) => (
                    <ContainerForm
                      key={field.id}
                      basePath={`spec.routerConfig.initContainers.${index}`}
                      register={register}
                      control={control}
                      showRemove={true}
                      onRemove={() => removeRouterInitContainer(index)}
                      title={`Init Container ${index + 1}`}
                    />
                  ))}
                  {routerInitContainerFields.length === 0 && (
                    <p className="text-xs text-slate-500 italic">No init containers defined</p>
                  )}
                </div>
              </div>

              <div>
                <div className="mb-3 flex items-center justify-between">
                  <div>
                    <h5 className="text-sm font-display font-semibold text-slate-700">
                      Sidecar Containers
                    </h5>
                    <p className="text-xs text-slate-500 mt-1">
                      Containers that run alongside the main container
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => appendRouterSidecar({ name: '', image: '' })}
                    className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:shadow-md transition-all"
                  >
                    + Add Sidecar
                  </button>
                </div>
                <div className="space-y-4">
                  {routerSidecarFields.map((field, index) => (
                    <ContainerForm
                      key={field.id}
                      basePath={`spec.routerConfig.sidecars.${index}`}
                      register={register}
                      control={control}
                      showRemove={true}
                      onRemove={() => removeRouterSidecar(index)}
                      title={`Sidecar ${index + 1}`}
                    />
                  ))}
                  {routerSidecarFields.length === 0 && (
                    <p className="text-xs text-slate-500 italic">No sidecar containers defined</p>
                  )}
                </div>
              </div>
            </div>
          </AccordionSection>

          {/* Form Actions */}
          <div className="section-card rounded-2xl p-4 flex justify-end gap-3 sticky bottom-4">
            <Link
              href={backLink}
              className="rounded-lg border-2 border-slate-300 px-6 py-2.5 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-all"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={isSubmitting}
              className="rounded-lg bg-gradient-to-br from-purple-600 to-purple-700 px-6 py-2.5 text-sm font-medium text-white shadow-lg shadow-purple-500/30 hover:shadow-xl hover:shadow-purple-500/40 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
            >
              {isSubmitting
                ? isEditMode
                  ? 'Updating...'
                  : 'Creating...'
                : isEditMode
                  ? 'Update Runtime'
                  : 'Create Runtime'}
            </button>
          </div>
        </form>
      </main>
    </div>
  )
}
