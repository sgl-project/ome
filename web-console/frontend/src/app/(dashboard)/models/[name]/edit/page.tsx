'use client'

import { useModel, useUpdateModel } from '@/lib/hooks/useModels'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { useForm } from 'react-hook-form'
import { useState, useEffect } from 'react'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { CollapsibleSection } from '@/components/forms/CollapsibleSection'
import {
  sectionStyles,
  gridStyles,
  inputClassName,
  selectClassName,
  labelStyles,
  helpTextClassName,
  arrayFieldStyles,
} from '@/components/forms/styles'
import type { ClusterBaseModel } from '@/lib/types/model'
import { MODEL_FORMAT_OPTIONS, MODEL_FRAMEWORK_OPTIONS } from '@/lib/constants/model-options'

interface EditModelFormData {
  spec: {
    vendor?: string
    modelParameterSize?: string
    modelFormat?: { name: string; version?: string }
    modelFramework?: { name?: string; version?: string }
  }
}

interface KeyValuePair {
  key: string
  value: string
}

function KeyValueEditor({
  items,
  onChange,
  addLabel,
}: {
  items: KeyValuePair[]
  onChange: (items: KeyValuePair[]) => void
  addLabel: string
}) {
  return (
    <div className="space-y-3">
      {items.map((item, index) => (
        <div key={index} className="flex items-center gap-2">
          <input
            type="text"
            value={item.key}
            onChange={(e) => {
              const updated = [...items]
              updated[index].key = e.target.value
              onChange(updated)
            }}
            className={inputClassName}
            placeholder="Key"
          />
          <input
            type="text"
            value={item.value}
            onChange={(e) => {
              const updated = [...items]
              updated[index].value = e.target.value
              onChange(updated)
            }}
            className={inputClassName}
            placeholder="Value"
          />
          <button
            type="button"
            onClick={() => onChange(items.filter((_, i) => i !== index))}
            className={arrayFieldStyles.removeButton}
          >
            Remove
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() => onChange([...items, { key: '', value: '' }])}
        className={arrayFieldStyles.addButton}
      >
        + {addLabel}
      </button>
    </div>
  )
}

export default function EditModelPage() {
  const params = useParams()
  const router = useRouter()
  const name = params.name as string
  const { data: model, isLoading, error } = useModel(name)
  const updateModel = useUpdateModel()

  const [submitError, setSubmitError] = useState<string | null>(null)
  const [labels, setLabels] = useState<KeyValuePair[]>([])
  const [annotations, setAnnotations] = useState<KeyValuePair[]>([])

  const {
    register,
    handleSubmit,
    reset,
    formState: { isSubmitting },
  } = useForm<EditModelFormData>()

  useEffect(() => {
    if (model) {
      reset({
        spec: {
          vendor: model.spec.vendor || '',
          modelParameterSize: model.spec.modelParameterSize || '',
          modelFormat: {
            name: model.spec.modelFormat?.name || '',
            version: model.spec.modelFormat?.version || '',
          },
          modelFramework: {
            name: model.spec.modelFramework?.name || '',
            version: model.spec.modelFramework?.version || '',
          },
        },
      })

      if (model.metadata.labels) {
        setLabels(Object.entries(model.metadata.labels).map(([key, value]) => ({ key, value })))
      }
      if (model.metadata.annotations) {
        setAnnotations(
          Object.entries(model.metadata.annotations).map(([key, value]) => ({ key, value }))
        )
      }
    }
  }, [model, reset])

  const onSubmit = async (data: EditModelFormData) => {
    if (!model) return

    try {
      setSubmitError(null)

      const labelsRecord: Record<string, string> = {}
      labels.forEach(({ key, value }) => {
        if (key && value) labelsRecord[key] = value
      })

      const annotationsRecord: Record<string, string> = {}
      annotations.forEach(({ key, value }) => {
        if (key && value) annotationsRecord[key] = value
      })

      const updatedModel: Partial<ClusterBaseModel> = {
        apiVersion: model.apiVersion,
        kind: model.kind,
        metadata: {
          ...model.metadata,
          labels: Object.keys(labelsRecord).length > 0 ? labelsRecord : undefined,
          annotations: Object.keys(annotationsRecord).length > 0 ? annotationsRecord : undefined,
        },
        spec: {
          ...model.spec,
          vendor: data.spec.vendor || undefined,
          modelParameterSize: data.spec.modelParameterSize || undefined,
          modelFormat: data.spec.modelFormat?.name
            ? {
                name: data.spec.modelFormat.name,
                version: data.spec.modelFormat.version || undefined,
              }
            : model.spec.modelFormat,
          modelFramework: data.spec.modelFramework?.name
            ? {
                name: data.spec.modelFramework.name,
                version: data.spec.modelFramework.version || undefined,
              }
            : model.spec.modelFramework,
        },
      }

      await updateModel.mutateAsync({ name, model: updatedModel })
      router.push(`/models/${name}`)
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Failed to update model')
    }
  }

  if (isLoading) {
    return <LoadingState message="Loading model..." />
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
    <div className="min-h-screen bg-background">
      <header className="border-b border-border bg-card shadow-sm">
        <div className="mx-auto max-w-4xl px-4 py-6 sm:px-6 lg:px-8">
          <Link
            href={`/models/${name}`}
            className="text-sm text-primary hover:text-primary/80 mb-2 inline-block"
          >
            ← Back to Model Details
          </Link>
          <h1 className="text-3xl font-bold text-foreground">Edit Model</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Editing <span className="font-mono text-primary">{model.metadata.name}</span>
          </p>
        </div>
      </header>

      <main className="mx-auto max-w-4xl px-4 py-6 sm:px-6 lg:px-8">
        {submitError && (
          <div className="mb-6 rounded-lg bg-destructive/10 border border-destructive/20 p-4">
            <p className="text-sm text-destructive">{submitError}</p>
          </div>
        )}

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
          {/* Basic Information */}
          <div className={sectionStyles.card}>
            <h2 className={sectionStyles.header}>Basic Information</h2>
            <div className={gridStyles.cols2}>
              <div>
                <label className={labelStyles.base}>Name</label>
                <input
                  type="text"
                  value={model.metadata.name}
                  disabled
                  className={`${inputClassName} bg-muted cursor-not-allowed`}
                />
                <p className={helpTextClassName}>Name cannot be changed</p>
              </div>

              <div>
                <label htmlFor="vendor" className={labelStyles.base}>
                  Vendor
                </label>
                <input
                  type="text"
                  id="vendor"
                  {...register('spec.vendor')}
                  className={inputClassName}
                  placeholder="e.g., meta, openai, anthropic"
                />
              </div>

              <div>
                <label htmlFor="modelParameterSize" className={labelStyles.base}>
                  Model Parameter Size
                </label>
                <input
                  type="text"
                  id="modelParameterSize"
                  {...register('spec.modelParameterSize')}
                  className={inputClassName}
                  placeholder="e.g., 7B, 13B, 70B"
                />
              </div>
            </div>
          </div>

          {/* Storage (Read-only) */}
          <div className={sectionStyles.card}>
            <h2 className={sectionStyles.header}>
              Storage <span className="text-sm font-normal text-muted-foreground">(Read-only)</span>
            </h2>
            <div className={gridStyles.cols2}>
              <div>
                <label className={labelStyles.base}>Storage URI</label>
                <input
                  type="text"
                  value={model.spec.storage?.storageUri || '-'}
                  disabled
                  className={`${inputClassName} bg-muted cursor-not-allowed font-mono text-sm`}
                />
              </div>
              <div>
                <label className={labelStyles.base}>Storage Path</label>
                <input
                  type="text"
                  value={model.spec.storage?.path || '-'}
                  disabled
                  className={`${inputClassName} bg-muted cursor-not-allowed font-mono text-sm`}
                />
              </div>
            </div>
            <p className={helpTextClassName}>
              Storage configuration cannot be changed. Delete and recreate to change storage.
            </p>
          </div>

          {/* Model Format */}
          <div className={sectionStyles.card}>
            <h2 className={sectionStyles.header}>Model Format</h2>
            <div className={gridStyles.cols2}>
              <div>
                <label htmlFor="formatName" className={labelStyles.base}>
                  Format Name
                </label>
                <select
                  id="formatName"
                  {...register('spec.modelFormat.name')}
                  className={selectClassName}
                >
                  {MODEL_FORMAT_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label htmlFor="formatVersion" className={labelStyles.base}>
                  Format Version
                </label>
                <input
                  type="text"
                  id="formatVersion"
                  {...register('spec.modelFormat.version')}
                  className={inputClassName}
                  placeholder="e.g., 1.0"
                />
              </div>
            </div>
          </div>

          {/* Model Framework */}
          <div className={sectionStyles.card}>
            <h2 className={sectionStyles.header}>Model Framework</h2>
            <div className={gridStyles.cols2}>
              <div>
                <label htmlFor="frameworkName" className={labelStyles.base}>
                  Framework Name
                </label>
                <select
                  id="frameworkName"
                  {...register('spec.modelFramework.name')}
                  className={selectClassName}
                >
                  {MODEL_FRAMEWORK_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label htmlFor="frameworkVersion" className={labelStyles.base}>
                  Framework Version
                </label>
                <input
                  type="text"
                  id="frameworkVersion"
                  {...register('spec.modelFramework.version')}
                  className={inputClassName}
                  placeholder="e.g., 2.0"
                />
              </div>
            </div>
          </div>

          {/* Labels */}
          <CollapsibleSection
            title="Labels"
            description="Key-value pairs for organizing resources"
            defaultOpen={labels.length > 0}
            badge={
              labels.length > 0 ? (
                <span className="text-xs bg-muted px-2 py-0.5 rounded-full">{labels.length}</span>
              ) : undefined
            }
          >
            <KeyValueEditor items={labels} onChange={setLabels} addLabel="Add Label" />
          </CollapsibleSection>

          {/* Annotations */}
          <CollapsibleSection
            title="Annotations"
            description="Key-value pairs for storing metadata"
            defaultOpen={annotations.length > 0}
            badge={
              annotations.length > 0 ? (
                <span className="text-xs bg-muted px-2 py-0.5 rounded-full">
                  {annotations.length}
                </span>
              ) : undefined
            }
          >
            <KeyValueEditor
              items={annotations}
              onChange={setAnnotations}
              addLabel="Add Annotation"
            />
          </CollapsibleSection>

          {/* Submit */}
          <div className="flex justify-end gap-4 pt-4">
            <Link
              href={`/models/${name}`}
              className="rounded-lg border border-border bg-card px-6 py-2.5 text-sm font-medium text-foreground hover:bg-muted transition-colors"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={isSubmitting}
              className="rounded-lg bg-primary px-6 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-colors"
            >
              {isSubmitting ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </form>
      </main>
    </div>
  )
}
