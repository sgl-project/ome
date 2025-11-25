interface MetadataCollapsibleProps {
  labels?: Record<string, string>
  annotations?: Record<string, string>
}

export function MetadataCollapsible({ labels, annotations }: MetadataCollapsibleProps) {
  const hasLabels = labels && Object.keys(labels).length > 0
  const hasAnnotations = annotations && Object.keys(annotations).length > 0

  if (!hasLabels && !hasAnnotations) return null

  return (
    <div className="space-y-3">
      {/* Labels */}
      {hasLabels && (
        <details className="group">
          <summary className="cursor-pointer list-none">
            <div className="flex items-center gap-2 text-sm font-medium text-gray-900 hover:text-primary transition-colors">
              <svg
                className="w-4 h-4 transition-transform group-open:rotate-90"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9 5l7 7-7 7"
                />
              </svg>
              <span>Labels ({Object.keys(labels).length})</span>
            </div>
          </summary>
          <div className="mt-2 ml-6 space-y-1">
            {Object.entries(labels).map(([key, value]) => (
              <div key={key} className="flex gap-2 text-xs">
                <span className="font-medium text-gray-600 min-w-[200px]">{key}:</span>
                <span className="text-gray-900 font-mono break-all">{String(value)}</span>
              </div>
            ))}
          </div>
        </details>
      )}

      {/* Annotations */}
      {hasAnnotations && (
        <details className="group">
          <summary className="cursor-pointer list-none">
            <div className="flex items-center gap-2 text-sm font-medium text-gray-900 hover:text-primary transition-colors">
              <svg
                className="w-4 h-4 transition-transform group-open:rotate-90"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9 5l7 7-7 7"
                />
              </svg>
              <span>Annotations ({Object.keys(annotations).length})</span>
            </div>
          </summary>
          <div className="mt-2 ml-6 space-y-1">
            {Object.entries(annotations).map(([key, value]) => (
              <div key={key} className="flex gap-2 text-xs">
                <span className="font-medium text-gray-600 min-w-[200px]">{key}:</span>
                <span className="text-gray-900 font-mono break-all">{String(value)}</span>
              </div>
            ))}
          </div>
        </details>
      )}
    </div>
  )
}
