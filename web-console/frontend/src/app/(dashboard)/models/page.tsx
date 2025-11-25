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
import { StatCard } from '@/components/ui/StatCard'
import { Button, ButtonIcons } from '@/components/ui/Button'
import { StatIcons } from '@/components/ui/Icons'
import { useSortedData } from '@/hooks/useSortedData'
import { SortableHeader } from '@/components/ui/SortableHeader'
import type { BaseModel } from '@/lib/types/model'

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

  const {
    sortedData: sortedModels,
    sortField,
    sortDirection,
    handleSort,
  } = useSortedData(data?.items, 'name' as SortField, getValue)

  if (isLoading) {
    return <LoadingState message="Loading models..." />
  }

  if (error) {
    return <ErrorState error={error || new Error('Failed to load models')} />
  }

  const readyCount = data?.items.filter((m) => m.status?.state === 'Ready').length || 0
  const transitCount = data?.items.filter((m) => m.status?.state === 'In_Transit').length || 0
  const failedCount = data?.items.filter((m) => m.status?.state === 'Failed').length || 0

  return (
    <div className="min-h-screen pb-12">
      {/* Header */}
      <header className="border-b border-border bg-card/50 backdrop-blur-sm">
        <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          <div className="flex items-start justify-between gap-8">
            <div>
              <h1 className="text-3xl font-semibold tracking-tight text-foreground">Models</h1>
              <p className="mt-1 text-sm text-muted-foreground">
                Manage your ClusterBaseModel and BaseModel resources
              </p>
            </div>
            <div className="flex gap-3">
              <Button href="/models/import" variant="outline" icon={ButtonIcons.import}>
                Import from HuggingFace
              </Button>
              <Button href="/models/new" icon={ButtonIcons.plus}>
                Create Model
              </Button>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {/* Stats */}
        <div className="mb-8 grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard
            label="Total Models"
            value={data?.total || 0}
            icon={StatIcons.total}
            variant="primary"
            delay={0}
          />
          <StatCard
            label="Ready"
            value={readyCount}
            icon={StatIcons.ready}
            variant="success"
            delay={1}
          />
          <StatCard
            label="In Transit"
            value={transitCount}
            icon={StatIcons.pending}
            variant="warning"
            delay={2}
          />
          <StatCard
            label="Failed"
            value={failedCount}
            icon={StatIcons.failed}
            variant="destructive"
            delay={3}
          />
        </div>

        {/* Models Table */}
        <div className="overflow-hidden rounded-xl border border-border bg-card shadow-sm">
          <div className="flex items-center justify-between border-b border-border px-6 py-4 bg-muted/30">
            <h3 className="text-base font-semibold tracking-tight">All Models</h3>
            <div className="flex items-center gap-3">
              <label htmlFor="namespace" className="text-sm font-medium text-muted-foreground">
                Scope:
              </label>
              <select
                id="namespace"
                value={selectedNamespace}
                onChange={(e) => setSelectedNamespace(e.target.value)}
                className="rounded-lg border border-border bg-background px-3 py-2 text-sm shadow-sm transition-colors focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20"
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
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-border">
              <thead className="bg-muted/50">
                <tr>
                  <SortableHeader
                    field="name"
                    currentField={sortField}
                    direction={sortDirection}
                    onSort={handleSort}
                  >
                    Name
                  </SortableHeader>
                  <SortableHeader
                    field="vendor"
                    currentField={sortField}
                    direction={sortDirection}
                    onSort={handleSort}
                  >
                    Vendor
                  </SortableHeader>
                  <SortableHeader
                    field="framework"
                    currentField={sortField}
                    direction={sortDirection}
                    onSort={handleSort}
                  >
                    Framework
                  </SortableHeader>
                  <SortableHeader
                    field="size"
                    currentField={sortField}
                    direction={sortDirection}
                    onSort={handleSort}
                  >
                    Size
                  </SortableHeader>
                  <SortableHeader
                    field="status"
                    currentField={sortField}
                    direction={sortDirection}
                    onSort={handleSort}
                  >
                    Status
                  </SortableHeader>
                  <SortableHeader
                    field="created"
                    currentField={sortField}
                    direction={sortDirection}
                    onSort={handleSort}
                  >
                    Created
                  </SortableHeader>
                </tr>
              </thead>
              <tbody className="divide-y divide-border bg-card">
                {sortedModels.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="px-6 py-12 text-center">
                      <div className="flex flex-col items-center gap-3">
                        <svg
                          className="h-12 w-12 text-muted-foreground/40"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                          strokeWidth={1}
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            d="M21 7.5l-9-5.25L3 7.5m18 0l-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9"
                          />
                        </svg>
                        <p className="text-sm text-muted-foreground">No models found</p>
                        <Button
                          href="/models/import"
                          variant="outline"
                          size="sm"
                          icon={ButtonIcons.import}
                        >
                          Import your first model
                        </Button>
                      </div>
                    </td>
                  </tr>
                ) : (
                  sortedModels.map((model) => (
                    <tr key={model.metadata.name} className="transition-colors hover:bg-muted/30">
                      <td className="whitespace-nowrap px-6 py-4 text-sm font-medium">
                        <Link
                          href={`/models/${model.metadata.name}`}
                          className="text-primary hover:text-primary/80 transition-colors"
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
                        <StatusBadge state={model.status?.state} size="sm" />
                      </td>
                      <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                        {model.metadata.creationTimestamp
                          ? new Date(model.metadata.creationTimestamp).toLocaleDateString()
                          : '-'}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      </main>
    </div>
  )
}
