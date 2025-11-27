'use client'

import { ReactNode } from 'react'
import { ResourceFilters } from './ResourceFilters'

interface ResourceTableProps {
  /** Table title */
  title: string
  /** Table content (thead + tbody) */
  children: ReactNode
  /** Header actions (e.g., bulk action dropdown) */
  headerActions?: ReactNode
  /** Namespace filter props (optional) */
  filterProps?: {
    namespaces?: string[]
    selectedNamespace: string
    onNamespaceChange: (namespace: string) => void
    scopeLabel?: string
    defaultOptionText?: string
    namespaceFormat?: (ns: string) => string
  }
}

/**
 * Container for resource tables with optional namespace filter and header actions.
 * Provides consistent card styling and header layout.
 */
export function ResourceTable({
  title,
  children,
  headerActions,
  filterProps,
}: ResourceTableProps) {
  return (
    <div className="overflow-hidden rounded-xl border border-border bg-card shadow-sm">
      <div className="flex items-center justify-between border-b border-border px-6 py-4 bg-muted/30">
        <div className="flex items-center gap-4">
          <h3 className="text-base font-semibold tracking-tight">{title}</h3>
          {headerActions}
        </div>
        {filterProps && <ResourceFilters {...filterProps} />}
      </div>
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-border">{children}</table>
      </div>
    </div>
  )
}

interface EmptyTableStateProps {
  /** Number of columns for colspan */
  colSpan: number
  /** Icon element */
  icon: ReactNode
  /** Message to display */
  message: string
  /** Action element (e.g., button) */
  action?: ReactNode
}

/**
 * Empty state for tables when no data is available.
 */
export function EmptyTableState({ colSpan, icon, message, action }: EmptyTableStateProps) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-6 py-12 text-center">
        <div className="flex flex-col items-center gap-3">
          <div className="h-12 w-12 text-muted-foreground/40">{icon}</div>
          <p className="text-sm text-muted-foreground">{message}</p>
          {action}
        </div>
      </td>
    </tr>
  )
}
