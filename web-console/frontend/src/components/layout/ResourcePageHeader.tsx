'use client'

import { ReactNode } from 'react'

interface ResourcePageHeaderProps {
  /** Page title */
  title: string
  /** Description shown below title */
  description: string
  /** Action buttons (import, create, etc.) */
  actions?: ReactNode
}

/**
 * Header component for resource list pages.
 * Provides consistent styling for models, services, runtimes pages.
 */
export function ResourcePageHeader({ title, description, actions }: ResourcePageHeaderProps) {
  return (
    <header className="border-b border-border bg-card/50 backdrop-blur-sm">
      <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <div className="flex items-start justify-between gap-8">
          <div>
            <h1 className="text-3xl font-semibold tracking-tight text-foreground">{title}</h1>
            <p className="mt-1 text-sm text-muted-foreground">{description}</p>
          </div>
          {actions && <div className="flex gap-3">{actions}</div>}
        </div>
      </div>
    </header>
  )
}
