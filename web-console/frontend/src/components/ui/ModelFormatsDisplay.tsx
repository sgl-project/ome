interface ModelFormat {
  name?: string
  modelFramework?: {
    name: string
    version?: string
  }
  modelArchitecture?: string
  quantization?:
    | string
    | {
        name: string
        bits?: number
      }
  version?: string
  autoSelect?: boolean
  priority?: number
}

interface ModelFormatsDisplayProps {
  formats: ModelFormat[]
}

export function ModelFormatsDisplay({ formats }: ModelFormatsDisplayProps) {
  if (!formats || formats.length === 0) return null

  return (
    <div>
      <h4 className="text-sm font-medium text-gray-900 mb-3">Supported Model Formats</h4>
      <div className="space-y-3">
        {formats.map((format, idx) => (
          <div key={idx} className="bg-purple-50 rounded-lg p-3 border border-purple-200">
            <div className="grid grid-cols-1 gap-2 text-xs">
              {format.name && (
                <div className="flex gap-2">
                  <span className="font-medium text-purple-700 min-w-[140px]">Name:</span>
                  <span className="text-purple-900 font-semibold">{format.name}</span>
                </div>
              )}
              {format.modelFramework && (
                <div className="flex gap-2">
                  <span className="font-medium text-purple-700 min-w-[140px]">Framework:</span>
                  <span className="text-purple-900">
                    {format.modelFramework.name}
                    {format.modelFramework.version && ` (v${format.modelFramework.version})`}
                  </span>
                </div>
              )}
              {format.modelArchitecture && (
                <div className="flex gap-2">
                  <span className="font-medium text-purple-700 min-w-[140px]">Architecture:</span>
                  <span className="text-purple-900 font-mono">{format.modelArchitecture}</span>
                </div>
              )}
              {format.quantization && (
                <div className="flex gap-2">
                  <span className="font-medium text-purple-700 min-w-[140px]">Quantization:</span>
                  <span className="text-purple-900">
                    {typeof format.quantization === 'string'
                      ? format.quantization
                      : `${format.quantization.name}${format.quantization.bits ? ` (${format.quantization.bits}-bit)` : ''}`}
                  </span>
                </div>
              )}
              {format.version && (
                <div className="flex gap-2">
                  <span className="font-medium text-purple-700 min-w-[140px]">Version:</span>
                  <span className="text-purple-900">{format.version}</span>
                </div>
              )}
              {format.autoSelect !== undefined && (
                <div className="flex gap-2">
                  <span className="font-medium text-purple-700 min-w-[140px]">Auto-Select:</span>
                  <span
                    className={`font-semibold ${format.autoSelect ? 'text-green-700' : 'text-gray-600'}`}
                  >
                    {format.autoSelect ? 'Enabled' : 'Disabled'}
                  </span>
                </div>
              )}
              {format.priority !== undefined && (
                <div className="flex gap-2">
                  <span className="font-medium text-purple-700 min-w-[140px]">Priority:</span>
                  <span className="text-purple-900 font-semibold">{format.priority}</span>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
