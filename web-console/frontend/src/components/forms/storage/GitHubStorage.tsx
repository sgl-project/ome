'use client'

interface GitHubStorageProps {
  owner: string
  repo: string
  tag: string
  onOwnerChange: (value: string) => void
  onRepoChange: (value: string) => void
  onTagChange: (value: string) => void
}

export function GitHubStorage({
  owner,
  repo,
  tag,
  onOwnerChange,
  onRepoChange,
  onTagChange,
}: GitHubStorageProps) {
  return (
    <>
      <div>
        <label className="block text-sm font-medium text-gray-700">Owner *</label>
        <input
          type="text"
          value={owner}
          onChange={(e) => onOwnerChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="myorg"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700">Repository *</label>
        <input
          type="text"
          value={repo}
          onChange={(e) => onRepoChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="my-model-repo"
        />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700">
          Tag/Release <span className="text-gray-500 text-xs">(default: latest)</span>
        </label>
        <input
          type="text"
          value={tag}
          onChange={(e) => onTagChange(e.target.value)}
          className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
          placeholder="latest"
        />
      </div>
    </>
  )
}
