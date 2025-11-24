'use client'

import { useModels } from '@/lib/hooks/useModels'
import { useNamespaces } from '@/lib/hooks/useNamespaces'
import { useServerEvents } from '@/hooks/useServerEvents'
import { useQueryClient } from '@tanstack/react-query'
import Link from 'next/link'
import { useState } from 'react'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import { PageHeader } from '@/components/layout/PageHeader'
import { useSortedData } from '@/hooks/useSortedData'
import { SortableHeader } from '@/components/ui/SortableHeader'
import type { BaseModel } from '@/types/model'

type SortField = 'name' | 'vendor' | 'framework' | 'size' | 'status' | 'created'

export default function ModelsPage() {
  const [selectedNamespace, setSelectedNamespace] = useState<string>('')
  const { data, isLoading, error } = useModels(selectedNamespace || undefined)
  const { data: namespacesData } = useNamespaces()
  const queryClient = useQueryClient()

  // Connect to SSE for real-time updates
  useServerEvents({
    onEvent: (event) => {
      if (event.resource === 'models') {
        // Invalidate models query to trigger refetch
        queryClient.invalidateQueries({ queryKey: ['models'] })
      }
    },
    onConnected: () => {
      console.log('Connected to real-time updates')
    },
  })

  const getValue = (model: BaseModel, field: SortField) => {
    switch (field) {
      case 'name':
        return model.metadata.name
      case 'vendor':
        return model.spec.vendor || ''
      case 'framework':
        return model.spec.modelFramework?.name || ''
      case 'size':
        return model.spec.modelParameterSize || ''
      case 'status':
        return model.status?.state || ''
      case 'created':
        return model.metadata.creationTimestamp || ''
      default:
        return ''
    }
  }

  const { sortedData: sortedModels, sortField, sortDirection, handleSort } = useSortedData(
    data?.items,
    'name' as SortField,
    getValue
  )

  if (isLoading) {
    return <LoadingState message="Loading models..." />
  }

  if (error) {
    return <ErrorState error={error || new Error('Failed to load models')} />
  }

  return (
    <div className="min-h-screen pb-12">
      <PageHeader
        title="Models"
        description="Manage your ClusterBaseModel and BaseModel resources"
        actions={
          <>
            <Link
              href="/models/import"
              className="group relative rounded-lg border border-primary px-4 py-2.5 text-sm font-medium text-primary hover:bg-primary/5 transition-all overflow-hidden"
            >
              <span className="relative z-10 flex items-center gap-2">
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                </svg>
                Import from HuggingFace
              </span>
            </Link>
            <Link
              href="/models/new"
              className="gradient-border relative rounded-lg bg-gradient-to-r from-primary to-accent px-5 py-2.5 text-sm font-medium text-white hover:shadow-lg hover:shadow-primary/25 transition-all"
            >
              <span className="flex items-center gap-2">
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                </svg>
                Create Model
              </span>
            </Link>
          </>
        }
      />

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        {/* Stats */}
        <div className="mb-6 grid grid-cols-1 gap-6 sm:grid-cols-4">
          <div className="group relative overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-md transition-all duration-300 animate-in">
            <div className="absolute inset-0 bg-gradient-to-br from-primary/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <div className="relative p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-muted-foreground">
                    Total Models
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold tracking-tight">
                    {data?.total || 0}
                  </dd>
                </div>
                <div className="ml-4">
                  <svg className="h-12 w-12 text-primary/20 group-hover:text-primary/30 group-hover:scale-110 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                  </svg>
                </div>
              </div>
            </div>
          </div>

          <div className="group relative overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-md transition-all duration-300 animate-in animate-in-delay-1">
            <div className="absolute inset-0 bg-gradient-to-br from-green-500/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <div className="relative p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-muted-foreground">
                    Ready
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-green-600 tracking-tight">
                    {data?.items.filter((m) => m.status?.state === 'Ready').length || 0}
                  </dd>
                </div>
                <div className="ml-4">
                  <svg className="h-12 w-12 text-green-500/20 group-hover:text-green-500/30 group-hover:scale-110 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
              </div>
            </div>
          </div>

          <div className="group relative overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-md transition-all duration-300 animate-in animate-in-delay-2">
            <div className="absolute inset-0 bg-gradient-to-br from-yellow-500/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <div className="relative p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-muted-foreground">
                    In Transit
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-yellow-600 tracking-tight">
                    {data?.items.filter((m) => m.status?.state === 'In_Transit').length || 0}
                  </dd>
                </div>
                <div className="ml-4">
                  <svg className="h-12 w-12 text-yellow-500/20 group-hover:text-yellow-500/30 group-hover:scale-110 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
              </div>
            </div>
          </div>

          <div className="group relative overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-md transition-all duration-300 animate-in animate-in-delay-3">
            <div className="absolute inset-0 bg-gradient-to-br from-red-500/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <div className="relative p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-muted-foreground">
                    Failed
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-red-600 tracking-tight">
                    {data?.items.filter((m) => m.status?.state === 'Failed').length || 0}
                  </dd>
                </div>
                <div className="ml-4">
                  <svg className="h-12 w-12 text-red-500/20 group-hover:text-red-500/30 group-hover:scale-110 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Models Table */}
        <div className="overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm">
          <div className="border-b border-border/50 px-6 py-5 bg-muted/30">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold tracking-tight">
                All Models
              </h3>
              {/* Namespace Selector */}
              <div className="flex items-center gap-2">
                <label htmlFor="namespace" className="text-sm font-medium text-muted-foreground">
                  Scope:
                </label>
                <select
                  id="namespace"
                  value={selectedNamespace}
                  onChange={(e) => setSelectedNamespace(e.target.value)}
                  className="rounded-lg border border-border/50 bg-background/50 backdrop-blur-sm px-3 py-2 text-sm shadow-sm transition-all duration-200 focus:border-primary/50 focus:outline-none focus:ring-2 focus:ring-primary/20"
                >
                  <option value="">Cluster-scoped</option>
                  {namespacesData?.items.map((ns) => (
                    <option key={ns} value={ns}>
                      Namespace: {ns}
                    </option>
                  ))}
                </select>
              </div>
            </div>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-border/50">
              <thead className="bg-muted/20">
                <tr>
                  <SortableHeader field="name" currentField={sortField} direction={sortDirection} onSort={handleSort}>Name</SortableHeader>
                  <SortableHeader field="vendor" currentField={sortField} direction={sortDirection} onSort={handleSort}>Vendor</SortableHeader>
                  <SortableHeader field="framework" currentField={sortField} direction={sortDirection} onSort={handleSort}>Framework</SortableHeader>
                  <SortableHeader field="size" currentField={sortField} direction={sortDirection} onSort={handleSort}>Size</SortableHeader>
                  <SortableHeader field="status" currentField={sortField} direction={sortDirection} onSort={handleSort}>Status</SortableHeader>
                  <SortableHeader field="created" currentField={sortField} direction={sortDirection} onSort={handleSort}>Created</SortableHeader>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/50 bg-card/50">
                {sortedModels.map((model) => (
                  <tr key={model.metadata.name} className="transition-colors duration-150 hover:bg-muted/30">
                    <td className="whitespace-nowrap px-6 py-4 text-sm font-medium">
                      <Link
                        href={`/models/${model.metadata.name}`}
                        className="text-primary hover:text-primary/80 transition-colors duration-150 font-medium"
                      >
                        {model.metadata.name}
                      </Link>
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                      {model.spec.vendor || '-'}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                      {model.spec.modelFramework?.name || '-'}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                      {model.spec.modelParameterSize || '-'}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4">
                      <StatusBadge state={model.status?.state} />
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                      {model.metadata.creationTimestamp
                        ? new Date(model.metadata.creationTimestamp).toLocaleDateString()
                        : '-'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </main>
    </div>
  )
}
