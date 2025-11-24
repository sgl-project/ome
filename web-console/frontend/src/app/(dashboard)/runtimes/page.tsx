'use client'

import { useRuntimes } from '@/lib/hooks/useRuntimes'
import { useNamespaces } from '@/lib/hooks/useNamespaces'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatusBadge } from '@/components/ui/StatusBadge'
import { StatCard } from '@/components/ui/StatCard'
import { Button, ButtonIcons } from '@/components/ui/Button'
import { StatIcons } from '@/components/ui/Icons'
import { useSortedData } from '@/hooks/useSortedData'
import { SortableHeader } from '@/components/ui/SortableHeader'
import Link from 'next/link'
import { useState } from 'react'
import type { ClusterServingRuntime } from '@/lib/types/runtime'

type SortField = 'name' | 'accelerators' | 'protocol' | 'status' | 'created'

export default function RuntimesPage() {
  const [selectedNamespace, setSelectedNamespace] = useState<string>('')
  const { data, isLoading, error } = useRuntimes(selectedNamespace || undefined)
  const { data: namespacesData } = useNamespaces()

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

  if (isLoading) {
    return <LoadingState message="Loading runtimes..." />
  }

  if (error) {
    return <ErrorState error={error} />
  }

  const autoSelectCount =
    data?.items.filter((r) => r.spec.supportedModelFormats?.some((f) => f.autoSelect)).length || 0
  const disabledCount = data?.items.filter((r) => r.spec.disabled).length || 0

  return (
    <div className="min-h-screen pb-12">
      {/* Header */}
      <header className="border-b border-border bg-card/50 backdrop-blur-sm">
        <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          <div className="flex items-start justify-between gap-8">
            <div>
              <h1 className="text-3xl font-semibold tracking-tight text-foreground">Runtimes</h1>
              <p className="mt-1 text-sm text-muted-foreground">
                Manage ClusterServingRuntime configurations for model deployment and inference
              </p>
            </div>
            <div className="flex gap-3">
              <Button href="/runtimes/import" variant="outline" icon={ButtonIcons.import}>
                Import
              </Button>
              <Button href="/runtimes/new" icon={ButtonIcons.plus}>
                Create Runtime
              </Button>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {/* Stats */}
        <div className="mb-8 grid grid-cols-1 gap-5 sm:grid-cols-3">
          <StatCard
            label="Total Runtimes"
            value={data?.total || 0}
            icon={StatIcons.runtimes}
            variant="primary"
            delay={0}
          />
          <StatCard
            label="Auto-Select"
            value={autoSelectCount}
            icon={StatIcons.autoSelect}
            variant="accent"
            delay={1}
          />
          <StatCard
            label="Disabled"
            value={disabledCount}
            icon={StatIcons.disabled}
            variant="muted"
            delay={2}
          />
        </div>

        {/* Runtimes Table */}
        <div className="overflow-hidden rounded-xl border border-border bg-card shadow-sm">
          <div className="flex items-center justify-between border-b border-border px-6 py-4 bg-muted/30">
            <h3 className="text-base font-semibold tracking-tight">All Runtimes</h3>
            <div className="flex items-center gap-3">
              <label htmlFor="namespace" className="text-sm font-medium text-muted-foreground">
                Scope:
              </label>
              <select
                id="namespace"
                value={selectedNamespace}
                onChange={(e) => setSelectedNamespace(e.target.value)}
                className="rounded-lg border border-border bg-background px-3 py-2 text-sm shadow-sm transition-colors focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20"
              >
                <option value="">Cluster-scoped</option>
                {namespacesData?.items.map((ns) => (
                  <option key={ns} value={ns}>
                    Namespace: {ns}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-border">
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
                  <tr>
                    <td colSpan={5} className="px-6 py-12 text-center">
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
                        <p className="text-sm text-muted-foreground">No runtimes found</p>
                        <Button
                          href="/runtimes/new"
                          variant="outline"
                          size="sm"
                          icon={ButtonIcons.plus}
                        >
                          Create your first runtime
                        </Button>
                      </div>
                    </td>
                  </tr>
                ) : (
                  sortedRuntimes.map((runtime) => (
                    <tr key={runtime.metadata.name} className="transition-colors hover:bg-muted/30">
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
                          {runtime.spec.acceleratorRequirements?.acceleratorClasses?.map(
                            (acc, idx) => (
                              <span
                                key={idx}
                                className="inline-flex items-center gap-1 rounded-md bg-accent/10 text-accent px-2 py-0.5 text-xs font-medium"
                              >
                                <svg
                                  className="w-3 h-3"
                                  fill="none"
                                  viewBox="0 0 24 24"
                                  stroke="currentColor"
                                  strokeWidth={2}
                                >
                                  <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z"
                                  />
                                </svg>
                                {acc}
                              </span>
                            )
                          ) || <span className="text-xs text-muted-foreground">Any</span>}
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
                        <StatusBadge
                          state={runtime.spec.disabled ? 'Disabled' : 'Active'}
                          size="sm"
                        />
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
            </table>
          </div>
        </div>
      </main>
    </div>
  )
}
