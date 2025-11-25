import Link from 'next/link'
import { ReactNode } from 'react'
import { clsx } from 'clsx'

type ColorVariant = 'primary' | 'success' | 'warning' | 'destructive' | 'accent' | 'muted'

interface StatCardProps {
  label: string
  value: number | string
  icon: ReactNode
  href?: string
  variant?: ColorVariant
  delay?: number
}

const variantStyles: Record<ColorVariant, { bg: string; text: string; iconBg: string }> = {
  primary: {
    bg: 'from-primary/5 to-transparent',
    text: 'text-primary',
    iconBg: 'bg-primary/10 text-primary',
  },
  success: {
    bg: 'from-success/5 to-transparent',
    text: 'text-success',
    iconBg: 'bg-success/10 text-success',
  },
  warning: {
    bg: 'from-warning/5 to-transparent',
    text: 'text-warning',
    iconBg: 'bg-warning/10 text-warning',
  },
  destructive: {
    bg: 'from-destructive/5 to-transparent',
    text: 'text-destructive',
    iconBg: 'bg-destructive/10 text-destructive',
  },
  accent: {
    bg: 'from-accent/5 to-transparent',
    text: 'text-accent',
    iconBg: 'bg-accent/10 text-accent',
  },
  muted: {
    bg: 'from-muted-foreground/5 to-transparent',
    text: 'text-muted-foreground',
    iconBg: 'bg-muted text-muted-foreground',
  },
}

const delayClasses = ['', 'animate-in-delay-1', 'animate-in-delay-2', 'animate-in-delay-3']

export function StatCard({
  label,
  value,
  icon,
  href,
  variant = 'primary',
  delay = 0,
}: StatCardProps) {
  const styles = variantStyles[variant]
  const delayClass = delayClasses[delay] || ''

  const content = (
    <div
      className={clsx(
        'group relative overflow-hidden rounded-xl border border-border bg-card shadow-sm transition-all duration-300 hover:shadow-md hover:border-border/80',
        'animate-in',
        delayClass
      )}
    >
      <div
        className={clsx(
          'absolute inset-0 bg-gradient-to-br opacity-0 group-hover:opacity-100 transition-opacity duration-300',
          styles.bg
        )}
      />
      <div className="relative p-5">
        <div className="flex items-center justify-between">
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-muted-foreground truncate">{label}</p>
            <p
              className={clsx(
                'mt-2 text-3xl font-semibold tracking-tight',
                typeof value === 'number' && value > 0 ? styles.text : ''
              )}
            >
              {value}
            </p>
          </div>
          <div
            className={clsx(
              'flex h-12 w-12 items-center justify-center rounded-xl transition-transform duration-300 group-hover:scale-110',
              styles.iconBg
            )}
          >
            {icon}
          </div>
        </div>
      </div>
    </div>
  )

  if (href) {
    return <Link href={href}>{content}</Link>
  }

  return content
}
