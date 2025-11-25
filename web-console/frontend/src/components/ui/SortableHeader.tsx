import { ReactNode } from 'react'
import { SortDirection } from '@/lib/hooks/useSortedData'

interface SortableHeaderProps<K extends string> {
  field: K
  currentField: K
  direction: SortDirection
  onSort: (field: K) => void
  children: ReactNode
}

export function SortableHeader<K extends string>({
  field,
  currentField,
  direction,
  onSort,
  children,
}: SortableHeaderProps<K>) {
  const isActive = currentField === field

  return (
    <th
      className="px-6 py-4 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground cursor-pointer hover:text-primary transition-colors select-none"
      onClick={() => onSort(field)}
    >
      <div className="flex items-center gap-2">
        {children}
        <div className="flex flex-col">
          <svg
            className={`w-3 h-3 -mb-1 transition-colors ${
              isActive && direction === 'asc' ? 'text-primary' : 'text-muted-foreground/40'
            }`}
            fill="currentColor"
            viewBox="0 0 12 12"
          >
            <path d="M6 2l4 4H2z" />
          </svg>
          <svg
            className={`w-3 h-3 transition-colors ${
              isActive && direction === 'desc' ? 'text-primary' : 'text-muted-foreground/40'
            }`}
            fill="currentColor"
            viewBox="0 0 12 12"
          >
            <path d="M6 10L2 6h8z" />
          </svg>
        </div>
      </div>
    </th>
  )
}
