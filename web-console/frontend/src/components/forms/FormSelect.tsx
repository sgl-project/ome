'use client'

import { forwardRef, SelectHTMLAttributes } from 'react'
import { FieldWrapper, FieldWrapperProps } from './FieldWrapper'
import { getSelectClassName } from './styles'

export interface SelectOption {
  value: string
  label: string
  disabled?: boolean
}

export interface FormSelectProps
  extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'className'>,
    Omit<FieldWrapperProps, 'children'> {
  /** Array of options to render */
  options: SelectOption[]
  /** Placeholder option text */
  placeholder?: string
  /** Additional className for select element */
  selectClassName?: string
}

/**
 * Form select component with integrated label, error, and help text.
 * Wraps a native select with consistent styling from the design system.
 *
 * @example
 * ```tsx
 * <FormSelect
 *   label="Format"
 *   name="format"
 *   options={[
 *     { value: 'gguf', label: 'GGUF' },
 *     { value: 'safetensors', label: 'SafeTensors' },
 *   ]}
 *   placeholder="Select a format"
 *   error={errors.format?.message}
 *   {...register('format')}
 * />
 * ```
 */
export const FormSelect = forwardRef<HTMLSelectElement, FormSelectProps>(
  (
    {
      label,
      name,
      required,
      error,
      helpText,
      options,
      placeholder,
      className,
      selectClassName,
      ...selectProps
    },
    ref
  ) => {
    return (
      <FieldWrapper
        label={label}
        name={name}
        required={required}
        error={error}
        helpText={helpText}
        className={className}
      >
        <select
          ref={ref}
          id={name}
          name={name}
          className={getSelectClassName({ error: !!error, className: selectClassName })}
          {...selectProps}
        >
          {placeholder && (
            <option value="" disabled>
              {placeholder}
            </option>
          )}
          {options.map((option) => (
            <option key={option.value} value={option.value} disabled={option.disabled}>
              {option.label}
            </option>
          ))}
        </select>
      </FieldWrapper>
    )
  }
)

FormSelect.displayName = 'FormSelect'
