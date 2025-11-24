'use client'

interface GCSStorageProps {
  bucket: string
  object: string
  onBucketChange: (value: string) => void
  onObjectChange: (value: string) => void
}

export function GCSStorage({
  bucket,
  object,
  onBucketChange,
  onObjectChange,
}: GCSStorageProps) {
  return (
    <>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          GCS Bucket *
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
          value={object}
          onChange={(e) => onObjectChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="models/my-model"
        />
      </div>
    </>
  )
}
