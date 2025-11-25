'use client'

import { FormInput } from '../FormField'

interface HuggingFaceStorageProps {
  modelId: string
  branch: string
  token: string
  onModelIdChange: (value: string) => void
  onBranchChange: (value: string) => void
  onTokenChange: (value: string) => void
}

export function HuggingFaceStorage({
  modelId,
  branch,
  token,
  onModelIdChange,
  onBranchChange,
  onTokenChange,
}: HuggingFaceStorageProps) {
  return (
    <>
      <FormInput
        label="Model ID"
        required
        value={modelId}
        onChange={(e) => onModelIdChange(e.target.value)}
        placeholder="meta-llama/Llama-2-7b"
        helpText="e.g., meta-llama/Llama-2-7b"
      />
      <FormInput
        label="Branch"
        value={branch}
        onChange={(e) => onBranchChange(e.target.value)}
        placeholder="main"
        helpText="Optional - defaults to main"
      />
      <FormInput
        label="HuggingFace Token"
        type="password"
        value={token}
        onChange={(e) => onTokenChange(e.target.value)}
        placeholder="hf_..."
        helpText="Required for gated models (e.g., Llama, Mistral) or private repos"
      />
    </>
  )
}
