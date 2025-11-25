'use client'

import { FormInput } from '../FormField'

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
      <FormInput
        label="S3 Bucket"
        required
        value={bucket}
        onChange={(e) => onBucketChange(e.target.value)}
        placeholder="my-bucket"
      />
      <FormInput
        label="Region"
        value={region}
        onChange={(e) => onRegionChange(e.target.value)}
        placeholder="us-west-2"
        helpText="Optional - defaults to the configured region"
      />
      <FormInput
        label="Prefix"
        value={prefix}
        onChange={(e) => onPrefixChange(e.target.value)}
        placeholder="models/my-model"
        helpText="Optional - path prefix within the bucket"
      />
    </>
  )
}
