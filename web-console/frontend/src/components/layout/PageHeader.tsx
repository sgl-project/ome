import Link from 'next/link'
import { ReactNode } from 'react'

interface PageHeaderProps {
  title: string
  description?: string
  actions?: ReactNode
  backLink?: {
    href: string
    label: string
  }
}

export function PageHeader({ title, description, actions, backLink }: PageHeaderProps) {
  return (
    <header className="relative border-b border-border/50 bg-card/50 backdrop-blur-sm animate-in">
      <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {backLink && (
          <Link
            href={backLink.href}
            className="group inline-flex items-center gap-2 text-sm font-medium text-muted-foreground hover:text-primary transition-colors mb-4"
          >
            <span className="transition-transform group-hover:-translate-x-1">←</span>
            <span>{backLink.label}</span>
          </Link>
        )}
        <div className="flex items-start justify-between gap-8">
          <div>
            <h1 className="text-4xl font-bold tracking-tight bg-gradient-to-r from-primary to-primary/60 bg-clip-text text-transparent">
              {title}
            </h1>
            {description && (
              <p className="mt-2 text-muted-foreground max-w-2xl">
                {description}
              </p>
            )}
          </div>
          {actions && (
            <div className="flex gap-3 flex-shrink-0">
              {actions}
            </div>
          )}
        </div>
      </div>
    </header>
  )
}
