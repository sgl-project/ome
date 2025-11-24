'use client'

interface OCIStorageProps {
  namespace: string
  bucket: string
  prefix: string
  onNamespaceChange: (value: string) => void
  onBucketChange: (value: string) => void
  onPrefixChange: (value: string) => void
}

export function OCIStorage({
  namespace,
  bucket,
  prefix,
  onNamespaceChange,
  onBucketChange,
  onPrefixChange,
}: OCIStorageProps) {
  return (
    <>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          OCI Namespace *
        </label>
        <input
          type="text"
          value={namespace}
          onChange={(e) => onNamespaceChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="my-namespace"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          Bucket Name *
        </label>
        <input
          type="text"
          value={bucket}
          onChange={(e) => onBucketChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="my-bucket"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          Object Path <span className="text-gray-500 text-xs">(optional)</span>
        </label>
        <input
          type="text"
          value={prefix}
          onChange={(e) => onPrefixChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="models/my-model"
        />
      </div>
    </>
  )
}
