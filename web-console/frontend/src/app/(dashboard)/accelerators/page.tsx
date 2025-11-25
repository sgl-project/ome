'use client'

import { useAccelerators } from '@/lib/hooks/useAccelerators'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatIcons } from '@/components/ui/Icons'
import { PageHeader, StatsGrid, type StatItem } from '@/components/layout'

export default function AcceleratorsPage() {
  const { data, isLoading, error } = useAccelerators()

  if (isLoading) return <LoadingState message="Loading accelerators..." />
  if (error) return <ErrorState error={error} />

  const uniqueTypes = new Set(data?.items.map((a) => a.spec.acceleratorType)).size || 0

  const stats: StatItem[] = [
    {
      label: 'Total Accelerators',
      value: data?.total || 0,
      icon: StatIcons.total,
      variant: 'primary',
    },
    {
      label: 'Accelerator Types',
      value: uniqueTypes,
      icon: StatIcons.runtimes,
      variant: 'accent',
    },
  ]

  return (
    <div className="min-h-screen">
      <PageHeader
        title="Accelerator Classes"
        description="View available accelerator configurations for your cluster"
      />

      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <StatsGrid stats={stats} columns={3} />

        {/* Accelerators Grid */}
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
          {data?.items.map((accelerator) => (
            <div
              key={accelerator.metadata.name}
              className="overflow-hidden rounded-xl border border-border bg-card shadow-sm transition-all hover:shadow-md hover:border-border/80"
            >
              <div className="p-6">
                <div className="flex items-center justify-between">
                  <h3 className="text-lg font-medium text-foreground">
                    {accelerator.metadata.name}
                  </h3>
                  <span className="text-2xl">⚡</span>
                </div>

                <dl className="mt-4 space-y-3">
                  <div>
                    <dt className="text-xs font-medium text-muted-foreground uppercase">Type</dt>
                    <dd className="mt-1 text-sm text-foreground">
                      {accelerator.spec.acceleratorType || 'Unknown'}
                    </dd>
                  </div>

                  {accelerator.spec.acceleratorCount && (
                    <div>
                      <dt className="text-xs font-medium text-muted-foreground uppercase">Count</dt>
                      <dd className="mt-1 text-sm text-foreground">
                        {accelerator.spec.acceleratorCount}
                      </dd>
                    </div>
                  )}

                  {accelerator.spec.memoryGB && (
                    <div>
                      <dt className="text-xs font-medium text-muted-foreground uppercase">
                        Memory
                      </dt>
                      <dd className="mt-1 text-sm text-foreground">
                        {accelerator.spec.memoryGB} GB
                      </dd>
                    </div>
                  )}

                  <div>
                    <dt className="text-xs font-medium text-muted-foreground uppercase">Created</dt>
                    <dd className="mt-1 text-sm text-foreground">
                      {accelerator.metadata.creationTimestamp
                        ? new Date(accelerator.metadata.creationTimestamp).toLocaleDateString()
                        : 'Unknown'}
                    </dd>
                  </div>
                </dl>
              </div>
            </div>
          ))}
        </div>

        {data?.items.length === 0 && (
          <div className="rounded-xl border border-border bg-card p-12 text-center shadow-sm">
            <p className="text-muted-foreground">No accelerator classes found in your cluster.</p>
          </div>
        )}
      </main>
    </div>
  )
}
