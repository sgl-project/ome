interface KeyValueListProps {
  title: string
  items: Record<string, string>
  truncate?: boolean
}

export function KeyValueList({ title, items, truncate = false }: KeyValueListProps) {
  const entries = Object.entries(items)
  if (entries.length === 0) return null

  return (
    <div>
      <h3 className="text-sm font-medium text-gray-700 mb-2">
        {title} ({entries.length})
      </h3>
      <div className="flex flex-wrap gap-2">
        {entries.map(([key, value]) => (
          <span
            key={key}
            className="inline-flex items-center rounded-md bg-gray-100 px-2 py-1 text-xs max-w-full"
            title={`${key}: ${value}`}
          >
            <span className={`text-gray-500 ${truncate ? 'truncate max-w-[150px]' : ''}`}>
              {key}:
            </span>
            <span
              className={`ml-1 font-medium text-gray-700 ${truncate ? 'truncate max-w-[200px]' : ''}`}
            >
              {value}
            </span>
          </span>
        ))}
      </div>
    </div>
  )
}
