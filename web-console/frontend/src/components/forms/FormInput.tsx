'use client'

import { forwardRef, InputHTMLAttributes } from 'react'
import { FieldWrapper, FieldWrapperProps } from './FieldWrapper'
import { getInputClassName } from './styles'

export interface FormInputProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, 'className'>,
    Omit<FieldWrapperProps, 'children'> {
  /** Use monospace font */
  mono?: boolean
  /** Additional className for input element */
  inputClassName?: string
}

/**
 * Form input component with integrated label, error, and help text.
 * Wraps a native input with consistent styling from the design system.
 *
 * @example
 * ```tsx
 * <FormInput
 *   label="Name"
 *   name="name"
 *   required
 *   error={errors.name?.message}
 *   {...register('name')}
 * />
 * ```
 */
export const FormInput = forwardRef<HTMLInputElement, FormInputProps>(
  (
    { label, name, required, error, helpText, mono, className, inputClassName, ...inputProps },
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
        <input
          ref={ref}
          id={name}
          name={name}
          className={getInputClassName({ error: !!error, mono, className: inputClassName })}
          {...inputProps}
        />
      </FieldWrapper>
    )
  }
)

FormInput.displayName = 'FormInput'
