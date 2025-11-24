'use client'

import { useServices } from '@/lib/hooks/useServices'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import { StatCard } from '@/components/ui/StatCard'
import { Button, ButtonIcons } from '@/components/ui/Button'
import Link from 'next/link'
import type { InferenceService } from '@/lib/types/service'

// Icons for stat cards
const Icons = {
  total: (
    <svg
      className="w-6 h-6"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={1.5}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z"
      />
    </svg>
  ),
  running: (
    <svg
      className="w-6 h-6"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={1.5}
    >
      <path strokeLinecap="round" strokeLinejoin="round" d="M5.636 5.636a9 9 0 1012.728 0M12 3v9" />
    </svg>
  ),
  pending: (
    <svg
      className="w-6 h-6"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={1.5}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  ),
  failed: (
    <svg
      className="w-6 h-6"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={1.5}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"
      />
    </svg>
  ),
}

export default function ServicesPage() {
  const { data, isLoading, error } = useServices()

  if (isLoading) {
    return <LoadingState message="Loading services..." />
  }

  if (error) {
    return <ErrorState error={error || new Error('Failed to load services')} />
  }

  const runningCount =
    data?.items.filter((s) => s.status?.state === 'Ready' || s.status?.state === 'Running')
      .length || 0
  const pendingCount =
    data?.items.filter((s) => s.status?.state === 'Pending' || s.status?.state === 'Creating')
      .length || 0
  const failedCount = data?.items.filter((s) => s.status?.state === 'Failed').length || 0

  return (
    <div className="min-h-screen pb-12">
      {/* Header */}
      <header className="border-b border-border bg-card/50 backdrop-blur-sm">
        <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          <div className="flex items-start justify-between gap-8">
            <div>
              <h1 className="text-3xl font-semibold tracking-tight text-foreground">
                Inference Services
              </h1>
              <p className="mt-1 text-sm text-muted-foreground">
                Manage your InferenceService deployments
              </p>
            </div>
            <Button href="/services/deploy" icon={ButtonIcons.plus}>
              Deploy Service
            </Button>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {/* Stats */}
        <div className="mb-8 grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard
            label="Total Services"
            value={data?.total || 0}
            icon={Icons.total}
            variant="primary"
            delay={0}
          />
          <StatCard
            label="Running"
            value={runningCount}
            icon={Icons.running}
            variant="success"
            delay={1}
          />
          <StatCard
            label="Pending"
            value={pendingCount}
            icon={Icons.pending}
            variant="warning"
            delay={2}
          />
          <StatCard
            label="Failed"
            value={failedCount}
            icon={Icons.failed}
            variant="destructive"
            delay={3}
          />
        </div>

        {/* Services Table */}
        <div className="overflow-hidden rounded-xl border border-border bg-card shadow-sm">
          <div className="border-b border-border px-6 py-4 bg-muted/30">
            <h3 className="text-base font-semibold tracking-tight">All Services</h3>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-border">
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
                  <tr>
                    <td colSpan={7} className="px-6 py-12 text-center">
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
                            d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z"
                          />
                        </svg>
                        <p className="text-sm text-muted-foreground">No inference services found</p>
                        <Button
                          href="/services/deploy"
                          variant="outline"
                          size="sm"
                          icon={ButtonIcons.plus}
                        >
                          Deploy your first service
                        </Button>
                      </div>
                    </td>
                  </tr>
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
            </table>
          </div>
        </div>
      </main>
    </div>
  )
}
