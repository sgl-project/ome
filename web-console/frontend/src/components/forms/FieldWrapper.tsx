'use client'

import { ReactNode } from 'react'
import { clsx } from 'clsx'
import { labelStyles, errorClassName, helpTextClassName } from './styles'

export interface FieldWrapperProps {
  /** Field label */
  label: string
  /** Field name for htmlFor */
  name?: string
  /** Whether field is required */
  required?: boolean
  /** Error message */
  error?: string
  /** Help text shown when no error */
  helpText?: string
  /** Additional class for wrapper */
  className?: string
  /** Form input element(s) */
  children: ReactNode
}

/**
 * Wrapper component for form fields providing consistent label, error, and help text styling.
 * Use this to wrap custom inputs that don't fit FormInput/FormSelect/FormTextarea patterns.
 */
export function FieldWrapper({
  label,
  name,
  required,
  error,
  helpText,
  className,
  children,
}: FieldWrapperProps) {
  return (
    <div className={className}>
      <label htmlFor={name} className={labelStyles.base}>
        {label}
        {required && <span className={clsx('ml-1', labelStyles.required)}>*</span>}
      </label>

      {children}

      {error && <p className={errorClassName}>{error}</p>}
      {helpText && !error && <p className={helpTextClassName}>{helpText}</p>}
    </div>
  )
}
