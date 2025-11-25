'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useModels } from '@/lib/hooks/useModels'
import { useNamespaces } from '@/lib/hooks/useNamespaces'
import { useServerEvents } from '@/lib/hooks/useServerEvents'
import { useQueryClient } from '@tanstack/react-query'
import { useSortedData } from '@/lib/hooks/useSortedData'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import { Button, ButtonIcons } from '@/components/ui/Button'
import { StatIcons } from '@/components/ui/Icons'
import { SortableHeader } from '@/components/ui/SortableHeader'
import {
  ResourcePageHeader,
  StatsGrid,
  ResourceTable,
  EmptyTableState,
  type StatItem,
} from '@/components/layout'
import type { ClusterBaseModel } from '@/lib/types/model'

type SortField = 'name' | 'vendor' | 'framework' | 'size' | 'status' | 'created'

const emptyIcon = (
  <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1}>
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      d="M21 7.5l-9-5.25L3 7.5m18 0l-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9"
    />
  </svg>
)

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

  const getValue = (model: ClusterBaseModel, field: SortField) => {
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

  if (isLoading) return <LoadingState message="Loading models..." />
  if (error) return <ErrorState error={error || new Error('Failed to load models')} />

  const readyCount = data?.items.filter((m) => m.status?.state === 'Ready').length || 0
  const transitCount = data?.items.filter((m) => m.status?.state === 'In_Transit').length || 0
  const failedCount = data?.items.filter((m) => m.status?.state === 'Failed').length || 0

  const stats: StatItem[] = [
    { label: 'Total Models', value: data?.total || 0, icon: StatIcons.total, variant: 'primary' },
    { label: 'Ready', value: readyCount, icon: StatIcons.ready, variant: 'success' },
    { label: 'In Transit', value: transitCount, icon: StatIcons.pending, variant: 'warning' },
    { label: 'Failed', value: failedCount, icon: StatIcons.failed, variant: 'destructive' },
  ]

  return (
    <div className="min-h-screen pb-12">
      <ResourcePageHeader
        title="Models"
        description="Manage your ClusterBaseModel and BaseModel resources"
        actions={
          <>
            <Button href="/models/import" variant="outline" icon={ButtonIcons.import}>
              Import from HuggingFace
            </Button>
            <Button href="/models/new" icon={ButtonIcons.plus}>
              Create Model
            </Button>
          </>
        }
      />

      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <StatsGrid stats={stats} />

        <ResourceTable
          title="All Models"
          filterProps={{
            namespaces: namespacesData?.items,
            selectedNamespace,
            onNamespaceChange: setSelectedNamespace,
          }}
        >
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
              <EmptyTableState
                colSpan={6}
                icon={emptyIcon}
                message="No models found"
                action={
                  <Button
                    href="/models/import"
                    variant="outline"
                    size="sm"
                    icon={ButtonIcons.import}
                  >
                    Import your first model
                  </Button>
                }
              />
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
        </ResourceTable>
      </main>
    </div>
  )
}
