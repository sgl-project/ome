'use client'

import { ReactNode, useState } from 'react'
import { clsx } from 'clsx'

export interface CollapsibleSectionProps {
  /** Section title */
  title: string
  /** Optional description shown below title */
  description?: string
  /** Whether the section is open by default */
  defaultOpen?: boolean
  /** Children to render inside the collapsible content */
  children: ReactNode
  /** Additional className for the container */
  className?: string
  /** Badge to show next to title (e.g., count) */
  badge?: ReactNode
}

/**
 * Collapsible section component for organizing form content.
 * Used in RuntimeForm and other complex forms to group related fields.
 *
 * @example
 * ```tsx
 * <CollapsibleSection
 *   title="Engine Configuration"
 *   description="Configure the inference engine settings"
 *   defaultOpen={false}
 * >
 *   <EngineConfigFields />
 * </CollapsibleSection>
 * ```
 */
export function CollapsibleSection({
  title,
  description,
  defaultOpen = false,
  children,
  className,
  badge,
}: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen)

  return (
    <div className={clsx('rounded-xl border border-border bg-card overflow-hidden', className)}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="w-full px-6 py-4 flex items-center justify-between hover:bg-muted/50 transition-colors"
      >
        <div className="text-left">
          <div className="flex items-center gap-2">
            <h3 className="text-base font-semibold text-foreground">{title}</h3>
            {badge}
          </div>
          {description && <p className="mt-0.5 text-sm text-muted-foreground">{description}</p>}
        </div>
        <svg
          className={clsx(
            'w-5 h-5 text-muted-foreground transition-transform duration-200',
            isOpen && 'rotate-180'
          )}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {isOpen && <div className="px-6 pb-6 pt-2 border-t border-border">{children}</div>}
    </div>
  )
}
