'use client'

interface PVCStorageProps {
  name: string
  subPath: string
  onNameChange: (value: string) => void
  onSubPathChange: (value: string) => void
}

export function PVCStorage({
  name,
  subPath,
  onNameChange,
  onSubPathChange,
}: PVCStorageProps) {
  return (
    <>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          PVC Name *
        </label>
        <input
          type="text"
          value={name}
          onChange={(e) => onNameChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="model-storage"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          Sub-path *
        </label>
        <input
          type="text"
          value={subPath}
          onChange={(e) => onSubPathChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="models/my-model"
        />
      </div>
    </>
  )
}
