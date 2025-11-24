'use client'

interface AzureStorageProps {
  account: string
  container: string
  blobPath: string
  onAccountChange: (value: string) => void
  onContainerChange: (value: string) => void
  onBlobPathChange: (value: string) => void
}

export function AzureStorage({
  account,
  container,
  blobPath,
  onAccountChange,
  onContainerChange,
  onBlobPathChange,
}: AzureStorageProps) {
  return (
    <>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          Storage Account *
        </label>
        <input
          type="text"
          value={account}
          onChange={(e) => onAccountChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="mystorageaccount"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          Container *
        </label>
        <input
          type="text"
          value={container}
          onChange={(e) => onContainerChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="models"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          Blob Path <span className="text-gray-500 text-xs">(optional)</span>
        </label>
        <input
          type="text"
          value={blobPath}
          onChange={(e) => onBlobPathChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="my-model"
        />
      </div>
    </>
  )
}
