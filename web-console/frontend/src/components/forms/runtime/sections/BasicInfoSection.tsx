'use client'

import { useFieldArray } from 'react-hook-form'
import { useRuntimeFormContext } from '../RuntimeFormContext'
import { CollapsibleSection } from '../../CollapsibleSection'
import {
  inputMonoClassName,
  getInputClassName,
  labelStyles,
  gridStyles,
  helpTextClassName,
  errorClassName,
} from '../../styles'

/**
 * Basic information section for RuntimeForm.
 * Handles name, namespace, disabled toggle, model size range, and protocol versions.
 */
export function BasicInfoSection() {
  const { form, isEditMode } = useRuntimeFormContext()
  const {
    register,
    control,
    formState: { errors },
  } = form

  const {
    fields: protocolFields,
    append: appendProtocol,
    remove: removeProtocol,
  } = useFieldArray({ control, name: 'spec.protocolVersions' })

  return (
    <CollapsibleSection title="Basic Information" defaultOpen>
      <div className="space-y-6">
        {/* Name and Namespace */}
        <div className={gridStyles.cols2}>
          <div>
            <label className={labelStyles.base}>Name *</label>
            <input
              type="text"
              {...register('metadata.name')}
              disabled={isEditMode}
              className={getInputClassName({ disabled: isEditMode, mono: true })}
              placeholder="my-runtime"
            />
            {isEditMode ? (
              <p className={helpTextClassName}>Name cannot be changed after creation</p>
            ) : (
              (errors.metadata as any)?.name && (
                <p className={errorClassName}>{(errors.metadata as any).name.message as string}</p>
              )
            )}
          </div>

          <div>
            <label className={labelStyles.base}>Namespace</label>
            <input
              type="text"
              {...register('metadata.namespace')}
              disabled={isEditMode}
              className={getInputClassName({ disabled: isEditMode, mono: true })}
              placeholder={isEditMode ? 'default' : 'Leave empty for cluster-scoped'}
            />
            <p className={helpTextClassName}>
              {isEditMode
                ? 'Namespace cannot be changed'
                : 'Leave empty for ClusterServingRuntime (cluster-scoped)'}
            </p>
          </div>
        </div>

        {/* Disabled Toggle */}
        <div className="flex items-center gap-3 p-4 rounded-lg bg-slate-50 border border-slate-200">
          <input
            type="checkbox"
            id="disabled"
            {...register('spec.disabled')}
            className="h-5 w-5 rounded border-slate-300 text-purple-600 focus:ring-purple-500 focus:ring-offset-2 transition-all"
          />
          <label htmlFor="disabled" className="flex-1">
            <span className="text-sm font-medium text-foreground block">Disabled</span>
            <span className="text-xs text-muted-foreground">
              Disable this runtime from being selected
            </span>
          </label>
        </div>

        {/* Model Size Range */}
        <div>
          <h3 className="text-base font-semibold text-foreground mb-4">Model Size Range</h3>
          <div className={gridStyles.cols2}>
            <div>
              <label className={labelStyles.base}>Minimum Size</label>
              <input
                type="text"
                {...register('spec.modelSizeRange.min')}
                className={inputMonoClassName}
                placeholder="e.g., 100MB"
              />
            </div>
            <div>
              <label className={labelStyles.base}>Maximum Size</label>
              <input
                type="text"
                {...register('spec.modelSizeRange.max')}
                className={inputMonoClassName}
                placeholder="e.g., 10GB"
              />
            </div>
          </div>
        </div>

        {/* Protocol Versions */}
        <div>
          <div className="mb-4 flex items-center justify-between">
            <h3 className="text-base font-semibold text-foreground">Protocol Versions</h3>
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
                  className={inputMonoClassName}
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
              <p className="text-sm text-muted-foreground italic">No protocols defined</p>
            )}
          </div>
        </div>
      </div>
    </CollapsibleSection>
  )
}
