'use client'

import { forwardRef, TextareaHTMLAttributes } from 'react'
import { FieldWrapper, FieldWrapperProps } from './FieldWrapper'
import { getTextareaClassName } from './styles'

export interface FormTextareaProps
  extends Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'className'>,
    Omit<FieldWrapperProps, 'children'> {
  /** Use monospace font */
  mono?: boolean
  /** Additional className for textarea element */
  textareaClassName?: string
}

/**
 * Form textarea component with integrated label, error, and help text.
 * Wraps a native textarea with consistent styling from the design system.
 *
 * @example
 * ```tsx
 * <FormTextarea
 *   label="Description"
 *   name="description"
 *   rows={4}
 *   error={errors.description?.message}
 *   {...register('description')}
 * />
 * ```
 */
export const FormTextarea = forwardRef<HTMLTextAreaElement, FormTextareaProps>(
  (
    {
      label,
      name,
      required,
      error,
      helpText,
      mono,
      className,
      textareaClassName,
      ...textareaProps
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
        <textarea
          ref={ref}
          id={name}
          name={name}
          className={getTextareaClassName({ error: !!error, mono, className: textareaClassName })}
          {...textareaProps}
        />
      </FieldWrapper>
    )
  }
)

FormTextarea.displayName = 'FormTextarea'
