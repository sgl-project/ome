import { InputHTMLAttributes, TextareaHTMLAttributes, SelectHTMLAttributes, ReactNode } from 'react'
import { clsx } from 'clsx'

// Shared input styling classes
export const inputStyles = {
  base: 'w-full rounded-lg border bg-background px-3 py-2 text-sm shadow-sm transition-colors',
  focus: 'focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20',
  error: 'border-destructive focus:border-destructive focus:ring-destructive/20',
  default: 'border-border',
  disabled: 'opacity-50 cursor-not-allowed bg-muted',
}

export function getInputClassName(hasError = false, disabled = false, className = '') {
  return clsx(
    inputStyles.base,
    inputStyles.focus,
    hasError ? inputStyles.error : inputStyles.default,
    disabled && inputStyles.disabled,
    className
  )
}

interface FormFieldBaseProps {
  label: string
  required?: boolean
  error?: string
  helpText?: string
  className?: string
}

interface FormInputProps
  extends FormFieldBaseProps, Omit<InputHTMLAttributes<HTMLInputElement>, 'className'> {
  inputClassName?: string
}

export function FormInput({
  label,
  required = false,
  error,
  helpText,
  className = '',
  inputClassName = '',
  disabled,
  ...props
}: FormInputProps) {
  return (
    <div className={className}>
      <label className="block text-sm font-medium text-foreground mb-1.5">
        {label} {required && <span className="text-destructive">*</span>}
      </label>
      <input
        {...props}
        disabled={disabled}
        className={getInputClassName(!!error, disabled, inputClassName)}
      />
      {error && <p className="mt-1 text-sm text-destructive">{error}</p>}
      {helpText && !error && <p className="mt-1 text-sm text-muted-foreground">{helpText}</p>}
    </div>
  )
}

interface FormTextareaProps
  extends FormFieldBaseProps, Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'className'> {
  inputClassName?: string
}

export function FormTextarea({
  label,
  required = false,
  error,
  helpText,
  className = '',
  inputClassName = '',
  disabled,
  ...props
}: FormTextareaProps) {
  return (
    <div className={className}>
      <label className="block text-sm font-medium text-foreground mb-1.5">
        {label} {required && <span className="text-destructive">*</span>}
      </label>
      <textarea
        {...props}
        disabled={disabled}
        className={clsx(
          getInputClassName(!!error, disabled),
          'min-h-[80px] resize-y',
          inputClassName
        )}
      />
      {error && <p className="mt-1 text-sm text-destructive">{error}</p>}
      {helpText && !error && <p className="mt-1 text-sm text-muted-foreground">{helpText}</p>}
    </div>
  )
}

interface FormSelectProps
  extends FormFieldBaseProps, Omit<SelectHTMLAttributes<HTMLSelectElement>, 'className'> {
  inputClassName?: string
  children: ReactNode
}

export function FormSelect({
  label,
  required = false,
  error,
  helpText,
  className = '',
  inputClassName = '',
  disabled,
  children,
  ...props
}: FormSelectProps) {
  return (
    <div className={className}>
      <label className="block text-sm font-medium text-foreground mb-1.5">
        {label} {required && <span className="text-destructive">*</span>}
      </label>
      <select
        {...props}
        disabled={disabled}
        className={getInputClassName(!!error, disabled, inputClassName)}
      >
        {children}
      </select>
      {error && <p className="mt-1 text-sm text-destructive">{error}</p>}
      {helpText && !error && <p className="mt-1 text-sm text-muted-foreground">{helpText}</p>}
    </div>
  )
}

interface FormCheckboxProps
  extends FormFieldBaseProps, Omit<InputHTMLAttributes<HTMLInputElement>, 'className' | 'type'> {
  inputClassName?: string
}

export function FormCheckbox({
  label,
  error,
  helpText,
  className = '',
  inputClassName = '',
  disabled,
  ...props
}: FormCheckboxProps) {
  return (
    <div className={className}>
      <label className="flex items-center gap-2 cursor-pointer">
        <input
          type="checkbox"
          {...props}
          disabled={disabled}
          className={clsx(
            'h-4 w-4 rounded border-border text-primary focus:ring-2 focus:ring-primary/20',
            disabled && 'opacity-50 cursor-not-allowed',
            inputClassName
          )}
        />
        <span className="text-sm font-medium text-foreground">{label}</span>
      </label>
      {error && <p className="mt-1 text-sm text-destructive">{error}</p>}
      {helpText && !error && <p className="mt-1 text-sm text-muted-foreground">{helpText}</p>}
    </div>
  )
}

// Convenience component for form sections
interface FormSectionProps {
  title: string
  description?: string
  children: ReactNode
  className?: string
}

export function FormSection({ title, description, children, className = '' }: FormSectionProps) {
  return (
    <div className={clsx('space-y-4', className)}>
      <div>
        <h3 className="text-sm font-semibold text-foreground">{title}</h3>
        {description && <p className="text-sm text-muted-foreground mt-0.5">{description}</p>}
      </div>
      {children}
    </div>
  )
}

// Convenience component for form rows (side by side fields)
interface FormRowProps {
  children: ReactNode
  className?: string
}

export function FormRow({ children, className = '' }: FormRowProps) {
  return <div className={clsx('grid grid-cols-1 gap-4 sm:grid-cols-2', className)}>{children}</div>
}
