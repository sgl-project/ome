'use client'

import { useServices } from '@/lib/hooks/useServices'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatusBadge } from '@/components/ui/StatusBadge'

export default function ServicesPage() {
  const { data, isLoading, error } = useServices()

  if (isLoading) {
    return <LoadingState message="Loading services..." />
  }

  if (error) {
    return <ErrorState error={error || new Error('Failed to load services')} />
  }

  return (
    <div className="min-h-screen bg-gradient-to-b from-background to-muted/20">
      {/* Header */}
      <header className="relative border-b border-border/50 bg-card/50 backdrop-blur-sm animate-in">
        <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          <div className="flex items-start justify-between gap-8">
            <div>
              <h1 className="text-4xl font-bold tracking-tight bg-gradient-to-r from-primary to-primary/60 bg-clip-text text-transparent">
                Inference Services
              </h1>
              <p className="mt-2 text-muted-foreground max-w-2xl">
                Manage your InferenceService deployments
              </p>
            </div>
            <div className="flex gap-3 flex-shrink-0">
              <button className="gradient-border relative rounded-lg bg-gradient-to-r from-primary to-accent px-5 py-2.5 text-sm font-medium text-white hover:shadow-lg hover:shadow-primary/25 transition-all">
                <span className="flex items-center gap-2">
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                  </svg>
                  Deploy Service
                </span>
              </button>
            </div>
          </div>
        </div>
      </header>

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
                    Total Services
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold tracking-tight">
                    {data?.total || 0}
                  </dd>
                </div>
                <div className="ml-4">
                  <svg className="h-12 w-12 text-primary/20 group-hover:text-primary/30 group-hover:scale-110 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
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
                    Running
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-green-600 tracking-tight">
                    {data?.items.filter((s) => s.status?.state === 'Ready' || s.status?.state === 'Running').length || 0}
                  </dd>
                </div>
                <div className="ml-4">
                  <svg className="h-12 w-12 text-green-500/20 group-hover:text-green-500/30 group-hover:scale-110 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M14 10l-2 1m0 0l-2-1m2 1v2.5M20 7l-2 1m2-1l-2-1m2 1v2.5M14 4l-2-1-2 1M4 7l2-1M4 7l2 1M4 7v2.5M12 21l-2-1m2 1l2-1m-2 1v-2.5M6 18l-2-1v-2.5M18 18l2-1v-2.5" />
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
                    Pending
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-yellow-600 tracking-tight">
                    {data?.items.filter((s) => s.status?.state === 'Pending' || s.status?.state === 'Creating').length || 0}
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
                    {data?.items.filter((s) => s.status?.state === 'Failed').length || 0}
                  </dd>
                </div>
                <div className="ml-4">
                  <svg className="h-12 w-12 text-red-500/20 group-hover:text-red-500/30 group-hover:scale-110 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Services Table */}
        <div className="overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm">
          <div className="border-b border-border/50 px-6 py-5 bg-muted/30">
            <h3 className="text-lg font-semibold tracking-tight">
              All Services
            </h3>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-border/50">
              <thead className="bg-muted/50 backdrop-blur-sm">
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
              <tbody className="divide-y divide-border/50 bg-card/50">
                {data?.items.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="px-6 py-8 text-center text-sm text-muted-foreground">
                      No inference services found. Deploy your first service to get started.
                    </td>
                  </tr>
                ) : (
                  data?.items.map((service) => (
                    <tr key={service.metadata.name} className="transition-colors duration-150 hover:bg-muted/30">
                      <td className="whitespace-nowrap px-6 py-4 text-sm font-medium">
                        {service.metadata.name}
                      </td>
                      <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                        {service.metadata.namespace || 'default'}
                      </td>
                      <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                        {service.spec.predictor?.model || '-'}
                      </td>
                      <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                        {service.spec.predictor?.runtime || '-'}
                      </td>
                      <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground">
                        {service.spec.predictor?.replicas || '-'}
                      </td>
                      <td className="whitespace-nowrap px-6 py-4">
                        <StatusBadge state={service.status?.state} />
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
