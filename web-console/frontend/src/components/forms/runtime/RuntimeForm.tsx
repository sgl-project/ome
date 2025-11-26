'use client'

import { useForm, FieldValues } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { clusterServingRuntimeSchema } from '@/lib/validation/runtime-schema'
import Link from 'next/link'
import { useState, useEffect, useMemo } from 'react'
import { RuntimeFormProvider } from './RuntimeFormContext'
import {
  BasicInfoSection,
  ModelFormatsSection,
  EngineConfigSection,
  DecoderConfigSection,
  RouterConfigSection,
} from './sections'
import { exportAsYaml } from '@/lib/utils'
import { Icons } from '@/components/ui/Icons'
import { Spinner } from '@/components/ui/Spinner'
import type { ClusterServingRuntime } from '@/lib/types/runtime'

interface RuntimeFormProps {
  mode: 'create' | 'edit'
  initialData?: ClusterServingRuntime
  onSubmit: (data: any) => Promise<void>
  isLoading?: boolean
  backLink: string
  backLinkText: string
  /** When set, indicates this is a clone operation from the specified runtime */
  cloneFrom?: string
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

/**
 * RuntimeForm - Main orchestrator component for creating/editing ClusterServingRuntime resources.
 *
 * This component has been refactored from 1200+ lines to ~150 lines by:
 * 1. Extracting sections into separate components (BasicInfo, ModelFormats, Engine, Decoder, Router)
 * 2. Using shared components for repeated patterns (ScalingConfig, MultiNodeConfig, ContainerListSection)
 * 3. Using RuntimeFormContext to share form state across sections
 *
 * The form uses react-hook-form with Zod validation.
 */
export function RuntimeForm({
  mode,
  initialData,
  onSubmit,
  isLoading = false,
  backLink,
  backLinkText,
  cloneFrom,
}: RuntimeFormProps) {
  const [error, setError] = useState<string | null>(null)

  // Compute initial multi-node states from initialData immediately (not in useEffect)
  // This ensures the provider receives the correct initial values on first render
  const initialEngineMultiNode = useMemo(() => {
    return !!(initialData?.spec?.engineConfig?.leader || initialData?.spec?.engineConfig?.worker)
  }, [initialData])

  const initialDecoderMultiNode = useMemo(() => {
    return !!(initialData?.spec?.decoderConfig?.leader || initialData?.spec?.decoderConfig?.worker)
  }, [initialData])

  const form = useForm<FieldValues>({
    resolver: zodResolver(clusterServingRuntimeSchema) as any,
    defaultValues,
  })

  const {
    handleSubmit,
    reset,
    getValues,
    formState: { isSubmitting },
  } = form

  const handleExportYaml = () => {
    const data = getValues()
    const filename = data.metadata?.name || 'runtime'
    exportAsYaml(data, `${filename}.yaml`)
  }

  // Pre-populate form when initial data is provided (edit/clone mode)
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
    }
  }, [initialData, reset])

  const handleFormSubmit = async (data: any) => {
    try {
      setError(null)
      await onSubmit(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to ${mode} runtime`)
    }
  }

  const isEditMode = mode === 'edit'
  const isCloneMode = mode === 'create' && !!cloneFrom

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-50 to-slate-100">
        <div className="flex flex-col items-center gap-3">
          <Spinner size="lg" className="text-purple-600" />
          <p className="text-sm font-medium text-slate-600">Loading runtime configuration...</p>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 via-slate-100 to-slate-50">
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
          <h1 className="text-4xl font-bold text-slate-900 tracking-tight">
            {isEditMode ? 'Edit Runtime' : isCloneMode ? 'Clone Runtime' : 'Create New Runtime'}
          </h1>
          <p className="mt-2 text-sm text-slate-600 font-medium">
            {isEditMode ? (
              <>
                Configure{' '}
                <span className="font-mono text-purple-600">{initialData?.metadata?.name}</span>{' '}
                runtime settings
              </>
            ) : isCloneMode ? (
              <>
                Create a new runtime based on{' '}
                <span className="font-mono text-purple-600">{cloneFrom}</span>
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

        <RuntimeFormProvider
          form={form}
          isEditMode={isEditMode}
          initialEngineMultiNode={initialEngineMultiNode}
          initialDecoderMultiNode={initialDecoderMultiNode}
        >
          <form onSubmit={handleSubmit(handleFormSubmit)} className="space-y-4">
            <BasicInfoSection />
            <ModelFormatsSection />
            <EngineConfigSection />
            <DecoderConfigSection />
            <RouterConfigSection />

            {/* Action Buttons */}
            <div className="flex items-center justify-between pt-6">
              {/* Export Button - Left side */}
              <button
                type="button"
                onClick={handleExportYaml}
                className="rounded-xl border border-slate-300 bg-white px-6 py-3 text-sm font-semibold text-slate-700 shadow-sm hover:bg-slate-50 transition-all inline-flex items-center gap-2"
              >
                <Icons.downloadFile size="sm" />
                Export YAML
              </button>

              {/* Cancel and Submit - Right side */}
              <div className="flex items-center gap-4">
                <Link
                  href={backLink}
                  className="rounded-xl border border-slate-200 bg-white px-6 py-3 text-sm font-semibold text-slate-700 shadow-sm hover:bg-slate-50 transition-all"
                >
                  Cancel
                </Link>
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="rounded-xl bg-gradient-to-br from-purple-600 to-purple-700 px-8 py-3 text-sm font-semibold text-white shadow-lg hover:shadow-xl disabled:opacity-50 disabled:cursor-not-allowed transition-all inline-flex items-center gap-2"
                >
                  {isSubmitting ? (
                    <>
                      <Spinner size="sm" />
                      {isEditMode ? 'Saving...' : 'Creating...'}
                    </>
                  ) : isEditMode ? (
                    'Save Changes'
                  ) : (
                    'Create Runtime'
                  )}
                </button>
              </div>
            </div>
          </form>
        </RuntimeFormProvider>
      </main>
    </div>
  )
}
