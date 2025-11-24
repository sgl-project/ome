type ModelState = 'Ready' | 'Failed' | 'In_Transit' | 'Unknown' | string

interface StatusBadgeProps {
  state: ModelState | undefined | null
  className?: string
}

export function StatusBadge({ state, className = '' }: StatusBadgeProps) {
  const normalizedState = state || 'Unknown'

  const getStatusClasses = () => {
    switch (normalizedState) {
      case 'Ready':
        return 'bg-green-100 text-green-800 border-green-200'
      case 'Failed':
        return 'bg-red-100 text-red-800 border-red-200'
      case 'In_Transit':
        return 'bg-yellow-100 text-yellow-800 border-yellow-200'
      default:
        return 'bg-gray-100 text-gray-800 border-gray-200'
    }
  }

  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-semibold ${getStatusClasses()} ${className}`}
    >
      {normalizedState}
    </span>
  )
}
