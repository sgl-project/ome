'use client'

import { useFieldArray } from 'react-hook-form'
import { useRuntimeFormContext } from '../RuntimeFormContext'
import { CollapsibleSection } from '../../CollapsibleSection'
import {
  inputMonoClassName,
  selectClassName,
  labelStyles,
  gridStyles,
  errorClassName,
} from '../../styles'

const FORMAT_OPTIONS = [
  { value: '', label: 'Select format...' },
  { value: 'safetensors', label: 'SafeTensors' },
  { value: 'pytorch', label: 'PyTorch' },
  { value: 'onnx', label: 'ONNX' },
  { value: 'tensorflow', label: 'TensorFlow' },
  { value: 'huggingface', label: 'HuggingFace' },
]

const QUANTIZATION_OPTIONS = [
  { value: '', label: 'None' },
  { value: 'fp8', label: 'FP8' },
  { value: 'fbgemm_fp8', label: 'FBGEMM FP8' },
  { value: 'int4', label: 'INT4' },
]

/**
 * Model formats section for RuntimeForm.
 * Manages the list of supported model formats with their configurations.
 */
export function ModelFormatsSection() {
  const { form } = useRuntimeFormContext()
  const {
    register,
    control,
    formState: { errors },
  } = form

  const {
    fields: formatFields,
    append: appendFormat,
    remove: removeFormat,
  } = useFieldArray({ control, name: 'spec.supportedModelFormats' })

  return (
    <CollapsibleSection title="Supported Model Formats">
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
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
            <div key={field.id} className="rounded-xl border border-border bg-card/50 p-5">
              <div className="mb-4 flex items-center justify-between">
                <h4 className="text-sm font-semibold text-foreground">Format {index + 1}</h4>
                {formatFields.length > 1 && (
                  <button
                    type="button"
                    onClick={() => removeFormat(index)}
                    className="text-sm font-medium text-destructive hover:text-destructive/80 transition-colors"
                  >
                    Remove
                  </button>
                )}
              </div>

              <div className={gridStyles.cols2}>
                {/* Format Name */}
                <div>
                  <label className={labelStyles.base}>Format Name *</label>
                  <select
                    {...register(`spec.supportedModelFormats.${index}.name` as const)}
                    className={selectClassName}
                  >
                    {FORMAT_OPTIONS.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                  {(errors.spec as any)?.supportedModelFormats?.[index]?.name && (
                    <p className={errorClassName}>
                      {(errors.spec as any).supportedModelFormats[index]?.name?.message as string}
                    </p>
                  )}
                </div>

                {/* Version */}
                <div>
                  <label className={labelStyles.base}>Version</label>
                  <input
                    type="text"
                    {...register(`spec.supportedModelFormats.${index}.version` as const)}
                    className={inputMonoClassName}
                    placeholder="1.0"
                  />
                </div>

                {/* Model Type */}
                <div>
                  <label className={labelStyles.base}>Model Type</label>
                  <input
                    type="text"
                    {...register(`spec.supportedModelFormats.${index}.modelType` as const)}
                    className={inputMonoClassName}
                    placeholder="text-generation"
                  />
                </div>

                {/* Model Architecture */}
                <div>
                  <label className={labelStyles.base}>Model Architecture</label>
                  <input
                    type="text"
                    {...register(`spec.supportedModelFormats.${index}.modelArchitecture` as const)}
                    className={inputMonoClassName}
                    placeholder="LlamaForCausalLM"
                  />
                </div>

                {/* Quantization */}
                <div>
                  <label className={labelStyles.base}>Quantization</label>
                  <select
                    {...register(`spec.supportedModelFormats.${index}.quantization` as const)}
                    className={selectClassName}
                  >
                    {QUANTIZATION_OPTIONS.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </div>

                {/* Priority */}
                <div>
                  <label className={labelStyles.base}>Priority</label>
                  <input
                    type="number"
                    {...register(`spec.supportedModelFormats.${index}.priority` as const, {
                      valueAsNumber: true,
                    })}
                    className={inputMonoClassName}
                    placeholder="0"
                  />
                </div>

                {/* Auto Select */}
                <div className="flex items-center gap-3 p-3 rounded-lg bg-slate-50">
                  <input
                    type="checkbox"
                    {...register(`spec.supportedModelFormats.${index}.autoSelect` as const)}
                    className="h-4 w-4 rounded border-slate-300 text-purple-600 focus:ring-purple-500"
                  />
                  <label className="text-sm text-foreground font-medium">Auto Select</label>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </CollapsibleSection>
  )
}
