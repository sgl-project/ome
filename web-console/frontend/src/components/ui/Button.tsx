import Link from 'next/link'
import { ReactNode, ButtonHTMLAttributes } from 'react'
import { clsx } from 'clsx'
import { Spinner } from './Spinner'
import { Icons } from './Icons'

type ButtonVariant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'destructive'
type ButtonSize = 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  href?: string
  icon?: ReactNode
  iconPosition?: 'left' | 'right'
  loading?: boolean
  children: ReactNode
}

const variantStyles: Record<ButtonVariant, string> = {
  primary:
    'bg-gradient-to-r from-primary to-accent text-white hover:shadow-lg hover:shadow-primary/25 border-transparent',
  secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/80 border-border',
  outline: 'bg-transparent border-primary text-primary hover:bg-primary/5',
  ghost: 'bg-transparent border-transparent text-foreground hover:bg-muted',
  destructive:
    'bg-destructive text-destructive-foreground hover:bg-destructive/90 border-transparent',
}

const sizeStyles: Record<ButtonSize, string> = {
  sm: 'px-3 py-1.5 text-xs gap-1.5',
  md: 'px-4 py-2.5 text-sm gap-2',
  lg: 'px-6 py-3 text-base gap-2',
}

export function Button({
  variant = 'primary',
  size = 'md',
  href,
  icon,
  iconPosition = 'left',
  loading = false,
  disabled,
  className = '',
  children,
  ...props
}: ButtonProps) {
  const baseStyles = clsx(
    'relative inline-flex items-center justify-center font-medium rounded-lg border transition-all duration-200',
    'focus:outline-none focus:ring-2 focus:ring-primary/50 focus:ring-offset-2',
    'disabled:opacity-50 disabled:cursor-not-allowed disabled:pointer-events-none',
    variantStyles[variant],
    sizeStyles[size],
    className
  )

  const content = (
    <>
      {loading && <Spinner size="sm" />}
      {icon && iconPosition === 'left' && !loading && <span className="flex-shrink-0">{icon}</span>}
      <span>{children}</span>
      {icon && iconPosition === 'right' && !loading && (
        <span className="flex-shrink-0 transition-transform group-hover:translate-x-0.5">
          {icon}
        </span>
      )}
    </>
  )

  if (href) {
    return (
      <Link href={href} className={clsx(baseStyles, 'group')}>
        {content}
      </Link>
    )
  }

  return (
    <button className={clsx(baseStyles, 'group')} disabled={disabled || loading} {...props}>
      {content}
    </button>
  )
}

/**
 * Pre-configured button icons using the shared Icons component.
 * These provide consistent sizing (sm) for use within buttons.
 */
export const ButtonIcons = {
  plus: <Icons.plus size="sm" />,
  import: <Icons.upload size="sm" />,
  arrowRight: <Icons.arrowRight size="sm" />,
  trash: <Icons.trash size="sm" />,
  edit: <Icons.pencil size="sm" />,
  chevronRight: <Icons.chevronRight size="sm" />,
  check: <Icons.check size="sm" />,
}
