import { clsx } from 'clsx'

type StatusState = 'Ready' | 'Running' | 'Failed' | 'In_Transit' | 'Pending' | 'Creating' | 'Unknown' | 'Active' | 'Disabled' | string

interface StatusBadgeProps {
  state: StatusState | undefined | null
  className?: string
  size?: 'sm' | 'md'
}

const statusConfig: Record<string, { bg: string; text: string; dot: string }> = {
  Ready: {
    bg: 'bg-success/10 border-success/20',
    text: 'text-success',
    dot: 'bg-success',
  },
  Running: {
    bg: 'bg-success/10 border-success/20',
    text: 'text-success',
    dot: 'bg-success',
  },
  Active: {
    bg: 'bg-success/10 border-success/20',
    text: 'text-success',
    dot: 'bg-success',
  },
  Failed: {
    bg: 'bg-destructive/10 border-destructive/20',
    text: 'text-destructive',
    dot: 'bg-destructive',
  },
  In_Transit: {
    bg: 'bg-warning/10 border-warning/20',
    text: 'text-warning',
    dot: 'bg-warning animate-pulse',
  },
  Pending: {
    bg: 'bg-warning/10 border-warning/20',
    text: 'text-warning',
    dot: 'bg-warning animate-pulse',
  },
  Creating: {
    bg: 'bg-accent/10 border-accent/20',
    text: 'text-accent',
    dot: 'bg-accent animate-pulse',
  },
  Disabled: {
    bg: 'bg-muted border-border',
    text: 'text-muted-foreground',
    dot: 'bg-muted-foreground',
  },
  Unknown: {
    bg: 'bg-muted border-border',
    text: 'text-muted-foreground',
    dot: 'bg-muted-foreground',
  },
}

export function StatusBadge({ state, className = '', size = 'md' }: StatusBadgeProps) {
  const normalizedState = state || 'Unknown'
  const config = statusConfig[normalizedState] || statusConfig.Unknown

  const sizeClasses = size === 'sm'
    ? 'px-2 py-0.5 text-xs gap-1'
    : 'px-3 py-1 text-xs gap-1.5'

  const dotSize = size === 'sm' ? 'h-1 w-1' : 'h-1.5 w-1.5'

  return (
    <span
      className={clsx(
        'inline-flex items-center rounded-full border font-medium',
        config.bg,
        config.text,
        sizeClasses,
        className
      )}
    >
      <span className={clsx('rounded-full', config.dot, dotSize)} />
      {normalizedState.replace('_', ' ')}
    </span>
  )
}
