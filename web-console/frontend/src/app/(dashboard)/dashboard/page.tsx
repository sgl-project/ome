'use client'

import { useModels } from '@/lib/hooks/useModels'
import { useRuntimes } from '@/lib/hooks/useRuntimes'
import { useServices } from '@/lib/hooks/useServices'
import { LoadingState } from '@/components/ui/LoadingState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import Link from 'next/link'

export default function DashboardPage() {
  const { data: modelsData, isLoading: modelsLoading } = useModels()
  const { data: runtimesData, isLoading: runtimesLoading } = useRuntimes()
  const { data: servicesData, isLoading: servicesLoading } = useServices()

  if (modelsLoading || runtimesLoading || servicesLoading) {
    return <LoadingState message="Loading dashboard..." />
  }

  const stats = [
    {
      name: 'Total Models',
      value: modelsData?.total || 0,
      color: 'bg-blue-500',
      textColor: 'text-blue-600',
      href: '/models',
    },
    {
      name: 'Ready Models',
      value: modelsData?.items.filter((m) => m.status?.state === 'Ready').length || 0,
      color: 'bg-green-500',
      textColor: 'text-green-600',
      href: '/models',
    },
    {
      name: 'Runtimes',
      value: runtimesData?.total || 0,
      color: 'bg-purple-500',
      textColor: 'text-purple-600',
      href: '/runtimes',
    },
    {
      name: 'Services',
      value: servicesData?.total || 0,
      color: 'bg-orange-500',
      textColor: 'text-orange-600',
      href: '/services',
    },
  ]

  return (
    <div className="min-h-screen bg-gradient-to-b from-background to-muted/20">
      {/* Header */}
      <header className="border-b border-border/50 bg-card/80 backdrop-blur-sm shadow-sm">
        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <h1 className="text-4xl font-bold tracking-tight bg-gradient-to-r from-primary to-primary/60 bg-clip-text text-transparent">
            Dashboard
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Overview of your OME resources
          </p>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        {/* Stats Grid */}
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4 mb-8">
          <Link
            href="/models"
            className="group relative overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-md transition-all duration-300 animate-in"
          >
            <div className="absolute inset-0 bg-gradient-to-br from-primary/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <div className="relative p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-muted-foreground">
                    Total Models
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold tracking-tight">
                    {modelsData?.total || 0}
                  </dd>
                </div>
                <div className="ml-4">
                  <svg className="h-12 w-12 text-primary/20 group-hover:text-primary/30 group-hover:scale-110 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                  </svg>
                </div>
              </div>
            </div>
          </Link>

          <Link
            href="/models"
            className="group relative overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-md transition-all duration-300 animate-in animate-in-delay-1"
          >
            <div className="absolute inset-0 bg-gradient-to-br from-green-500/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <div className="relative p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-muted-foreground">
                    Ready Models
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-green-600 tracking-tight">
                    {modelsData?.items.filter((m) => m.status?.state === 'Ready').length || 0}
                  </dd>
                </div>
                <div className="ml-4">
                  <svg className="h-12 w-12 text-green-500/20 group-hover:text-green-500/30 group-hover:scale-110 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
              </div>
            </div>
          </Link>

          <Link
            href="/runtimes"
            className="group relative overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-md transition-all duration-300 animate-in animate-in-delay-2"
          >
            <div className="absolute inset-0 bg-gradient-to-br from-purple-500/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <div className="relative p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-muted-foreground">
                    Runtimes
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-purple-600 tracking-tight">
                    {runtimesData?.total || 0}
                  </dd>
                </div>
                <div className="ml-4">
                  <svg className="h-12 w-12 text-purple-500/20 group-hover:text-purple-500/30 group-hover:scale-110 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  </svg>
                </div>
              </div>
            </div>
          </Link>

          <Link
            href="/services"
            className="group relative overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-md transition-all duration-300 animate-in animate-in-delay-3"
          >
            <div className="absolute inset-0 bg-gradient-to-br from-orange-500/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <div className="relative p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-muted-foreground">
                    Services
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-orange-600 tracking-tight">
                    {servicesData?.total || 0}
                  </dd>
                </div>
                <div className="ml-4">
                  <svg className="h-12 w-12 text-orange-500/20 group-hover:text-orange-500/30 group-hover:scale-110 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
                  </svg>
                </div>
              </div>
            </div>
          </Link>
        </div>

        {/* Recent Activity */}
        <div className="mt-8">
          <div className="overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm">
            <div className="border-b border-border/50 px-6 py-5 bg-muted/30">
              <h3 className="text-lg font-semibold tracking-tight">
                Recent Models
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
                      Vendor
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                      Framework
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                      Status
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/50 bg-card/50">
                  {modelsData?.items.slice(0, 5).map((model) => (
                    <tr key={model.metadata.name} className="transition-colors duration-150 hover:bg-muted/30">
                      <td className="whitespace-nowrap px-6 py-4 text-sm font-medium">
                        <Link
                          href={`/models`}
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
                      <td className="whitespace-nowrap px-6 py-4">
                        <StatusBadge state={model.status?.state} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="bg-muted/30 px-6 py-4 text-right">
              <Link
                href="/models"
                className="text-sm font-medium text-primary hover:text-primary/80 transition-colors duration-150 inline-flex items-center gap-1 group"
              >
                View all models
                <svg className="w-4 h-4 group-hover:translate-x-1 transition-transform duration-150" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              </Link>
            </div>
          </div>
        </div>
      </main>
    </div>
  )
}
