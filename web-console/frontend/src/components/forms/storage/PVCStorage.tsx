'use client'

import { FormInput } from '../FormField'

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
      <FormInput
        label="PVC Name"
        required
        value={name}
        onChange={(e) => onNameChange(e.target.value)}
        placeholder="model-storage"
      />
      <FormInput
        label="Sub-path"
        required
        value={subPath}
        onChange={(e) => onSubPathChange(e.target.value)}
        placeholder="models/my-model"
      />
    </>
  )
}
