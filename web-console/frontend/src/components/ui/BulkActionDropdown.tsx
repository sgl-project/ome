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

export function BulkActionDropdown({
  actions,
  selectedCount,
  label = 'Actions',
}: BulkActionDropdownProps) {
  const [isOpen, setIsOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!isOpen) return

    const handleClose = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent && e.key === 'Escape') {
        setIsOpen(false)
      } else if (
        e instanceof MouseEvent &&
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClose)
    document.addEventListener('keydown', handleClose)
    return () => {
      document.removeEventListener('mousedown', handleClose)
      document.removeEventListener('keydown', handleClose)
    }
  }, [isOpen])

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
        <div className="absolute right-0 z-50 mt-2 w-48 origin-top-right rounded-lg border border-border bg-card shadow-lg">
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
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
