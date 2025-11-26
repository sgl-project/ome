'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import {
  useHuggingFaceSearch,
  useHuggingFaceModelInfo,
  useHuggingFaceModelConfig,
} from '@/lib/hooks/useHuggingFace'
import {
  ModelScope,
  type HuggingFaceModelSearchResult,
  type HuggingFaceSearchParams,
} from '@/lib/types/model'

type WizardStep = 'search' | 'scope' | 'review' | 'importing'

export default function ImportModelPage() {
  const router = useRouter()

  // Wizard state
  const [step, setStep] = useState<WizardStep>('search')
  const [searchQuery, setSearchQuery] = useState('')
  const [searchParams, setSearchParams] = useState<HuggingFaceSearchParams>({})
  const [selectedModel, setSelectedModel] = useState<HuggingFaceModelSearchResult | null>(null)
  const [modelScope, setModelScope] = useState<ModelScope>(ModelScope.Cluster)
  const [namespace, setNamespace] = useState('default')
  const [modelName, setModelName] = useState('')
  const [storagePath, setStoragePath] = useState('')
  const [vendor, setVendor] = useState('')
  const [version, setVersion] = useState('')
  const [huggingfaceToken, setHuggingfaceToken] = useState('')
  const [error, setError] = useState<string | null>(null)

  // Pagination state
  const [currentPage, setCurrentPage] = useState(1)
  const itemsPerPage = 20

  // API hooks
  const { data: searchResults, isLoading: isSearching } = useHuggingFaceSearch(searchParams)
  const { data: modelInfo, isLoading: isLoadingInfo } = useHuggingFaceModelInfo(
    selectedModel?.modelId || null
  )
  const { data: modelConfig, isLoading: isLoadingConfig } = useHuggingFaceModelConfig(
    selectedModel?.modelId || null
  )

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setCurrentPage(1) // Reset to first page on new search
    setSearchParams({
      q: searchQuery,
      limit: itemsPerPage * 5, // Load 5 pages worth of data (100 items) for client-side pagination
      sort: 'downloads',
      direction: 'desc',
    })
  }

  // Calculate paginated results
  const totalResults = searchResults?.length || 0
  const totalPages = Math.ceil(totalResults / itemsPerPage)
  const startIndex = (currentPage - 1) * itemsPerPage
  const endIndex = startIndex + itemsPerPage
  const paginatedResults = searchResults?.slice(startIndex, endIndex) || []

  const handleNextPage = () => {
    if (currentPage < totalPages) {
      setCurrentPage(currentPage + 1)
      // Scroll to top of results
      window.scrollTo({ top: 0, behavior: 'smooth' })
    }
  }

  const handlePrevPage = () => {
    if (currentPage > 1) {
      setCurrentPage(currentPage - 1)
      // Scroll to top of results
      window.scrollTo({ top: 0, behavior: 'smooth' })
    }
  }

  const handleSelectModel = (model: HuggingFaceModelSearchResult) => {
    setSelectedModel(model)
    // Auto-generate model name from HF model ID
    setModelName(model.modelId.replace('/', '-').toLowerCase())
    // Auto-populate vendor from the model author (first part of modelId)
    const authorPart = model.modelId.split('/')[0]
    if (authorPart) {
      setVendor(authorPart)
    }
    setStep('scope')
  }

  const handleScopeNext = () => {
    if (modelScope === ModelScope.Namespace && !namespace) {
      setError('Namespace is required for namespace-scoped models')
      return
    }
    if (!modelName) {
      setError('Model name is required')
      return
    }
    setError(null)
    setStep('review')
  }

  const handleImport = async () => {
    if (!selectedModel || !modelInfo) return

    setStep('importing')
    setError(null)

    try {
      // Detect model format from siblings
      const detectedFormat = modelInfo.detectedFormat || 'safetensors'

      // Build storage spec with hf:// URI scheme
      // Note: The backend automatically adds storage.key when huggingfaceToken is provided
      const storageSpec: { storageUri: string; path?: string } = {
        storageUri: `hf://${selectedModel.modelId}`,
      }
      if (storagePath.trim()) {
        storageSpec.path = storagePath.trim()
      }

      // Build model spec
      const modelSpec: Record<string, unknown> = {
        modelFormat: {
          name: detectedFormat,
        },
        modelType: modelConfig?.model_type || 'text-generation',
        modelArchitecture: modelConfig?.architectures?.[0] || '',
        storage: storageSpec,
        displayName: selectedModel.modelId,
      }

      // Add optional fields if provided
      if (vendor.trim()) {
        modelSpec.vendor = vendor.trim()
      }
      if (version.trim()) {
        modelSpec.version = version.trim()
      }

      // Prepare model data based on scope
      const modelData = {
        apiVersion: 'ome.io/v1beta1',
        kind: modelScope === ModelScope.Cluster ? 'ClusterBaseModel' : 'BaseModel',
        metadata: {
          name: modelName,
          ...(modelScope === ModelScope.Namespace && { namespace }),
        },
        spec: modelSpec,
      }

      // Call appropriate API endpoint
      const apiUrl =
        modelScope === ModelScope.Cluster
          ? `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/v1/models`
          : `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/v1/namespaces/${namespace}/models`

      const response = await fetch(apiUrl, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          model: modelData,
          huggingfaceToken: huggingfaceToken || undefined,
        }),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.details || 'Failed to import model')
      }

      // Success - redirect to models list
      router.push('/models')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to import model')
      setStep('review')
    }
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <Link
            href="/models"
            className="text-sm text-blue-600 hover:text-blue-800 mb-2 inline-block"
          >
            ← Back to Models
          </Link>
          <h1 className="text-3xl font-bold text-gray-900">Import Model from HuggingFace</h1>
          <p className="mt-1 text-sm text-gray-500">
            Search and import models from the HuggingFace Model Hub
          </p>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-4xl px-4 py-6 sm:px-6 lg:px-8">
        {error && (
          <div className="mb-6 rounded-lg bg-red-50 p-4">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        {/* Step 1: Search */}
        {step === 'search' && (
          <div className="rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Search HuggingFace Models</h2>

            <form onSubmit={handleSearch} className="mb-6">
              <div className="flex gap-3">
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Search for models (e.g., meta-llama/Llama-2-7b)"
                  className="flex-1 rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                />
                <button
                  type="submit"
                  disabled={!searchQuery || isSearching}
                  className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:bg-blue-400"
                >
                  {isSearching ? 'Searching...' : 'Search'}
                </button>
              </div>
            </form>

            {/* Search Results */}
            {searchResults && searchResults.length > 0 && (
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <h3 className="text-sm font-medium text-gray-700">
                    Found {totalResults} models - Showing page {currentPage} of {totalPages}
                  </h3>
                  <div className="text-sm text-gray-500">
                    {startIndex + 1}-{Math.min(endIndex, totalResults)} of {totalResults}
                  </div>
                </div>

                {paginatedResults.map((model) => (
                  <div
                    key={model.id}
                    className="cursor-pointer rounded-lg border border-gray-200 p-4 hover:border-blue-500 hover:bg-blue-50"
                    onClick={() => handleSelectModel(model)}
                  >
                    <div className="flex items-start justify-between">
                      <div className="flex-1">
                        <h4 className="font-medium text-gray-900">{model.modelId}</h4>
                        <p className="mt-1 text-sm text-gray-500">
                          {model.pipeline_tag && (
                            <span className="inline-block rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-800 mr-2">
                              {model.pipeline_tag}
                            </span>
                          )}
                          {model.library_name && (
                            <span className="inline-block rounded bg-green-100 px-2 py-0.5 text-xs text-green-800">
                              {model.library_name}
                            </span>
                          )}
                        </p>
                      </div>
                      <div className="ml-4 text-right text-sm text-gray-500">
                        <div>↓ {model.downloads.toLocaleString()}</div>
                        <div>♥ {model.likes.toLocaleString()}</div>
                      </div>
                    </div>
                  </div>
                ))}

                {/* Pagination Controls */}
                {totalPages > 1 && (
                  <div className="flex items-center justify-between border-t border-gray-200 pt-4">
                    <button
                      onClick={handlePrevPage}
                      disabled={currentPage === 1}
                      className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-400"
                    >
                      ← Previous
                    </button>

                    <div className="flex items-center gap-2">
                      {Array.from({ length: totalPages }, (_, i) => i + 1).map((pageNum) => (
                        <button
                          key={pageNum}
                          onClick={() => setCurrentPage(pageNum)}
                          className={`h-10 w-10 rounded-lg text-sm font-medium transition-colors ${
                            pageNum === currentPage
                              ? 'bg-blue-600 text-white'
                              : 'border border-gray-300 text-gray-700 hover:bg-gray-50'
                          }`}
                        >
                          {pageNum}
                        </button>
                      ))}
                    </div>

                    <button
                      onClick={handleNextPage}
                      disabled={currentPage === totalPages}
                      className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-400"
                    >
                      Next →
                    </button>
                  </div>
                )}
              </div>
            )}

            {searchResults && searchResults.length === 0 && (
              <div className="text-center text-gray-500">
                No models found. Try a different search query.
              </div>
            )}
          </div>
        )}

        {/* Step 2: Select Scope */}
        {step === 'scope' && selectedModel && (
          <div className="rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Configure Model Scope</h2>

            <div className="mb-4 rounded-lg bg-blue-50 p-4">
              <p className="text-sm font-medium text-blue-900">Selected Model</p>
              <p className="text-blue-700">{selectedModel.modelId}</p>
            </div>

            <div className="space-y-6">
              {/* Scope Selection */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Model Scope *
                </label>
                <div className="space-y-2">
                  <label className="flex items-center cursor-pointer">
                    <input
                      type="radio"
                      checked={modelScope === ModelScope.Cluster}
                      onChange={() => setModelScope(ModelScope.Cluster)}
                      className="h-4 w-4 border-gray-300 text-blue-600 focus:ring-blue-500"
                    />
                    <span className="ml-2 text-sm text-gray-700">
                      <strong>Cluster-scoped</strong> - Available to all namespaces
                    </span>
                  </label>
                  <label className="flex items-center cursor-pointer">
                    <input
                      type="radio"
                      checked={modelScope === ModelScope.Namespace}
                      onChange={() => setModelScope(ModelScope.Namespace)}
                      className="h-4 w-4 border-gray-300 text-blue-600 focus:ring-blue-500"
                    />
                    <span className="ml-2 text-sm text-gray-700">
                      <strong>Namespace-scoped</strong> - Only available in specific namespace
                    </span>
                  </label>
                </div>
              </div>

              {/* Namespace Input (conditional) */}
              {modelScope === ModelScope.Namespace && (
                <div>
                  <label htmlFor="namespace" className="block text-sm font-medium text-gray-700">
                    Namespace *
                  </label>
                  <input
                    type="text"
                    id="namespace"
                    value={namespace}
                    onChange={(e) => setNamespace(e.target.value)}
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                    placeholder="default"
                  />
                </div>
              )}

              {/* Model Name */}
              <div>
                <label htmlFor="modelName" className="block text-sm font-medium text-gray-700">
                  Model Name *
                </label>
                <input
                  type="text"
                  id="modelName"
                  value={modelName}
                  onChange={(e) => setModelName(e.target.value)}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                  placeholder="my-model"
                />
                <p className="mt-1 text-sm text-gray-500">
                  Must be lowercase alphanumeric with dashes
                </p>
              </div>

              {/* Vendor and Version Row */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label htmlFor="vendor" className="block text-sm font-medium text-gray-700">
                    Vendor
                  </label>
                  <input
                    type="text"
                    id="vendor"
                    value={vendor}
                    onChange={(e) => setVendor(e.target.value)}
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                    placeholder="meta-llama"
                  />
                  <p className="mt-1 text-sm text-gray-500">Model provider or organization</p>
                </div>
                <div>
                  <label htmlFor="version" className="block text-sm font-medium text-gray-700">
                    Version
                  </label>
                  <input
                    type="text"
                    id="version"
                    value={version}
                    onChange={(e) => setVersion(e.target.value)}
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                    placeholder="1.0.0"
                  />
                  <p className="mt-1 text-sm text-gray-500">Optional version identifier</p>
                </div>
              </div>

              {/* Storage Path */}
              <div>
                <label htmlFor="storagePath" className="block text-sm font-medium text-gray-700">
                  Storage Path
                </label>
                <input
                  type="text"
                  id="storagePath"
                  value={storagePath}
                  onChange={(e) => setStoragePath(e.target.value)}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                  placeholder="/raid/models/meta-llama/llama-3.1-8b-instruct"
                />
                <p className="mt-1 text-sm text-gray-500">
                  Local filesystem path where the model will be stored. If not specified, the system
                  will use a default path.
                </p>
              </div>

              {/* HuggingFace Token */}
              <div>
                <label htmlFor="hfToken" className="block text-sm font-medium text-gray-700">
                  HuggingFace Token
                  {selectedModel?.gated && <span className="ml-1 text-red-600">*</span>}
                </label>
                <input
                  type="password"
                  id="hfToken"
                  value={huggingfaceToken}
                  onChange={(e) => setHuggingfaceToken(e.target.value)}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                  placeholder="hf_..."
                />
                <p className="mt-1 text-sm text-gray-500">
                  {selectedModel?.gated ? (
                    <span className="text-amber-600">
                      ⚠️ This model is gated and requires a HuggingFace token
                    </span>
                  ) : (
                    'Optional. Required for gated models or private repos'
                  )}
                </p>
              </div>
            </div>

            <div className="mt-6 flex justify-between">
              <button
                onClick={() => setStep('search')}
                className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
              >
                Back
              </button>
              <button
                onClick={handleScopeNext}
                className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
              >
                Next: Review
              </button>
            </div>
          </div>
        )}

        {/* Step 3: Review and Import */}
        {step === 'review' && selectedModel && (
          <div className="rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Review and Import</h2>

            {isLoadingInfo || isLoadingConfig ? (
              <div className="text-center text-gray-500">Loading model information...</div>
            ) : (
              <div className="space-y-4">
                {/* Model Information */}
                <div className="rounded-lg border border-gray-200 p-4">
                  <h3 className="font-medium text-gray-900 mb-2">Model Information</h3>
                  <dl className="grid grid-cols-2 gap-3 text-sm">
                    <div>
                      <dt className="text-gray-500">Model ID</dt>
                      <dd className="font-medium text-gray-900">{selectedModel.modelId}</dd>
                    </div>
                    <div>
                      <dt className="text-gray-500">Scope</dt>
                      <dd className="font-medium text-gray-900">
                        {modelScope === ModelScope.Cluster ? 'Cluster' : `Namespace: ${namespace}`}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-gray-500">Model Name</dt>
                      <dd className="font-medium text-gray-900">{modelName}</dd>
                    </div>
                    {vendor && (
                      <div>
                        <dt className="text-gray-500">Vendor</dt>
                        <dd className="font-medium text-gray-900">{vendor}</dd>
                      </div>
                    )}
                    {version && (
                      <div>
                        <dt className="text-gray-500">Version</dt>
                        <dd className="font-medium text-gray-900">{version}</dd>
                      </div>
                    )}
                    <div>
                      <dt className="text-gray-500">Storage URI</dt>
                      <dd className="font-medium text-gray-900 font-mono text-xs">
                        hf://{selectedModel.modelId}
                      </dd>
                    </div>
                    {storagePath && (
                      <div>
                        <dt className="text-gray-500">Storage Path</dt>
                        <dd className="font-medium text-gray-900 font-mono text-xs">
                          {storagePath}
                        </dd>
                      </div>
                    )}
                    {huggingfaceToken && (
                      <div>
                        <dt className="text-gray-500">Token Secret</dt>
                        <dd className="font-medium text-gray-900 font-mono text-xs">
                          {modelName}-hf-token
                        </dd>
                      </div>
                    )}
                    {modelInfo?.detectedFormat && (
                      <div>
                        <dt className="text-gray-500">Detected Format</dt>
                        <dd className="font-medium text-gray-900">{modelInfo.detectedFormat}</dd>
                      </div>
                    )}
                    {modelInfo?.estimatedSize != null && modelInfo.estimatedSize > 0 && (
                      <div>
                        <dt className="text-gray-500">Estimated Size</dt>
                        <dd className="font-medium text-gray-900">
                          {(modelInfo.estimatedSize / 1024 / 1024 / 1024).toFixed(2)} GB
                        </dd>
                      </div>
                    )}
                    {modelConfig?.architectures && modelConfig.architectures.length > 0 && (
                      <div>
                        <dt className="text-gray-500">Architecture</dt>
                        <dd className="font-medium text-gray-900">
                          {modelConfig.architectures[0]}
                        </dd>
                      </div>
                    )}
                  </dl>
                </div>

                {/* Auto-detected Config */}
                {modelConfig && (
                  <div className="rounded-lg border border-gray-200 p-4">
                    <h3 className="font-medium text-gray-900 mb-2">Auto-detected Configuration</h3>
                    <dl className="grid grid-cols-2 gap-3 text-sm">
                      {modelConfig.model_type && (
                        <div>
                          <dt className="text-gray-500">Model Type</dt>
                          <dd className="font-medium text-gray-900">{modelConfig.model_type}</dd>
                        </div>
                      )}
                      {modelConfig.hidden_size && (
                        <div>
                          <dt className="text-gray-500">Hidden Size</dt>
                          <dd className="font-medium text-gray-900">{modelConfig.hidden_size}</dd>
                        </div>
                      )}
                      {modelConfig.num_hidden_layers && (
                        <div>
                          <dt className="text-gray-500">Layers</dt>
                          <dd className="font-medium text-gray-900">
                            {modelConfig.num_hidden_layers}
                          </dd>
                        </div>
                      )}
                      {modelConfig.num_attention_heads && (
                        <div>
                          <dt className="text-gray-500">Attention Heads</dt>
                          <dd className="font-medium text-gray-900">
                            {modelConfig.num_attention_heads}
                          </dd>
                        </div>
                      )}
                      {modelConfig.vocab_size && (
                        <div>
                          <dt className="text-gray-500">Vocab Size</dt>
                          <dd className="font-medium text-gray-900">
                            {modelConfig.vocab_size.toLocaleString()}
                          </dd>
                        </div>
                      )}
                      {modelConfig.torch_dtype && (
                        <div>
                          <dt className="text-gray-500">Data Type</dt>
                          <dd className="font-medium text-gray-900">{modelConfig.torch_dtype}</dd>
                        </div>
                      )}
                    </dl>
                  </div>
                )}
              </div>
            )}

            <div className="mt-6 flex justify-between">
              <button
                onClick={() => setStep('scope')}
                className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
              >
                Back
              </button>
              <button
                onClick={handleImport}
                disabled={isLoadingInfo || isLoadingConfig}
                className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:bg-blue-400"
              >
                Import Model
              </button>
            </div>
          </div>
        )}

        {/* Step 4: Importing */}
        {step === 'importing' && (
          <div className="rounded-lg bg-white p-6 shadow text-center">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Importing Model...</h2>
            <div className="flex justify-center">
              <div className="h-8 w-8 animate-spin rounded-full border-4 border-blue-200 border-t-blue-600"></div>
            </div>
            <p className="mt-4 text-sm text-gray-500">
              Creating {modelScope === ModelScope.Cluster ? 'ClusterBaseModel' : 'BaseModel'}{' '}
              resource...
            </p>
          </div>
        )}
      </main>
    </div>
  )
}
