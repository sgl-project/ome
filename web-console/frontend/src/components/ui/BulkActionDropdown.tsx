'use client'

import { useState, useRef, useEffect } from 'react'
import { Icons } from './Icons'

export interface BulkAction {
  id: string
  label: string
  icon: React.ReactNode
  onClick: () => void
  disabled?: boolean
  variant?: 'default' | 'destructive'
}

interface BulkActionDropdownProps {
  actions: BulkAction[]
  selectedCount: number
  label?: string
}

/**
 * Dropdown menu for bulk actions on selected items.
 * Shows a count of selected items and available actions.
 */
export function BulkActionDropdown({
  actions,
  selectedCount,
  label = 'Actions',
}: BulkActionDropdownProps) {
  const [isOpen, setIsOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // Close on escape key
  useEffect(() => {
    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setIsOpen(false)
      }
    }

    document.addEventListener('keydown', handleEscape)
    return () => document.removeEventListener('keydown', handleEscape)
  }, [])

  const hasSelection = selectedCount > 0

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={`inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-sm font-medium transition-colors ${
          hasSelection
            ? 'border-primary bg-primary/5 text-primary hover:bg-primary/10'
            : 'border-border bg-card text-foreground hover:bg-muted'
        }`}
      >
        {label}
        {hasSelection && (
          <span className="inline-flex items-center justify-center rounded-full bg-primary px-2 py-0.5 text-xs font-semibold text-primary-foreground">
            {selectedCount}
          </span>
        )}
        <Icons.chevronDown
          size="xs"
          className={`transition-transform ${isOpen ? 'rotate-180' : ''}`}
        />
      </button>

      {isOpen && (
        <div className="absolute right-0 z-50 mt-2 w-48 origin-top-right rounded-lg border border-border bg-card shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none">
          <div className="py-1">
            {actions.map((action) => (
              <button
                key={action.id}
                type="button"
                onClick={() => {
                  if (!action.disabled) {
                    action.onClick()
                    setIsOpen(false)
                  }
                }}
                disabled={action.disabled}
                className={`flex w-full items-center gap-2 px-4 py-2 text-sm transition-colors ${
                  action.disabled
                    ? 'cursor-not-allowed text-muted-foreground/50'
                    : action.variant === 'destructive'
                      ? 'text-destructive hover:bg-destructive/10'
                      : 'text-foreground hover:bg-muted'
                }`}
              >
                {action.icon}
                {action.label}
                {action.disabled && selectedCount > 1 && action.id === 'edit' && (
                  <span className="ml-auto text-xs text-muted-foreground">(single only)</span>
                )}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
