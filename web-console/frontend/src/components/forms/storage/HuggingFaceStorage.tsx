'use client'

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
      <div>
        <label className="block text-sm font-medium text-gray-700">
          Model ID * <span className="text-gray-500 text-xs">(e.g., meta-llama/Llama-2-7b)</span>
        </label>
        <input
          type="text"
          value={modelId}
          onChange={(e) => onModelIdChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="meta-llama/Llama-2-7b"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          Branch <span className="text-gray-500 text-xs">(default: main)</span>
        </label>
        <input
          type="text"
          value={branch}
          onChange={(e) => onBranchChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="main"
        />
      </div>
      <div>
        <label htmlFor="hfToken" className="block text-sm font-medium text-gray-700">
          HuggingFace Token <span className="text-gray-500 text-xs">(optional for gated models)</span>
        </label>
        <input
          type="password"
          id="hfToken"
          value={token}
          onChange={(e) => onTokenChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="hf_..."
        />
        <p className="mt-1 text-sm text-gray-500">
          Required for gated models (e.g., Llama, Mistral) or private repos
        </p>
      </div>
    </>
  )
}
