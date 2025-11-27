'use client'

import { useState, useMemo } from 'react'
import Link from 'next/link'
import { useServices, useDeleteService } from '@/lib/hooks/useServices'
import { useNamespaces } from '@/lib/hooks/useNamespaces'
import { useBulkSelection } from '@/lib/hooks/useBulkSelection'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import { Button, ButtonIcons } from '@/components/ui/Button'
import { Icons, StatIcons } from '@/components/ui/Icons'
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
import type { InferenceService } from '@/lib/types/service'

const emptyIcon = <Icons.server size="lg" />

export default function ServicesPage() {
  const [selectedNamespace, setSelectedNamespace] = useState<string>('')

  const { data, isLoading, error } = useServices(selectedNamespace || undefined)
  const { data: namespacesData } = useNamespaces()
  const deleteService = useDeleteService()

  const services = useMemo(() => data?.items || [], [data?.items])

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
    items: services,
    resourceType: 'service',
    basePath: '/services',
    deleteMutation: deleteService,
  })

  if (isLoading) return <LoadingState message="Loading services..." />
  if (error) return <ErrorState error={error || new Error('Failed to load services')} />

  const runningCount =
    data?.items.filter((s) => s.status?.state === 'Ready' || s.status?.state === 'Running')
      .length || 0
  const pendingCount =
    data?.items.filter((s) => s.status?.state === 'Pending' || s.status?.state === 'Creating')
      .length || 0
  const failedCount = data?.items.filter((s) => s.status?.state === 'Failed').length || 0

  const stats: StatItem[] = [
    {
      label: 'Total Services',
      value: data?.total || 0,
      icon: StatIcons.services,
      variant: 'primary',
    },
    { label: 'Running', value: runningCount, icon: StatIcons.ready, variant: 'success' },
    { label: 'Pending', value: pendingCount, icon: StatIcons.pending, variant: 'warning' },
    { label: 'Failed', value: failedCount, icon: StatIcons.failed, variant: 'destructive' },
  ]

  return (
    <div className="min-h-screen pb-12">
      <ResourcePageHeader
        title="Inference Services"
        description="Manage your InferenceService deployments"
        actions={
          <Button href="/services/deploy" icon={ButtonIcons.plus}>
            Deploy Service
          </Button>
        }
      />

      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <StatsGrid stats={stats} />

        <ResourceTable
          title="All Services"
          headerActions={
            <BulkActionDropdown actions={bulkActions} selectedCount={selectedItems.size} />
          }
          filterProps={{
            namespaces: namespacesData?.items,
            selectedNamespace,
            onNamespaceChange: setSelectedNamespace,
            scopeLabel: 'Namespace:',
            defaultOptionText: 'All namespaces',
            namespaceFormat: (ns) => ns,
          }}
        >
          <thead className="bg-muted/50">
            <tr>
              <th className="w-12 px-4 py-3">
                <Checkbox
                  checked={allSelected}
                  indeterminate={someSelected}
                  onChange={handleSelectAll}
                  aria-label="Select all services"
                />
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Name
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Namespace
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Model
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Runtime
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Replicas
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Status
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Created
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border bg-card">
            {services.length === 0 ? (
              <EmptyTableState
                colSpan={8}
                icon={emptyIcon}
                message="No inference services found"
                action={
                  <Button
                    href="/services/deploy"
                    variant="outline"
                    size="sm"
                    icon={ButtonIcons.plus}
                  >
                    Deploy your first service
                  </Button>
                }
              />
            ) : (
              services.map((service: InferenceService) => (
                <tr
                  key={service.metadata.name}
                  className={`transition-colors hover:bg-muted/30 ${
                    selectedItems.has(service.metadata.name) ? 'bg-primary/5' : ''
                  }`}
                >
                  <td className="w-12 px-4 py-4">
                    <Checkbox
                      checked={selectedItems.has(service.metadata.name)}
                      onChange={(checked) => handleSelectItem(service.metadata.name, checked)}
                      aria-label={`Select ${service.metadata.name}`}
                    />
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm font-medium">
                    <Link
                      href={`/services/${service.metadata.name}`}
                      className="text-primary hover:text-primary/80 transition-colors"
                    >
                      {service.metadata.name}
                    </Link>
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                    <span className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-xs font-medium">
                      {service.metadata.namespace || 'default'}
                    </span>
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                    {service.spec.predictor?.model || '-'}
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                    {service.spec.predictor?.runtime || '-'}
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                    <span className="inline-flex items-center rounded-md bg-accent/10 text-accent px-2 py-0.5 text-xs font-medium">
                      {service.spec.predictor?.replicas || 1}
                    </span>
                  </td>
                  <td className="whitespace-nowrap px-6 py-4">
                    <StatusBadge state={service.status?.state} size="sm" />
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                    {service.metadata.creationTimestamp
                      ? new Date(service.metadata.creationTimestamp).toLocaleDateString()
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
        resourceType="service"
        isDeleting={isDeleting}
      />
    </div>
  )
}
