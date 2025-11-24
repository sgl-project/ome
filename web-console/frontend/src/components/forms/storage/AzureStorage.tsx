'use client'

import { FormInput } from '../FormField'

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
      <FormInput
        label="Storage Account"
        required
        value={account}
        onChange={(e) => onAccountChange(e.target.value)}
        placeholder="mystorageaccount"
      />
      <FormInput
        label="Container"
        required
        value={container}
        onChange={(e) => onContainerChange(e.target.value)}
        placeholder="models"
      />
      <FormInput
        label="Blob Path"
        value={blobPath}
        onChange={(e) => onBlobPathChange(e.target.value)}
        placeholder="my-model"
        helpText="Optional - path within the container"
      />
    </>
  )
}
