'use client'

import { FormInput } from '../FormField'

interface GCSStorageProps {
  bucket: string
  object: string
  onBucketChange: (value: string) => void
  onObjectChange: (value: string) => void
}

export function GCSStorage({ bucket, object, onBucketChange, onObjectChange }: GCSStorageProps) {
  return (
    <>
      <FormInput
        label="GCS Bucket"
        required
        value={bucket}
        onChange={(e) => onBucketChange(e.target.value)}
        placeholder="my-bucket"
      />
      <FormInput
        label="Object Path"
        value={object}
        onChange={(e) => onObjectChange(e.target.value)}
        placeholder="models/my-model"
        helpText="Optional - path to the object within the bucket"
      />
    </>
  )
}
