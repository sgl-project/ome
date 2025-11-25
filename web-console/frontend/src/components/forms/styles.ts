import { clsx } from 'clsx'

/**
 * Centralized form styling system
 * Eliminates 50+ repeated className strings across form components
 */

// Base input styles
export const inputStyles = {
  base: [
    'w-full rounded-lg border px-4 py-2.5 text-sm shadow-sm',
    'transition-colors duration-150',
    'placeholder:text-muted-foreground/60',
  ].join(' '),

  variants: {
    default: 'border-slate-300 bg-background',
    error: 'border-destructive bg-destructive/5',
    disabled: 'border-slate-200 bg-slate-50 cursor-not-allowed opacity-60',
  },

  focus: [
    'focus:border-purple-500 focus:outline-none',
    'focus:ring-2 focus:ring-purple-500/20',
  ].join(' '),

  mono: 'font-mono',
}

/**
 * Generate input className with optional states
 */
export function getInputClassName(options?: {
  error?: boolean
  disabled?: boolean
  mono?: boolean
  className?: string
}) {
  return clsx(
    inputStyles.base,
    inputStyles.focus,
    options?.error ? inputStyles.variants.error : inputStyles.variants.default,
    options?.disabled && inputStyles.variants.disabled,
    options?.mono && inputStyles.mono,
    options?.className
  )
}

// Pre-composed common variants for direct use
export const inputClassName = getInputClassName()
export const inputMonoClassName = getInputClassName({ mono: true })

// Select styles (extends input)
export const selectStyles = {
  base: clsx(inputClassName, 'appearance-none bg-no-repeat pr-10'),
  // Background image for dropdown arrow
  arrow: `bg-[url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 24 24' stroke='%236b7280'%3E%3Cpath stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M19 9l-7 7-7-7'%3E%3C/path%3E%3C/svg%3E")] bg-[length:1.25rem] bg-[right_0.75rem_center]`,
}

export function getSelectClassName(options?: {
  error?: boolean
  disabled?: boolean
  className?: string
}) {
  return clsx(
    getInputClassName({ error: options?.error, disabled: options?.disabled }),
    'appearance-none bg-no-repeat pr-10',
    selectStyles.arrow,
    options?.className
  )
}

export const selectClassName = getSelectClassName()

// Textarea styles
export const textareaStyles = {
  base: clsx(inputClassName, 'min-h-[100px] resize-y'),
}

export function getTextareaClassName(options?: {
  error?: boolean
  disabled?: boolean
  mono?: boolean
  className?: string
}) {
  return clsx(
    getInputClassName({ error: options?.error, disabled: options?.disabled, mono: options?.mono }),
    'min-h-[100px] resize-y',
    options?.className
  )
}

export const textareaClassName = getTextareaClassName()

// Label styles
export const labelStyles = {
  base: 'block text-sm font-medium text-foreground mb-1.5',
  required: 'text-destructive',
  optional: 'text-muted-foreground font-normal ml-1',
}

export function getLabelClassName(options?: { className?: string }) {
  return clsx(labelStyles.base, options?.className)
}

// Error message styles
export const errorClassName = 'mt-1.5 text-xs text-destructive'

// Help text styles
export const helpTextClassName = 'mt-1.5 text-xs text-muted-foreground'

// Section/card styles for form grouping
export const sectionStyles = {
  card: 'rounded-xl border border-border bg-card p-6 shadow-sm',
  cardCompact: 'rounded-lg border border-border bg-card p-4 shadow-sm',
  header: 'text-lg font-semibold text-foreground mb-4',
  headerSmall: 'text-base font-medium text-foreground mb-3',
  description: 'text-sm text-muted-foreground mb-4',
  divider: 'border-t border-border my-6',
}

// Button group styles
export const buttonGroupStyles = {
  horizontal: 'flex items-center gap-3',
  vertical: 'flex flex-col gap-3',
  end: 'flex items-center justify-end gap-3',
  between: 'flex items-center justify-between gap-3',
}

// Form grid layouts
export const gridStyles = {
  cols2: 'grid grid-cols-1 gap-4 sm:grid-cols-2',
  cols3: 'grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3',
  cols4: 'grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4',
}

// Accordion/collapsible section styles
export const accordionStyles = {
  trigger: clsx(
    'flex w-full items-center justify-between rounded-lg px-4 py-3',
    'bg-slate-50 hover:bg-slate-100',
    'text-sm font-medium text-foreground',
    'transition-colors duration-150'
  ),
  triggerIcon: 'h-5 w-5 text-muted-foreground transition-transform duration-200',
  triggerIconOpen: 'rotate-180',
  content: 'px-4 py-4',
}

// Array field (useFieldArray) item styles
export const arrayFieldStyles = {
  container: 'space-y-3',
  item: 'flex items-start gap-3 rounded-lg border border-border bg-card/50 p-4',
  itemCompact: 'flex items-center gap-3',
  addButton: clsx(
    'flex items-center gap-2 rounded-lg px-3 py-2',
    'border border-dashed border-slate-300',
    'text-sm text-muted-foreground',
    'hover:border-purple-500 hover:text-purple-600',
    'transition-colors duration-150'
  ),
  removeButton: clsx(
    'flex-shrink-0 rounded-lg p-2',
    'text-muted-foreground hover:text-destructive',
    'hover:bg-destructive/10',
    'transition-colors duration-150'
  ),
}

// Checkbox/radio styles
export const checkboxStyles = {
  container: 'flex items-center gap-3',
  input: clsx('h-4 w-4 rounded border-slate-300', 'text-purple-600 focus:ring-purple-500/20'),
  label: 'text-sm text-foreground',
}

// Key-value editor styles (for labels, annotations, env vars)
export const keyValueStyles = {
  row: 'flex items-center gap-2',
  keyInput: 'flex-1',
  valueInput: 'flex-1',
  equalsSign: 'text-muted-foreground text-sm',
}
