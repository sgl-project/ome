'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useServices } from '@/lib/hooks/useServices'
import { useNamespaces } from '@/lib/hooks/useNamespaces'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import { Button, ButtonIcons } from '@/components/ui/Button'
import { Icons, StatIcons } from '@/components/ui/Icons'
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
            {data?.items.length === 0 ? (
              <EmptyTableState
                colSpan={7}
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
              data?.items.map((service: InferenceService) => (
                <tr key={service.metadata.name} className="transition-colors hover:bg-muted/30">
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
    </div>
  )
}
