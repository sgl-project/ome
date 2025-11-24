'use client'

import { useAccelerators } from '@/lib/hooks/useAccelerators'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'

export default function AcceleratorsPage() {
  const { data, isLoading, error } = useAccelerators()

  if (isLoading) {
    return <LoadingState message="Loading accelerators..." />
  }

  if (error) {
    return <ErrorState error={error} />
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="relative border-b border-border/50 bg-card/50 backdrop-blur-sm animate-in">
        <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          <div>
            <h1 className="text-4xl font-bold tracking-tight bg-gradient-to-r from-primary to-primary/60 bg-clip-text text-transparent">
              Accelerator Classes
            </h1>
            <p className="mt-2 text-muted-foreground max-w-2xl">
              View available accelerator configurations for your cluster
            </p>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        {/* Stats */}
        <div className="mb-6 grid grid-cols-1 gap-6 sm:grid-cols-2">
          <div className="overflow-hidden rounded-lg bg-white shadow">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-gray-500">Total Accelerators</dt>
                  <dd className="mt-1 text-3xl font-semibold text-gray-900">{data?.total || 0}</dd>
                </div>
              </div>
            </div>
          </div>

          <div className="overflow-hidden rounded-lg bg-white shadow">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-gray-500">Accelerator Types</dt>
                  <dd className="mt-1 text-3xl font-semibold text-blue-600">
                    {new Set(data?.items.map((a) => a.spec.acceleratorType)).size || 0}
                  </dd>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Accelerators Grid */}
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
          {data?.items.map((accelerator) => (
            <div
              key={accelerator.metadata.name}
              className="overflow-hidden rounded-lg bg-white shadow transition-shadow hover:shadow-lg"
            >
              <div className="p-6">
                <div className="flex items-center justify-between">
                  <h3 className="text-lg font-medium text-gray-900">{accelerator.metadata.name}</h3>
                  <span className="text-2xl">⚡</span>
                </div>

                <dl className="mt-4 space-y-3">
                  <div>
                    <dt className="text-xs font-medium text-gray-500 uppercase">Type</dt>
                    <dd className="mt-1 text-sm text-gray-900">
                      {accelerator.spec.acceleratorType || 'Unknown'}
                    </dd>
                  </div>

                  {accelerator.spec.acceleratorCount && (
                    <div>
                      <dt className="text-xs font-medium text-gray-500 uppercase">Count</dt>
                      <dd className="mt-1 text-sm text-gray-900">
                        {accelerator.spec.acceleratorCount}
                      </dd>
                    </div>
                  )}

                  {accelerator.spec.memoryGB && (
                    <div>
                      <dt className="text-xs font-medium text-gray-500 uppercase">Memory</dt>
                      <dd className="mt-1 text-sm text-gray-900">{accelerator.spec.memoryGB} GB</dd>
                    </div>
                  )}

                  <div>
                    <dt className="text-xs font-medium text-gray-500 uppercase">Created</dt>
                    <dd className="mt-1 text-sm text-gray-900">
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
          <div className="rounded-lg bg-white p-12 text-center shadow">
            <p className="text-gray-500">No accelerator classes found in your cluster.</p>
          </div>
        )}
      </main>
    </div>
  )
}
