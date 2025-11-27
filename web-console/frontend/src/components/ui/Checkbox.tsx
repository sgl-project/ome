interface CheckboxProps {
  checked: boolean
  onChange: (checked: boolean) => void
  indeterminate?: boolean
  disabled?: boolean
  className?: string
  'aria-label'?: string
}

/**
 * Styled checkbox component with support for indeterminate state.
 * Used for row selection in tables.
 */
export function Checkbox({
  checked,
  onChange,
  indeterminate = false,
  disabled = false,
  className = '',
  'aria-label': ariaLabel,
}: CheckboxProps) {
  return (
    <input
      type="checkbox"
      checked={checked}
      ref={(el) => {
        if (el) {
          el.indeterminate = indeterminate
        }
      }}
      onChange={(e) => onChange(e.target.checked)}
      disabled={disabled}
      aria-label={ariaLabel}
      className={`h-4 w-4 rounded border-gray-300 text-primary focus:ring-primary focus:ring-offset-0 disabled:cursor-not-allowed disabled:opacity-50 ${className}`}
    />
  )
}
