'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRuntimes, useDeleteRuntime } from '@/lib/hooks/useRuntimes'
import { useNamespaces } from '@/lib/hooks/useNamespaces'
import { useBulkSelection } from '@/lib/hooks/useBulkSelection'
import { useSortedData } from '@/lib/hooks/useSortedData'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import { Button, ButtonIcons } from '@/components/ui/Button'
import { Icons, StatIcons } from '@/components/ui/Icons'
import { SortableHeader } from '@/components/ui/SortableHeader'
import { BulkActionDropdown } from '@/components/ui/BulkActionDropdown'
import { Checkbox } from '@/components/ui/Checkbox'
import { ConfirmDeleteModal } from '@/components/ui/Modal'
import {
  ResourcePageHeader,
  StatsGrid,
  ResourceTable,
  EmptyTableState,
  type StatItem,
} from '@/components/layout'
import type { ClusterServingRuntime } from '@/lib/types/runtime'

type SortField = 'name' | 'accelerators' | 'protocol' | 'status' | 'created'

const emptyIcon = <Icons.server size="lg" />

export default function RuntimesPage() {
  const [selectedNamespace, setSelectedNamespace] = useState<string>('')

  const { data, isLoading, error } = useRuntimes(selectedNamespace || undefined)
  const { data: namespacesData } = useNamespaces()
  const deleteRuntime = useDeleteRuntime()

  const getValue = (runtime: ClusterServingRuntime, field: SortField) => {
    switch (field) {
      case 'name':
        return runtime.metadata.name
      case 'accelerators':
        return runtime.spec.acceleratorRequirements?.acceleratorClasses?.join(',') || ''
      case 'protocol':
        return runtime.spec.protocolVersions?.join(',') || ''
      case 'status':
        return runtime.spec.disabled ? 'disabled' : 'active'
      case 'created':
        return runtime.metadata.creationTimestamp || ''
      default:
        return ''
    }
  }

  const {
    sortedData: sortedRuntimes,
    sortField,
    sortDirection,
    handleSort,
  } = useSortedData(data?.items, 'name' as SortField, getValue)

  const {
    selectedItems,
    showDeleteModal,
    isDeleting,
    allSelected,
    someSelected,
    handleSelectAll,
    handleSelectItem,
    handleBulkDelete,
    closeDeleteModal,
    bulkActions,
    deleteModalResourceName,
  } = useBulkSelection({
    items: sortedRuntimes,
    resourceType: 'runtime',
    basePath: '/runtimes',
    deleteMutation: deleteRuntime,
  })

  if (isLoading) return <LoadingState message="Loading runtimes..." />
  if (error) return <ErrorState error={error} />

  const autoSelectCount =
    data?.items.filter((r) => r.spec.supportedModelFormats?.some((f) => f.autoSelect)).length || 0
  const disabledCount = data?.items.filter((r) => r.spec.disabled).length || 0

  const stats: StatItem[] = [
    {
      label: 'Total Runtimes',
      value: data?.total || 0,
      icon: StatIcons.runtimes,
      variant: 'primary',
    },
    { label: 'Auto-Select', value: autoSelectCount, icon: StatIcons.autoSelect, variant: 'accent' },
    { label: 'Disabled', value: disabledCount, icon: StatIcons.disabled, variant: 'muted' },
  ]

  return (
    <div className="min-h-screen pb-12">
      <ResourcePageHeader
        title="Runtimes"
        description="Manage ClusterServingRuntime configurations for model deployment and inference"
        actions={
          <>
            <Button href="/runtimes/import" variant="outline" icon={ButtonIcons.import}>
              Import
            </Button>
            <Button href="/runtimes/new" icon={ButtonIcons.plus}>
              Create Runtime
            </Button>
          </>
        }
      />

      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <StatsGrid stats={stats} columns={3} />

        <ResourceTable
          title="All Runtimes"
          headerActions={
            <BulkActionDropdown actions={bulkActions} selectedCount={selectedItems.size} />
          }
          filterProps={{
            namespaces: namespacesData?.items,
            selectedNamespace,
            onNamespaceChange: setSelectedNamespace,
          }}
        >
          <thead className="bg-muted/50">
            <tr>
              <th className="w-12 px-4 py-3">
                <Checkbox
                  checked={allSelected}
                  indeterminate={someSelected}
                  onChange={handleSelectAll}
                  aria-label="Select all runtimes"
                />
              </th>
              <SortableHeader
                field="name"
                currentField={sortField}
                direction={sortDirection}
                onSort={handleSort}
              >
                Name
              </SortableHeader>
              <SortableHeader
                field="accelerators"
                currentField={sortField}
                direction={sortDirection}
                onSort={handleSort}
              >
                Accelerators
              </SortableHeader>
              <SortableHeader
                field="protocol"
                currentField={sortField}
                direction={sortDirection}
                onSort={handleSort}
              >
                Protocol
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
            {sortedRuntimes.length === 0 ? (
              <EmptyTableState
                colSpan={6}
                icon={emptyIcon}
                message="No runtimes found"
                action={
                  <Button href="/runtimes/new" variant="outline" size="sm" icon={ButtonIcons.plus}>
                    Create your first runtime
                  </Button>
                }
              />
            ) : (
              sortedRuntimes.map((runtime) => (
                <tr
                  key={runtime.metadata.name}
                  className={`transition-colors hover:bg-muted/30 ${
                    selectedItems.has(runtime.metadata.name) ? 'bg-primary/5' : ''
                  }`}
                >
                  <td className="w-12 px-4 py-4">
                    <Checkbox
                      checked={selectedItems.has(runtime.metadata.name)}
                      onChange={(checked) => handleSelectItem(runtime.metadata.name, checked)}
                      aria-label={`Select ${runtime.metadata.name}`}
                    />
                  </td>
                  <td className="whitespace-nowrap px-6 py-4">
                    <Link
                      href={`/runtimes/${runtime.metadata.name}`}
                      className="text-sm font-medium text-primary hover:text-primary/80 transition-colors"
                    >
                      {runtime.metadata.name}
                    </Link>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex flex-wrap gap-1">
                      {runtime.spec.acceleratorRequirements?.acceleratorClasses?.map((acc, idx) => (
                        <span
                          key={idx}
                          className="inline-flex items-center gap-1 rounded-md bg-accent/10 text-accent px-2 py-0.5 text-xs font-medium"
                        >
                          <Icons.bolt size="xs" />
                          {acc}
                        </span>
                      )) || <span className="text-xs text-muted-foreground">Any</span>}
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex flex-wrap gap-1">
                      {runtime.spec.protocolVersions?.map((protocol, idx) => (
                        <span
                          key={idx}
                          className="inline-flex items-center rounded-md bg-primary/10 text-primary px-2 py-0.5 text-xs font-medium"
                        >
                          {protocol}
                        </span>
                      )) || <span className="text-xs text-muted-foreground">-</span>}
                    </div>
                  </td>
                  <td className="whitespace-nowrap px-6 py-4">
                    <StatusBadge state={runtime.spec.disabled ? 'Disabled' : 'Active'} size="sm" />
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                    {runtime.metadata.creationTimestamp
                      ? new Date(runtime.metadata.creationTimestamp).toLocaleDateString()
                      : '-'}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </ResourceTable>
      </main>

      <ConfirmDeleteModal
        isOpen={showDeleteModal}
        onClose={closeDeleteModal}
        onConfirm={handleBulkDelete}
        resourceName={deleteModalResourceName}
        resourceType="runtime"
        isDeleting={isDeleting}
      />
    </div>
  )
}
