interface ResourceSpec {
  requests?: Record<string, string>
  limits?: Record<string, string>
}

interface ResourceRequirementsProps {
  resources: ResourceSpec | undefined
}

export function ResourceRequirements({ resources }: ResourceRequirementsProps) {
  if (!resources || (!resources.requests && !resources.limits)) {
    return null
  }

  return (
    <div className="mb-6 rounded-lg bg-white p-6 shadow">
      <h2 className="mb-4 text-lg font-medium text-gray-900">Resource Requirements</h2>
      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        {resources.requests && Object.keys(resources.requests).length > 0 && (
          <div>
            <h3 className="mb-2 text-sm font-medium text-gray-700">Requests</h3>
            <dl className="space-y-2">
              {Object.entries(resources.requests).map(([key, value]) => (
                <div key={key} className="flex justify-between">
                  <dt className="text-sm text-gray-500">{key}:</dt>
                  <dd className="text-sm text-gray-900">{value}</dd>
                </div>
              ))}
            </dl>
          </div>
        )}
        {resources.limits && Object.keys(resources.limits).length > 0 && (
          <div>
            <h3 className="mb-2 text-sm font-medium text-gray-700">Limits</h3>
            <dl className="space-y-2">
              {Object.entries(resources.limits).map(([key, value]) => (
                <div key={key} className="flex justify-between">
                  <dt className="text-sm text-gray-500">{key}:</dt>
                  <dd className="text-sm text-gray-900">{value}</dd>
                </div>
              ))}
            </dl>
          </div>
        )}
      </div>
    </div>
  )
}
