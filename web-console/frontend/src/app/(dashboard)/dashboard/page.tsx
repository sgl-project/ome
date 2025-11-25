'use client'

import { useModels } from '@/lib/hooks/useModels'
import { useRuntimes } from '@/lib/hooks/useRuntimes'
import { useServices } from '@/lib/hooks/useServices'
import { LoadingState } from '@/components/ui/LoadingState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import { StatCard } from '@/components/ui/StatCard'
import { DataTable, TableLink, TableText } from '@/components/ui/DataTable'
import { Button, ButtonIcons } from '@/components/ui/Button'
import { StatIcons } from '@/components/ui/Icons'
import Link from 'next/link'
import type { BaseModel } from '@/lib/types/model'

export default function DashboardPage() {
  const { data: modelsData, isLoading: modelsLoading } = useModels()
  const { data: runtimesData, isLoading: runtimesLoading } = useRuntimes()
  const { data: servicesData, isLoading: servicesLoading } = useServices()

  if (modelsLoading || runtimesLoading || servicesLoading) {
    return <LoadingState message="Loading dashboard..." />
  }

  const recentModels = modelsData?.items.slice(0, 5) || []

  const columns = [
    {
      key: 'name',
      header: 'Name',
      cell: (model: BaseModel) => (
        <TableLink href={`/models/${model.metadata.name}`}>{model.metadata.name}</TableLink>
      ),
    },
    {
      key: 'vendor',
      header: 'Vendor',
      cell: (model: BaseModel) => <TableText muted>{model.spec.vendor || '-'}</TableText>,
    },
    {
      key: 'framework',
      header: 'Framework',
      cell: (model: BaseModel) => (
        <TableText muted>{model.spec.modelFramework?.name || '-'}</TableText>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      cell: (model: BaseModel) => <StatusBadge state={model.status?.state} size="sm" />,
    },
  ]

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="border-b border-border bg-card/50 backdrop-blur-sm">
        <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          <div className="flex items-start justify-between gap-8">
            <div>
              <h1 className="text-3xl font-semibold tracking-tight text-foreground">Dashboard</h1>
              <p className="mt-1 text-sm text-muted-foreground">Overview of your OME resources</p>
            </div>
            <Button href="/models/import" icon={ButtonIcons.import}>
              Quick Import
            </Button>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {/* Stats Grid */}
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4 mb-8">
          <StatCard
            label="Total Models"
            value={modelsData?.total || 0}
            icon={StatIcons.models}
            href="/models"
            variant="primary"
            delay={0}
          />
          <StatCard
            label="Ready Models"
            value={modelsData?.items.filter((m) => m.status?.state === 'Ready').length || 0}
            icon={StatIcons.ready}
            href="/models"
            variant="success"
            delay={1}
          />
          <StatCard
            label="Runtimes"
            value={runtimesData?.total || 0}
            icon={StatIcons.runtimes}
            href="/runtimes"
            variant="accent"
            delay={2}
          />
          <StatCard
            label="Services"
            value={servicesData?.total || 0}
            icon={StatIcons.services}
            href="/services"
            variant="warning"
            delay={3}
          />
        </div>

        {/* Recent Models Table */}
        <DataTable
          title="Recent Models"
          columns={columns}
          data={recentModels}
          keyExtractor={(model) => model.metadata.name}
          headerActions={
            <Link
              href="/models"
              className="inline-flex items-center gap-1 text-sm font-medium text-primary hover:text-primary/80 transition-colors group"
            >
              View all
              <svg
                className="w-4 h-4 group-hover:translate-x-0.5 transition-transform"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
              </svg>
            </Link>
          }
          emptyState={
            <div className="flex flex-col items-center gap-3 py-4">
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
              <p className="text-sm text-muted-foreground">No models yet</p>
              <Button href="/models/import" variant="outline" size="sm" icon={ButtonIcons.import}>
                Import your first model
              </Button>
            </div>
          }
        />
      </main>
    </div>
  )
}
