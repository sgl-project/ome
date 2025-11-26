interface NodeListProps {
  title: string
  nodes: string[]
  variant: 'success' | 'error'
}

export function NodeList({ title, nodes, variant }: NodeListProps) {
  if (nodes.length === 0) return null

  const colors = {
    success: 'bg-green-100 text-green-800',
    error: 'bg-red-100 text-red-800',
  }

  return (
    <div className="mt-6">
      <dt className="text-sm font-medium text-gray-500 mb-2">
        {title} ({nodes.length})
      </dt>
      <dd className="flex flex-wrap gap-2">
        {nodes.map((node, index) => (
          <span
            key={index}
            className={`inline-flex items-center rounded-full px-3 py-1 text-xs font-medium ${colors[variant]}`}
          >
            {node}
          </span>
        ))}
      </dd>
    </div>
  )
}
