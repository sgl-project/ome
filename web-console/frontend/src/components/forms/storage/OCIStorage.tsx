'use client'

import { FormInput } from '../FormField'

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
      <FormInput
        label="OCI Namespace"
        required
        value={namespace}
        onChange={(e) => onNamespaceChange(e.target.value)}
        placeholder="my-namespace"
      />
      <FormInput
        label="Bucket Name"
        required
        value={bucket}
        onChange={(e) => onBucketChange(e.target.value)}
        placeholder="my-bucket"
      />
      <FormInput
        label="Object Path"
        value={prefix}
        onChange={(e) => onPrefixChange(e.target.value)}
        placeholder="models/my-model"
        helpText="Optional - path to the object within the bucket"
      />
    </>
  )
}
