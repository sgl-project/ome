'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { runtimesApi } from '@/lib/api/runtimes'
import { useQueryClient } from '@tanstack/react-query'
import * as yaml from 'js-yaml'
import { ModelFormatsDisplay } from '@/components/runtime/ModelFormatsDisplay'
import { MetadataCollapsible } from '@/components/runtime/MetadataCollapsible'

type ImportMethod = 'upload' | 'url'

// Reusable component for displaying K8s resources
function ResourceDisplay({ resources }: { resources: any }) {
  if (!resources) return null

  return (
    <div className="mt-2 bg-gray-50 border border-gray-200 rounded p-2">
      <p className="text-xs font-semibold text-gray-700 mb-1.5">Resources</p>
      <div className="space-y-1.5 text-xs">
        {resources.requests && (
          <div>
            <span className="font-medium text-gray-600">Requests:</span>
            <div className="ml-3 space-y-0.5">
              {Object.entries(resources.requests).map(([key, value]: [string, any]) => (
                <div key={key} className="flex">
                  <span className="text-gray-500 w-32">{key}:</span>
                  <span className="font-mono text-gray-900">{value}</span>
                </div>
              ))}
            </div>
          </div>
        )}
        {resources.limits && (
          <div>
            <span className="font-medium text-gray-600">Limits:</span>
            <div className="ml-3 space-y-0.5">
              {Object.entries(resources.limits).map(([key, value]: [string, any]) => (
                <div key={key} className="flex">
                  <span className="text-gray-500 w-32">{key}:</span>
                  <span className="font-mono text-gray-900">{value}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export default function ImportRuntimePage() {
  const router = useRouter()
  const queryClient = useQueryClient()

  const [method, setMethod] = useState<ImportMethod>('upload')
  const [yamlContent, setYamlContent] = useState('')
  const [githubUrl, setGithubUrl] = useState('')
  const [parsedRuntime, setParsedRuntime] = useState<any>(null)
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isImporting, setIsImporting] = useState(false)
  const [showYaml, setShowYaml] = useState(false)

  // Handle file upload
  const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return

    const reader = new FileReader()
    reader.onload = (e) => {
      const content = e.target?.result as string
      setYamlContent(content)
      validateYAML(content)
    }
    reader.onerror = () => {
      setError('Failed to read file')
    }
    reader.readAsText(file)
  }

  // Handle drag and drop
  const handleDrop = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    const file = event.dataTransfer.files[0]
    if (!file) return

    const reader = new FileReader()
    reader.onload = (e) => {
      const content = e.target?.result as string
      setYamlContent(content)
      validateYAML(content)
    }
    reader.readAsText(file)
  }

  const handleDragOver = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault()
  }

  // Fetch YAML from GitHub URL
  const fetchFromGitHub = async () => {
    if (!githubUrl) {
      setError('Please enter a GitHub URL')
      return
    }

    setIsLoading(true)
    setError(null)

    try {
      // Convert GitHub URL to raw URL if needed
      let rawUrl = githubUrl
      if (githubUrl.includes('github.com') && !githubUrl.includes('raw.githubusercontent.com')) {
        rawUrl = githubUrl
          .replace('github.com', 'raw.githubusercontent.com')
          .replace('/blob/', '/')
      }

      const response = await fetch(`/api/v1/runtimes/fetch-yaml?url=${encodeURIComponent(rawUrl)}`)
      if (!response.ok) {
        throw new Error(`Failed to fetch: ${response.statusText}`)
      }

      const data = await response.json()
      setYamlContent(data.content)
      validateYAML(data.content)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch YAML from URL')
    } finally {
      setIsLoading(false)
    }
  }

  // Validate and parse YAML
  const validateYAML = (content: string) => {
    try {
      const parsed = yaml.load(content) as any

      // Validate that it's a runtime
      if (!parsed.kind || (parsed.kind !== 'ClusterServingRuntime' && parsed.kind !== 'ServingRuntime')) {
        throw new Error('YAML must be a ClusterServingRuntime or ServingRuntime resource')
      }

      if (!parsed.apiVersion || !parsed.apiVersion.startsWith('ome.io/')) {
        throw new Error('Invalid apiVersion. Must be ome.io/v1beta1 or similar')
      }

      if (!parsed.metadata?.name) {
        throw new Error('Runtime must have a metadata.name field')
      }

      setParsedRuntime(parsed)
      setError(null)
    } catch (err) {
      setParsedRuntime(null)
      setError(err instanceof Error ? err.message : 'Invalid YAML format')
    }
  }

  // Import runtime
  const handleImport = async () => {
    if (!parsedRuntime) {
      setError('Please provide valid YAML first')
      return
    }

    setIsImporting(true)
    setError(null)

    try {
      await runtimesApi.create(parsedRuntime)
      await queryClient.invalidateQueries({ queryKey: ['runtimes'] })
      router.push('/runtimes')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to import runtime')
    } finally {
      setIsImporting(false)
    }
  }

  return (
    <div className="min-h-screen pb-12">
      {/* Header */}
      <header className="relative border-b border-border/50 bg-card/50 backdrop-blur-sm animate-in">
        <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          <Link
            href="/runtimes"
            className="group inline-flex items-center gap-2 text-sm font-medium text-primary hover:text-primary/80 transition-colors mb-4"
          >
            <svg className="w-4 h-4 transition-transform group-hover:-translate-x-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            Back to Runtimes
          </Link>
          <h1 className="text-4xl font-bold tracking-tight">Import Runtime</h1>
          <p className="mt-2 text-muted-foreground max-w-2xl">
            Import a ClusterServingRuntime configuration from a local YAML file or remote GitHub repository
          </p>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
        {error && (
          <div className="mb-6 rounded-lg border border-destructive/20 bg-destructive/10 p-4 animate-in">
            <div className="flex items-start gap-3">
              <svg className="w-5 h-5 text-destructive flex-shrink-0 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
                <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
              </svg>
              <p className="text-sm text-destructive font-medium">{error}</p>
            </div>
          </div>
        )}

        <div className="rounded-xl border border-border bg-card shadow-lg animate-in-delay-1">
          {/* Method Tabs */}
          <div className="border-b border-border/50 px-6">
            <nav className="-mb-px flex gap-8">
              <button
                onClick={() => setMethod('upload')}
                className={`relative py-4 px-1 text-sm font-medium transition-colors ${
                  method === 'upload'
                    ? 'text-primary'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                Upload YAML
                {method === 'upload' && (
                  <span className="absolute bottom-0 left-0 right-0 h-0.5 bg-gradient-to-r from-primary to-accent rounded-full" />
                )}
              </button>
              <button
                onClick={() => setMethod('url')}
                className={`relative py-4 px-1 text-sm font-medium transition-colors ${
                  method === 'url'
                    ? 'text-primary'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                GitHub URL
                {method === 'url' && (
                  <span className="absolute bottom-0 left-0 right-0 h-0.5 bg-gradient-to-r from-primary to-accent rounded-full" />
                )}
              </button>
            </nav>
          </div>
          <div className="p-6">

          {/* Upload YAML Tab */}
          {method === 'upload' && (
            <div className="space-y-6">
              {/* File Upload Area */}
              <div
                onDrop={handleDrop}
                onDragOver={handleDragOver}
                className="group relative border-2 border-dashed border-border rounded-xl p-12 text-center hover:border-primary/50 hover:bg-primary/5 transition-all duration-300 cursor-pointer overflow-hidden"
              >
                <div className="absolute inset-0 bg-gradient-to-br from-primary/5 to-accent/5 opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
                <input
                  type="file"
                  accept=".yaml,.yml"
                  onChange={handleFileUpload}
                  className="hidden"
                  id="file-upload"
                />
                <label htmlFor="file-upload" className="relative cursor-pointer">
                  <div className="mx-auto w-16 h-16 rounded-full bg-primary/10 flex items-center justify-center group-hover:scale-110 transition-transform duration-300">
                    <svg
                      className="h-8 w-8 text-primary"
                      stroke="currentColor"
                      fill="none"
                      viewBox="0 0 48 48"
                    >
                      <path
                        d="M28 8H12a4 4 0 00-4 4v20m32-12v8m0 0v8a4 4 0 01-4 4H12a4 4 0 01-4-4v-4m32-4l-3.172-3.172a4 4 0 00-5.656 0L28 28M8 32l9.172-9.172a4 4 0 015.656 0L28 28m0 0l4 4m4-24h8m-4-4v8m-12 4h.02"
                        strokeWidth={2}
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </div>
                  <p className="mt-4 text-sm font-medium">
                    <span className="text-primary">Click to upload</span> or drag and drop
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground font-mono">YAML files only (.yaml, .yml)</p>
                </label>
              </div>

              {/* Simple YAML status */}
              {yamlContent && (
                <div className="rounded-lg bg-accent/10 border border-accent/20 p-4 animate-in">
                  <div className="flex items-center gap-3">
                    <div className="flex-shrink-0 w-8 h-8 rounded-full bg-accent flex items-center justify-center">
                      <svg className="h-4 w-4 text-white" fill="currentColor" viewBox="0 0 20 20">
                        <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                      </svg>
                    </div>
                    <div>
                      <p className="text-sm font-medium text-foreground">YAML loaded successfully</p>
                      <p className="text-xs text-muted-foreground font-mono">{yamlContent.split('\n').length} lines</p>
                    </div>
                  </div>
                </div>
              )}
              {!yamlContent && (
                <div>
                  <label className="block text-sm font-medium mb-3">
                    Or paste YAML directly
                  </label>
                  <textarea
                    value={yamlContent}
                    onChange={(e) => {
                      setYamlContent(e.target.value)
                      validateYAML(e.target.value)
                    }}
                    rows={12}
                    className="block w-full rounded-lg border border-border bg-background px-4 py-3 shadow-sm focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20 font-mono text-sm transition-all"
                    placeholder="apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: my-runtime
spec:
  ..."
                  />
                </div>
              )}
            </div>
          )}

          {/* GitHub URL Tab */}
          {method === 'url' && (
            <div className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  GitHub URL
                </label>
                <div className="flex gap-3">
                  <input
                    type="url"
                    value={githubUrl}
                    onChange={(e) => setGithubUrl(e.target.value)}
                    placeholder="https://github.com/user/repo/blob/main/runtime.yaml"
                    className="flex-1 rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-purple-500"
                  />
                  <button
                    onClick={fetchFromGitHub}
                    disabled={isLoading || !githubUrl}
                    className="rounded-lg bg-purple-600 px-4 py-2 text-sm font-medium text-white hover:bg-purple-700 disabled:bg-gray-400 disabled:cursor-not-allowed"
                  >
                    {isLoading ? 'Fetching...' : 'Fetch'}
                  </button>
                </div>
                <p className="mt-2 text-xs text-gray-500">
                  Paste a GitHub URL pointing to a YAML file. Works with both github.com and raw.githubusercontent.com URLs.
                </p>
              </div>

              {/* YAML Status */}
              {yamlContent && (
                <div className="rounded-lg bg-blue-50 border border-accent/20 p-3">
                  <div className="flex items-center">
                    <svg className="h-5 w-5 text-blue-600 mr-2" fill="currentColor" viewBox="0 0 20 20">
                      <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
                    </svg>
                    <span className="text-sm text-accent/90">YAML fetched successfully ({yamlContent.split('\n').length} lines)</span>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Parsed Runtime Preview */}
          {parsedRuntime && (
            <div className="mt-6 space-y-4">
              {/* Success Banner with Runtime Type */}
              <div className="rounded-lg bg-green-50 border border-green-200 p-4">
                <div className="flex items-start justify-between">
                  <div className="flex items-start">
                    <svg className="h-5 w-5 text-green-600 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
                      <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
                    </svg>
                    <div className="ml-3">
                      <h3 className="text-sm font-medium text-green-800">Valid Runtime Configuration</h3>
                      <p className="mt-1 text-sm text-green-700">
                        The YAML has been validated and is ready to import.
                      </p>
                    </div>
                  </div>
                  {/* Runtime Type Badge */}
                  <div>
                    {parsedRuntime.spec?.engineConfig && parsedRuntime.spec?.decoderConfig && (
                      <span className="inline-flex items-center rounded-md bg-purple-100 px-3 py-1.5 text-xs font-bold text-purple-900 ring-1 ring-inset ring-purple-600/20">
                        🚀 PREFILL-DECODE DISAGGREGATED (PDD)
                      </span>
                    )}
                    {parsedRuntime.spec?.engineConfig && !parsedRuntime.spec?.decoderConfig && (
                      <span className="inline-flex items-center rounded-md bg-accent/20 px-3 py-1.5 text-xs font-bold text-blue-900 ring-1 ring-inset ring-blue-600/20">
                        ENGINE ONLY
                      </span>
                    )}
                    {!parsedRuntime.spec?.engineConfig && parsedRuntime.spec?.decoderConfig && (
                      <span className="inline-flex items-center rounded-md bg-accent/20 px-3 py-1.5 text-xs font-bold text-orange-900 ring-1 ring-inset ring-orange-600/20">
                        DECODER ONLY
                      </span>
                    )}
                    {parsedRuntime.spec?.containers && !parsedRuntime.spec?.engineConfig && !parsedRuntime.spec?.decoderConfig && (
                      <span className="inline-flex items-center rounded-md bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-900 ring-1 ring-inset ring-gray-600/20">
                        STANDARD RUNTIME
                      </span>
                    )}
                  </div>
                </div>
              </div>

              {/* Runtime Details */}
              <div className="rounded-lg border border-gray-200 bg-white overflow-hidden">
                <div className="px-4 py-3 bg-gray-50 border-b border-gray-200">
                  <h3 className="text-base font-medium text-gray-900">Runtime Configuration</h3>
                </div>

                <div className="p-6 space-y-6">
                  {/* Basic Info */}
                  <div>
                    <h4 className="text-sm font-medium text-gray-900 mb-3">Basic Information</h4>
                    <dl className="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2">
                      <div>
                        <dt className="text-xs font-medium text-gray-500">Name</dt>
                        <dd className="mt-1 text-sm text-gray-900">{parsedRuntime.metadata?.name}</dd>
                      </div>
                      <div>
                        <dt className="text-xs font-medium text-gray-500">Kind</dt>
                        <dd className="mt-1 text-sm text-gray-900">{parsedRuntime.kind}</dd>
                      </div>
                      <div>
                        <dt className="text-xs font-medium text-gray-500">API Version</dt>
                        <dd className="mt-1 text-sm text-gray-900">{parsedRuntime.apiVersion}</dd>
                      </div>
                      {parsedRuntime.spec?.disabled !== undefined && (
                        <div>
                          <dt className="text-xs font-medium text-gray-500">Status</dt>
                          <dd className="mt-1">
                            <span className={`inline-flex rounded-full px-2 text-xs font-semibold leading-5 ${
                              parsedRuntime.spec.disabled ? 'bg-gray-100 text-gray-800' : 'bg-green-100 text-green-800'
                            }`}>
                              {parsedRuntime.spec.disabled ? 'Disabled' : 'Active'}
                            </span>
                          </dd>
                        </div>
                      )}
                      {parsedRuntime.spec?.multiModel !== undefined && (
                        <div>
                          <dt className="text-xs font-medium text-gray-500">Multi-Model</dt>
                          <dd className="mt-1">
                            <span className={`inline-flex rounded-full px-2 text-xs font-semibold leading-5 ${
                              parsedRuntime.spec.multiModel ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                            }`}>
                              {parsedRuntime.spec.multiModel ? 'Enabled' : 'Disabled'}
                            </span>
                          </dd>
                        </div>
                      )}
                    </dl>
                  </div>

                  {/* Metadata - Labels and Annotations */}
                  <MetadataCollapsible
                    labels={parsedRuntime.metadata?.labels}
                    annotations={parsedRuntime.metadata?.annotations}
                  />

                  {/* Supported Model Formats */}
                  <ModelFormatsDisplay formats={parsedRuntime.spec?.supportedModelFormats || []} />

                  {/* Protocol Versions */}
                  {parsedRuntime.spec?.protocolVersions && parsedRuntime.spec.protocolVersions.length > 0 && (
                    <div>
                      <h4 className="text-sm font-medium text-gray-900 mb-3">Protocol Versions</h4>
                      <div className="flex flex-wrap gap-2">
                        {parsedRuntime.spec.protocolVersions.map((version: string, idx: number) => (
                          <span
                            key={idx}
                            className="inline-flex items-center rounded-md bg-blue-50 px-3 py-1 text-sm font-medium text-accent ring-1 ring-inset ring-blue-700/10"
                          >
                            {version}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Router Config */}
                  {parsedRuntime.spec?.routerConfig && (
                    <div className="border-l-4 border-accent bg-accent/10 p-4">
                      <h4 className="text-sm font-medium text-accent mb-3">🔀 Router Configuration</h4>
                      <div className="space-y-3">
                        {parsedRuntime.spec.routerConfig.runner && (
                          <div className="bg-white rounded-lg p-3 border border-accent/20">
                            <p className="text-xs font-medium text-gray-700 mb-2">Container: {parsedRuntime.spec.routerConfig.runner.name || 'router-container'}</p>
                            <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {parsedRuntime.spec.routerConfig.runner.image}</p>
                            {parsedRuntime.spec.routerConfig.runner.resources && (
                              <div className="mt-2 bg-gray-50 border border-gray-200 rounded p-2">
                                <p className="text-xs font-semibold text-gray-700 mb-1.5">Resources</p>
                                <div className="space-y-1.5 text-xs">
                                  {parsedRuntime.spec.routerConfig.runner.resources.requests && (
                                    <div>
                                      <span className="font-medium text-gray-600">Requests:</span>
                                      <div className="ml-3 space-y-0.5">
                                        {Object.entries(parsedRuntime.spec.routerConfig.runner.resources.requests).map(([key, value]: [string, any]) => (
                                          <div key={key} className="flex">
                                            <span className="text-gray-500 w-32">{key}:</span>
                                            <span className="font-mono text-gray-900">{value}</span>
                                          </div>
                                        ))}
                                      </div>
                                    </div>
                                  )}
                                  {parsedRuntime.spec.routerConfig.runner.resources.limits && (
                                    <div>
                                      <span className="font-medium text-gray-600">Limits:</span>
                                      <div className="ml-3 space-y-0.5">
                                        {Object.entries(parsedRuntime.spec.routerConfig.runner.resources.limits).map(([key, value]: [string, any]) => (
                                          <div key={key} className="flex">
                                            <span className="text-gray-500 w-32">{key}:</span>
                                            <span className="font-mono text-gray-900">{value}</span>
                                          </div>
                                        ))}
                                      </div>
                                    </div>
                                  )}
                                </div>
                              </div>
                            )}
                            {parsedRuntime.spec.routerConfig.runner.env && parsedRuntime.spec.routerConfig.runner.env.length > 0 && (
                              <details className="mt-2">
                                <summary className="text-xs font-medium text-accent cursor-pointer">Environment Variables ({parsedRuntime.spec.routerConfig.runner.env.length})</summary>
                                <div className="mt-1 space-y-1 pl-2">
                                  {parsedRuntime.spec.routerConfig.runner.env.map((env: any, i: number) => (
                                    <p key={i} className="text-xs text-gray-600 font-mono">{env.name}={env.value}</p>
                                  ))}
                                </div>
                              </details>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Engine Config */}
                  {parsedRuntime.spec?.engineConfig && (
                    <div className="border-l-4 border-accent bg-accent/10 p-4">
                      <h4 className="text-sm font-medium text-accent mb-3">⚡ Engine Configuration (Prefill)</h4>
                      <div className="space-y-3">
                        {/* Single-node: Direct runner */}
                        {parsedRuntime.spec.engineConfig.runner && !parsedRuntime.spec.engineConfig.leader && (
                          <div className="bg-white rounded-lg p-3 border border-accent/20">
                            <p className="text-xs font-bold text-accent/90 mb-2">🖥️ Single Node</p>
                            <p className="text-xs font-medium text-gray-700 mb-1">Container: {parsedRuntime.spec.engineConfig.runner.name || 'N/A'}</p>
                            <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {parsedRuntime.spec.engineConfig.runner.image || 'N/A'}</p>
                            {parsedRuntime.spec.engineConfig.runner.resources && (
                              <div className="mt-2 bg-gray-50 border border-gray-200 rounded p-2">
                                <p className="text-xs font-semibold text-gray-700 mb-1.5">Resources</p>
                                <div className="space-y-1.5 text-xs">
                                  {parsedRuntime.spec.engineConfig.runner.resources.requests && (
                                    <div>
                                      <span className="font-medium text-gray-600">Requests:</span>
                                      <div className="ml-3 space-y-0.5">
                                        {Object.entries(parsedRuntime.spec.engineConfig.runner.resources.requests).map(([key, value]: [string, any]) => (
                                          <div key={key} className="flex">
                                            <span className="text-gray-500 w-32">{key}:</span>
                                            <span className="font-mono text-gray-900">{value}</span>
                                          </div>
                                        ))}
                                      </div>
                                    </div>
                                  )}
                                  {parsedRuntime.spec.engineConfig.runner.resources.limits && (
                                    <div>
                                      <span className="font-medium text-gray-600">Limits:</span>
                                      <div className="ml-3 space-y-0.5">
                                        {Object.entries(parsedRuntime.spec.engineConfig.runner.resources.limits).map(([key, value]: [string, any]) => (
                                          <div key={key} className="flex">
                                            <span className="text-gray-500 w-32">{key}:</span>
                                            <span className="font-mono text-gray-900">{value}</span>
                                          </div>
                                        ))}
                                      </div>
                                    </div>
                                  )}
                                </div>
                              </div>
                            )}
                            {parsedRuntime.spec.engineConfig.runner.env && Array.isArray(parsedRuntime.spec.engineConfig.runner.env) && parsedRuntime.spec.engineConfig.runner.env.length > 0 && (
                              <details className="mt-2">
                                <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({parsedRuntime.spec.engineConfig.runner.env.length})</summary>
                                <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                                  {parsedRuntime.spec.engineConfig.runner.env.map((env: any, i: number) => (
                                    <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || (env.valueFrom ? '[from field]' : '')}</p>
                                  ))}
                                </div>
                              </details>
                            )}
                            {parsedRuntime.spec.engineConfig.runner.command && Array.isArray(parsedRuntime.spec.engineConfig.runner.command) && (
                              <details className="mt-2">
                                <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({parsedRuntime.spec.engineConfig.runner.command.length} args)</summary>
                                <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{parsedRuntime.spec.engineConfig.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                              </details>
                            )}
                            {parsedRuntime.spec.engineConfig.runner.args && Array.isArray(parsedRuntime.spec.engineConfig.runner.args) && parsedRuntime.spec.engineConfig.runner.args.length > 0 && (
                              <details className="mt-2">
                                <summary className="text-xs font-medium text-accent cursor-pointer">📝 Args ({parsedRuntime.spec.engineConfig.runner.args.length})</summary>
                                <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{parsedRuntime.spec.engineConfig.runner.args.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                              </details>
                            )}
                          </div>
                        )}

                        {/* Multi-node: Leader */}
                        {parsedRuntime.spec.engineConfig.leader && (
                          <div className="bg-white rounded-lg p-3 border border-accent/20">
                            <p className="text-xs font-bold text-accent/90 mb-2">👑 Leader Node</p>
                            {parsedRuntime.spec.engineConfig.leader.runner ? (
                              <>
                                <p className="text-xs font-medium text-gray-700 mb-1">Container: {parsedRuntime.spec.engineConfig.leader.runner.name || 'N/A'}</p>
                                <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {parsedRuntime.spec.engineConfig.leader.runner.image || 'N/A'}</p>
                                <ResourceDisplay resources={parsedRuntime.spec.engineConfig.leader.runner.resources} />
                                {parsedRuntime.spec.engineConfig.leader.runner.env && Array.isArray(parsedRuntime.spec.engineConfig.leader.runner.env) && parsedRuntime.spec.engineConfig.leader.runner.env.length > 0 && (
                                  <details className="mt-2">
                                    <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({parsedRuntime.spec.engineConfig.leader.runner.env.length})</summary>
                                    <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                                      {parsedRuntime.spec.engineConfig.leader.runner.env.map((env: any, i: number) => (
                                        <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || env.valueFrom ? '[from field]' : ''}</p>
                                      ))}
                                    </div>
                                  </details>
                                )}
                                {parsedRuntime.spec.engineConfig.leader.runner.command && Array.isArray(parsedRuntime.spec.engineConfig.leader.runner.command) && (
                                  <details className="mt-2">
                                    <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({parsedRuntime.spec.engineConfig.leader.runner.command.length} args)</summary>
                                    <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{parsedRuntime.spec.engineConfig.leader.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                                  </details>
                                )}
                              </>
                            ) : (
                              <p className="text-xs text-gray-500 italic">No runner configuration</p>
                            )}
                            {parsedRuntime.spec.engineConfig.leader.nodeSelector && Object.keys(parsedRuntime.spec.engineConfig.leader.nodeSelector).length > 0 && (
                              <details className="mt-2">
                                <summary className="text-xs font-medium text-accent cursor-pointer">🎯 Node Selector ({Object.keys(parsedRuntime.spec.engineConfig.leader.nodeSelector).length} rules)</summary>
                                <div className="mt-1 space-y-1 pl-2">
                                  {Object.entries(parsedRuntime.spec.engineConfig.leader.nodeSelector).map(([key, value]: [string, any], i: number) => (
                                    <p key={i} className="text-xs text-gray-600 font-mono break-all">{key}: {String(value)}</p>
                                  ))}
                                </div>
                              </details>
                            )}
                          </div>
                        )}

                        {/* Worker */}
                        {parsedRuntime.spec.engineConfig.worker && (
                          <div className="bg-white rounded-lg p-3 border border-accent/20">
                            <div className="flex items-center justify-between mb-2">
                              <p className="text-xs font-bold text-accent/90">👥 Worker Nodes</p>
                              <span className="inline-flex items-center rounded-md bg-accent/20 px-2 py-1 text-xs font-medium text-accent">
                                {parsedRuntime.spec.engineConfig.worker.size || 1} node{(parsedRuntime.spec.engineConfig.worker.size || 1) > 1 ? 's' : ''}
                              </span>
                            </div>
                            {parsedRuntime.spec.engineConfig.worker.runner ? (
                              <>
                                <p className="text-xs font-medium text-gray-700 mb-1">Container: {parsedRuntime.spec.engineConfig.worker.runner.name || 'N/A'}</p>
                                <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {parsedRuntime.spec.engineConfig.worker.runner.image || 'N/A'}</p>
                                <ResourceDisplay resources={parsedRuntime.spec.engineConfig.worker.runner.resources} />
                                {parsedRuntime.spec.engineConfig.worker.runner.env && Array.isArray(parsedRuntime.spec.engineConfig.worker.runner.env) && parsedRuntime.spec.engineConfig.worker.runner.env.length > 0 && (
                                  <details className="mt-2">
                                    <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({parsedRuntime.spec.engineConfig.worker.runner.env.length})</summary>
                                    <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                                      {parsedRuntime.spec.engineConfig.worker.runner.env.map((env: any, i: number) => (
                                        <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || env.valueFrom ? '[from field]' : ''}</p>
                                      ))}
                                    </div>
                                  </details>
                                )}
                                {parsedRuntime.spec.engineConfig.worker.runner.command && Array.isArray(parsedRuntime.spec.engineConfig.worker.runner.command) && (
                                  <details className="mt-2">
                                    <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({parsedRuntime.spec.engineConfig.worker.runner.command.length} args)</summary>
                                    <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{parsedRuntime.spec.engineConfig.worker.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                                  </details>
                                )}
                              </>
                            ) : (
                              <p className="text-xs text-gray-500 italic">No runner configuration</p>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Decoder Config */}
                  {parsedRuntime.spec?.decoderConfig && (
                    <div className="border-l-4 border-accent bg-accent/10 p-4">
                      <h4 className="text-sm font-medium text-accent mb-3">🔄 Decoder Configuration (Decode)</h4>
                      <div className="space-y-3">
                        {/* Single-node: Direct runner */}
                        {parsedRuntime.spec.decoderConfig.runner && !parsedRuntime.spec.decoderConfig.leader && (
                          <div className="bg-white rounded-lg p-3 border border-accent/20">
                            <p className="text-xs font-bold text-accent/90 mb-2">🖥️ Single Node</p>
                            <p className="text-xs font-medium text-gray-700 mb-1">Container: {parsedRuntime.spec.decoderConfig.runner.name || 'N/A'}</p>
                            <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {parsedRuntime.spec.decoderConfig.runner.image || 'N/A'}</p>
                            <ResourceDisplay resources={parsedRuntime.spec.decoderConfig.runner.resources} />
                            {parsedRuntime.spec.decoderConfig.runner.env && Array.isArray(parsedRuntime.spec.decoderConfig.runner.env) && parsedRuntime.spec.decoderConfig.runner.env.length > 0 && (
                              <details className="mt-2">
                                <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({parsedRuntime.spec.decoderConfig.runner.env.length})</summary>
                                <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                                  {parsedRuntime.spec.decoderConfig.runner.env.map((env: any, i: number) => (
                                    <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || (env.valueFrom ? '[from field]' : '')}</p>
                                  ))}
                                </div>
                              </details>
                            )}
                            {parsedRuntime.spec.decoderConfig.runner.command && Array.isArray(parsedRuntime.spec.decoderConfig.runner.command) && (
                              <details className="mt-2">
                                <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({parsedRuntime.spec.decoderConfig.runner.command.length} args)</summary>
                                <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{parsedRuntime.spec.decoderConfig.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                              </details>
                            )}
                            {parsedRuntime.spec.decoderConfig.runner.args && Array.isArray(parsedRuntime.spec.decoderConfig.runner.args) && parsedRuntime.spec.decoderConfig.runner.args.length > 0 && (
                              <details className="mt-2">
                                <summary className="text-xs font-medium text-accent cursor-pointer">📝 Args ({parsedRuntime.spec.decoderConfig.runner.args.length})</summary>
                                <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-2 rounded overflow-x-auto max-h-60">{parsedRuntime.spec.decoderConfig.runner.args.join(' ')}</pre>
                              </details>
                            )}
                          </div>
                        )}

                        {/* Multi-node: Leader */}
                        {parsedRuntime.spec.decoderConfig.leader && (
                          <div className="bg-white rounded-lg p-3 border border-accent/20">
                            <p className="text-xs font-bold text-accent/90 mb-2">👑 Leader Node</p>
                            {parsedRuntime.spec.decoderConfig.leader.runner ? (
                              <>
                                <p className="text-xs font-medium text-gray-700 mb-1">Container: {parsedRuntime.spec.decoderConfig.leader.runner.name || 'N/A'}</p>
                                <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {parsedRuntime.spec.decoderConfig.leader.runner.image || 'N/A'}</p>
                                <ResourceDisplay resources={parsedRuntime.spec.decoderConfig.leader.runner.resources} />
                                {parsedRuntime.spec.decoderConfig.leader.runner.env && Array.isArray(parsedRuntime.spec.decoderConfig.leader.runner.env) && parsedRuntime.spec.decoderConfig.leader.runner.env.length > 0 && (
                                  <details className="mt-2">
                                    <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({parsedRuntime.spec.decoderConfig.leader.runner.env.length})</summary>
                                    <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                                      {parsedRuntime.spec.decoderConfig.leader.runner.env.map((env: any, i: number) => (
                                        <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || env.valueFrom ? '[from field]' : ''}</p>
                                      ))}
                                    </div>
                                  </details>
                                )}
                                {parsedRuntime.spec.decoderConfig.leader.runner.command && Array.isArray(parsedRuntime.spec.decoderConfig.leader.runner.command) && (
                                  <details className="mt-2">
                                    <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({parsedRuntime.spec.decoderConfig.leader.runner.command.length} args)</summary>
                                    <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{parsedRuntime.spec.decoderConfig.leader.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                                  </details>
                                )}
                              </>
                            ) : (
                              <p className="text-xs text-gray-500 italic">No runner configuration</p>
                            )}
                            {parsedRuntime.spec.decoderConfig.leader.nodeSelector && Object.keys(parsedRuntime.spec.decoderConfig.leader.nodeSelector).length > 0 && (
                              <details className="mt-2">
                                <summary className="text-xs font-medium text-accent cursor-pointer">🎯 Node Selector ({Object.keys(parsedRuntime.spec.decoderConfig.leader.nodeSelector).length} rules)</summary>
                                <div className="mt-1 space-y-1 pl-2">
                                  {Object.entries(parsedRuntime.spec.decoderConfig.leader.nodeSelector).map(([key, value]: [string, any], i: number) => (
                                    <p key={i} className="text-xs text-gray-600 font-mono break-all">{key}: {String(value)}</p>
                                  ))}
                                </div>
                              </details>
                            )}
                          </div>
                        )}

                        {/* Worker */}
                        {parsedRuntime.spec.decoderConfig.worker && (
                          <div className="bg-white rounded-lg p-3 border border-accent/20">
                            <div className="flex items-center justify-between mb-2">
                              <p className="text-xs font-bold text-accent/90">👥 Worker Nodes</p>
                              <span className="inline-flex items-center rounded-md bg-accent/20 px-2 py-1 text-xs font-medium text-accent">
                                {parsedRuntime.spec.decoderConfig.worker.size || 1} node{(parsedRuntime.spec.decoderConfig.worker.size || 1) > 1 ? 's' : ''}
                              </span>
                            </div>
                            {parsedRuntime.spec.decoderConfig.worker.runner ? (
                              <>
                                <p className="text-xs font-medium text-gray-700 mb-1">Container: {parsedRuntime.spec.decoderConfig.worker.runner.name || 'N/A'}</p>
                                <p className="text-xs text-gray-600 font-mono break-all mb-2">Image: {parsedRuntime.spec.decoderConfig.worker.runner.image || 'N/A'}</p>
                                <ResourceDisplay resources={parsedRuntime.spec.decoderConfig.worker.runner.resources} />
                                {parsedRuntime.spec.decoderConfig.worker.runner.env && Array.isArray(parsedRuntime.spec.decoderConfig.worker.runner.env) && parsedRuntime.spec.decoderConfig.worker.runner.env.length > 0 && (
                                  <details className="mt-2">
                                    <summary className="text-xs font-medium text-accent cursor-pointer">🔧 Environment Variables ({parsedRuntime.spec.decoderConfig.worker.runner.env.length})</summary>
                                    <div className="mt-1 space-y-1 pl-2 max-h-40 overflow-y-auto">
                                      {parsedRuntime.spec.decoderConfig.worker.runner.env.map((env: any, i: number) => (
                                        <p key={i} className="text-xs text-gray-600 font-mono break-all">{env.name}={env.value || env.valueFrom ? '[from field]' : ''}</p>
                                      ))}
                                    </div>
                                  </details>
                                )}
                                {parsedRuntime.spec.decoderConfig.worker.runner.command && Array.isArray(parsedRuntime.spec.decoderConfig.worker.runner.command) && (
                                  <details className="mt-2">
                                    <summary className="text-xs font-medium text-accent cursor-pointer">💻 Command ({parsedRuntime.spec.decoderConfig.worker.runner.command.length} args)</summary>
                                    <pre className="mt-1 text-xs text-gray-600 font-mono bg-gray-50 p-3 rounded overflow-x-auto max-h-60 whitespace-pre-wrap break-all leading-relaxed">{parsedRuntime.spec.decoderConfig.worker.runner.command.join(' ').replace(/--/g, '\n  --').replace(/^\s+/, '')}</pre>
                                  </details>
                                )}
                              </>
                            ) : (
                              <p className="text-xs text-gray-500 italic">No runner configuration</p>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Standard Containers (if no engine/decoder config) */}
                  {parsedRuntime.spec?.containers && !parsedRuntime.spec?.engineConfig && !parsedRuntime.spec?.decoderConfig && (
                    <div className="border-l-4 border-gray-500 bg-gray-50 p-4">
                      <h4 className="text-sm font-medium text-gray-900 mb-3">📦 Containers ({parsedRuntime.spec.containers.length})</h4>
                      <div className="space-y-3">
                        {parsedRuntime.spec.containers.map((container: any, idx: number) => (
                          <div key={idx} className="bg-white rounded-lg p-3 border border-gray-200">
                            <p className="text-xs font-medium text-gray-700 mb-1">{container.name}</p>
                            <p className="text-xs text-gray-600 font-mono break-all mb-2">{container.image}</p>
                            <ResourceDisplay resources={container.resources} />
                            {container.env && container.env.length > 0 && (
                              <details className="mt-2">
                                <summary className="text-xs font-medium text-gray-700 cursor-pointer">🔧 Environment Variables ({container.env.length})</summary>
                                <div className="mt-1 space-y-1 pl-2">
                                  {container.env.map((env: any, i: number) => (
                                    <p key={i} className="text-xs text-gray-600 font-mono">{env.name}={env.value}</p>
                                  ))}
                                </div>
                              </details>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              </div>

              {/* YAML Content Viewer - After Runtime Configuration */}
              {yamlContent && (
                <div className="border border-gray-200 rounded-lg overflow-hidden">
                  <button
                    onClick={() => setShowYaml(!showYaml)}
                    className="w-full flex items-center justify-between px-4 py-3 bg-gray-50 hover:bg-gray-100"
                  >
                    <span className="text-sm font-medium text-gray-700">📄 YAML Content ({yamlContent.split('\n').length} lines)</span>
                    <svg
                      className={`w-5 h-5 text-gray-500 transform transition-transform ${showYaml ? 'rotate-180' : ''}`}
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                  </button>
                  {showYaml && (
                    <div className="p-4 bg-white">
                      <textarea
                        value={yamlContent}
                        onChange={(e) => {
                          setYamlContent(e.target.value)
                          validateYAML(e.target.value)
                        }}
                        rows={20}
                        className="block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-purple-500 font-mono text-xs"
                        placeholder="YAML content will appear here..."
                      />
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          {/* Actions */}
          <div className="mt-6 flex justify-end gap-3">
            <Link
              href="/runtimes"
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              Cancel
            </Link>
            <button
              onClick={handleImport}
              disabled={!parsedRuntime || isImporting}
              className="rounded-lg bg-purple-600 px-4 py-2 text-sm font-medium text-white hover:bg-purple-700 disabled:bg-gray-400 disabled:cursor-not-allowed"
            >
              {isImporting ? 'Importing...' : 'Import Runtime'}
            </button>
          </div>
        </div>
        </div>
      </main>
    </div>
  )
}
