'use client'

import { selectClassName } from '@/components/forms/styles'

interface ResourceFiltersProps {
  /** List of available namespaces */
  namespaces?: string[]
  /** Currently selected namespace */
  selectedNamespace: string
  /** Callback when namespace changes */
  onNamespaceChange: (namespace: string) => void
  /** Label for the scope selector (default: "Scope:") */
  scopeLabel?: string
  /** Default option text (default: "Cluster-scoped") */
  defaultOptionText?: string
  /** Format for namespace option (default: "Namespace: {ns}") */
  namespaceFormat?: (ns: string) => string
}

/**
 * Filter controls for resource list pages.
 * Provides namespace selection with consistent styling.
 */
export function ResourceFilters({
  namespaces,
  selectedNamespace,
  onNamespaceChange,
  scopeLabel = 'Scope:',
  defaultOptionText = 'Cluster-scoped',
  namespaceFormat = (ns) => `Namespace: ${ns}`,
}: ResourceFiltersProps) {
  return (
    <div className="flex items-center gap-3">
      <label htmlFor="namespace" className="text-sm font-medium text-muted-foreground">
        {scopeLabel}
      </label>
      <select
        id="namespace"
        value={selectedNamespace}
        onChange={(e) => onNamespaceChange(e.target.value)}
        className={selectClassName}
        style={{ width: 'auto', minWidth: '180px' }}
      >
        <option value="">{defaultOptionText}</option>
        {namespaces?.map((ns) => (
          <option key={ns} value={ns}>
            {namespaceFormat(ns)}
          </option>
        ))}
      </select>
    </div>
  )
}
