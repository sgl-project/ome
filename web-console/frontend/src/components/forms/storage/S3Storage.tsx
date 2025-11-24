'use client'

interface S3StorageProps {
  bucket: string
  region: string
  prefix: string
  onBucketChange: (value: string) => void
  onRegionChange: (value: string) => void
  onPrefixChange: (value: string) => void
}

export function S3Storage({
  bucket,
  region,
  prefix,
  onBucketChange,
  onRegionChange,
  onPrefixChange,
}: S3StorageProps) {
  return (
    <>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          S3 Bucket *
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
          Region <span className="text-gray-500 text-xs">(optional)</span>
        </label>
        <input
          type="text"
          value={region}
          onChange={(e) => onRegionChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="us-west-2"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          Prefix <span className="text-gray-500 text-xs">(optional)</span>
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
