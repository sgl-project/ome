import { useState, useMemo } from 'react'

export type SortDirection = 'asc' | 'desc'

export interface SortConfig<T, K extends string> {
  field: K
  direction: SortDirection
  getValue: (item: T, field: K) => any
}

export function useSortedData<T, K extends string>(
  data: T[] | undefined,
  initialField: K,
  getValue: (item: T, field: K) => any
) {
  const [sortField, setSortField] = useState<K>(initialField)
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc')

  const handleSort = (field: K) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortDirection('asc')
    }
  }

  const sortedData = useMemo(() => {
    if (!data) return []

    const items = [...data]

    items.sort((a, b) => {
      const aValue = getValue(a, sortField)
      const bValue = getValue(b, sortField)

      // Handle string comparisons (case-insensitive)
      const aCompare = typeof aValue === 'string' ? aValue.toLowerCase() : aValue
      const bCompare = typeof bValue === 'string' ? bValue.toLowerCase() : bValue

      if (aCompare < bCompare) return sortDirection === 'asc' ? -1 : 1
      if (aCompare > bCompare) return sortDirection === 'asc' ? 1 : -1
      return 0
    })

    return items
  }, [data, sortField, sortDirection, getValue])

  return {
    sortedData,
    sortField,
    sortDirection,
    handleSort,
  }
}
