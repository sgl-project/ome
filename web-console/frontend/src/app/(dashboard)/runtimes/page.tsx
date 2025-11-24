'use client'

import { useRuntimes } from '@/lib/hooks/useRuntimes'
import { useNamespaces } from '@/lib/hooks/useNamespaces'
import Link from 'next/link'
import { useState, useMemo } from 'react'

type SortField = 'name' | 'accelerators' | 'protocol' | 'status' | 'created'
type SortDirection = 'asc' | 'desc'

export default function RuntimesPage() {
  const [selectedNamespace, setSelectedNamespace] = useState<string>('')
  const { data, isLoading, error } = useRuntimes(selectedNamespace || undefined)
  const { data: namespacesData } = useNamespaces()
  const [sortField, setSortField] = useState<SortField>('name')
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc')

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortDirection('asc')
    }
  }

  const sortedRuntimes = useMemo(() => {
    if (!data?.items) return []

    const items = [...data.items]

    items.sort((a, b) => {
      let aValue: any
      let bValue: any

      switch (sortField) {
        case 'name':
          aValue = a.metadata.name.toLowerCase()
          bValue = b.metadata.name.toLowerCase()
          break
        case 'accelerators':
          aValue = a.spec.acceleratorRequirements?.acceleratorClasses?.join(',') || ''
          bValue = b.spec.acceleratorRequirements?.acceleratorClasses?.join(',') || ''
          break
        case 'protocol':
          aValue = a.spec.protocolVersions?.join(',') || ''
          bValue = b.spec.protocolVersions?.join(',') || ''
          break
        case 'status':
          aValue = a.spec.disabled ? 'disabled' : 'active'
          bValue = b.spec.disabled ? 'disabled' : 'active'
          break
        case 'created':
          aValue = a.metadata.creationTimestamp || ''
          bValue = b.metadata.creationTimestamp || ''
          break
      }

      if (aValue < bValue) return sortDirection === 'asc' ? -1 : 1
      if (aValue > bValue) return sortDirection === 'asc' ? 1 : -1
      return 0
    })

    return items
  }, [data?.items, sortField, sortDirection])

  const SortableHeader = ({ field, children }: { field: SortField; children: React.ReactNode }) => (
    <th
      className="px-6 py-4 text-left text-xs font-bold uppercase tracking-wider text-muted-foreground cursor-pointer hover:text-primary transition-colors select-none"
      onClick={() => handleSort(field)}
    >
      <div className="flex items-center gap-2">
        {children}
        <div className="flex flex-col">
          <svg
            className={`w-3 h-3 -mb-1 transition-colors ${sortField === field && sortDirection === 'asc' ? 'text-primary' : 'text-gray-400'}`}
            fill="currentColor"
            viewBox="0 0 20 20"
          >
            <path d="M5.293 9.707a1 1 0 010-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 01-1.414 1.414L10 6.414l-3.293 3.293a1 1 0 01-1.414 0z" />
          </svg>
          <svg
            className={`w-3 h-3 transition-colors ${sortField === field && sortDirection === 'desc' ? 'text-primary' : 'text-gray-400'}`}
            fill="currentColor"
            viewBox="0 0 20 20"
          >
            <path d="M14.707 10.293a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 111.414-1.414L10 13.586l3.293-3.293a1 1 0 011.414 0z" />
          </svg>
        </div>
      </div>
    </th>
  )

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-lg">Loading runtimes...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-lg text-red-600">
          Error: {error instanceof Error ? error.message : 'Failed to load runtimes'}
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen pb-12">
      {/* Header */}
      <header className="relative border-b border-border/50 bg-card/50 backdrop-blur-sm animate-in">
        <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          <div className="flex items-start justify-between gap-8">
            <div>
              <h1 className="text-4xl font-bold tracking-tight">Runtimes</h1>
              <p className="mt-2 text-muted-foreground max-w-2xl">
                Manage ClusterServingRuntime configurations for model deployment and inference
              </p>
            </div>
            <div className="flex gap-3 flex-shrink-0">
              <Link
                href="/runtimes/import"
                className="group relative rounded-lg border border-primary px-4 py-2.5 text-sm font-medium text-primary hover:bg-primary/5 transition-all overflow-hidden"
              >
                <span className="relative z-10 flex items-center gap-2">
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                  </svg>
                  Import
                </span>
              </Link>
              <Link
                href="/runtimes/new"
                className="gradient-border relative rounded-lg bg-gradient-to-r from-primary to-accent px-5 py-2.5 text-sm font-medium text-white hover:shadow-lg hover:shadow-primary/25 transition-all"
              >
                <span className="flex items-center gap-2">
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                  </svg>
                  Create Runtime
                </span>
              </Link>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        {/* Stats */}
        <div className="mb-8 grid grid-cols-1 gap-5 sm:grid-cols-3">
          {/* Total Runtimes Card */}
          <div className="group relative overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-md transition-all duration-300 animate-in">
            <div className="absolute inset-0 bg-gradient-to-br from-primary/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <div className="relative p-6">
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <dt className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    Total Runtimes
                  </dt>
                  <dd className="mt-3 text-4xl font-bold tracking-tight">
                    {data?.total || 0}
                  </dd>
                </div>
                <div className="rounded-lg bg-primary/10 p-3 group-hover:scale-110 transition-transform duration-300">
                  <svg className="h-6 w-6 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                  </svg>
                </div>
              </div>
            </div>
          </div>

          {/* Auto-Select Enabled Card */}
          <div className="group relative overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-md transition-all duration-300 animate-in-delay-1">
            <div className="absolute inset-0 bg-gradient-to-br from-accent/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <div className="relative p-6">
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <dt className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    Auto-Select
                  </dt>
                  <dd className="mt-3 text-4xl font-bold tracking-tight text-accent">
                    {data?.items.filter((r) => r.spec.supportedModelFormats?.some(f => f.autoSelect)).length || 0}
                  </dd>
                </div>
                <div className="rounded-lg bg-accent/10 p-3 group-hover:scale-110 transition-transform duration-300">
                  <svg className="h-6 w-6 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                  </svg>
                </div>
              </div>
            </div>
          </div>

          {/* Disabled Card */}
          <div className="group relative overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm hover:shadow-md transition-all duration-300 animate-in-delay-2">
            <div className="absolute inset-0 bg-gradient-to-br from-muted/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
            <div className="relative p-6">
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <dt className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    Disabled
                  </dt>
                  <dd className="mt-3 text-4xl font-bold tracking-tight text-muted-foreground">
                    {data?.items.filter((r) => r.spec.disabled).length || 0}
                  </dd>
                </div>
                <div className="rounded-lg bg-muted/20 p-3 group-hover:scale-110 transition-transform duration-300">
                  <svg className="h-6 w-6 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
                  </svg>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Runtimes Table */}
        <div className="overflow-hidden rounded-xl border border-border/50 bg-card/80 backdrop-blur-sm shadow-sm animate-in-delay-3">
          <div className="border-b border-border/50 px-6 py-5 bg-muted/30">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold tracking-tight">
                All Runtimes
              </h3>
              {/* Namespace Selector */}
              <div className="flex items-center gap-2">
                <label htmlFor="namespace" className="text-sm font-medium text-gray-700">
                  Scope:
                </label>
                <select
                  id="namespace"
                  value={selectedNamespace}
                  onChange={(e) => setSelectedNamespace(e.target.value)}
                  className="rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
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
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-border/50">
              <thead className="bg-muted/20">
                <tr>
                  <SortableHeader field="name">Name</SortableHeader>
                  <SortableHeader field="accelerators">Accelerators</SortableHeader>
                  <SortableHeader field="protocol">Protocol</SortableHeader>
                  <SortableHeader field="status">Status</SortableHeader>
                  <SortableHeader field="created">Created</SortableHeader>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/30 bg-card">
                {sortedRuntimes.map((runtime, index) => (
                  <tr
                    key={runtime.metadata.name}
                    className="group hover:bg-primary/5 transition-colors duration-200"
                    style={{ animationDelay: `${index * 50}ms` }}
                  >
                    <td className="whitespace-nowrap px-6 py-4">
                      <Link
                        href={`/runtimes/${runtime.metadata.name}`}
                        className="inline-flex items-center gap-2 text-sm font-semibold text-primary hover:text-accent transition-colors duration-200 group-hover:underline decoration-2 underline-offset-2"
                      >
                        <span>{runtime.metadata.name}</span>
                        <svg className="w-4 h-4 opacity-0 -translate-x-2 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-200" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                        </svg>
                      </Link>
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex flex-wrap gap-1">
                        {runtime.spec.acceleratorRequirements?.acceleratorClasses?.map((acc, idx) => (
                          <span key={idx} className="inline-flex items-center gap-1 rounded-md bg-muted/50 px-2 py-0.5 text-xs font-medium">
                            <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                            </svg>
                            {acc}
                          </span>
                        )) || <span className="text-xs text-muted-foreground">Any</span>}
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex flex-wrap gap-1">
                        {runtime.spec.protocolVersions?.map((protocol, idx) => (
                          <span key={idx} className="inline-flex items-center rounded-md bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
                            {protocol}
                          </span>
                        )) || <span className="text-xs text-muted-foreground">-</span>}
                      </div>
                    </td>
                    <td className="whitespace-nowrap px-6 py-4">
                      <span
                        className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold border ${
                          runtime.spec.disabled
                            ? 'bg-muted/30 text-muted-foreground border-border/50'
                            : 'bg-green-50 text-green-700 border-green-200'
                        }`}
                      >
                        <span className={`w-1.5 h-1.5 rounded-full ${
                          runtime.spec.disabled ? 'bg-muted-foreground' : 'bg-green-500'
                        }`} />
                        {runtime.spec.disabled ? 'Disabled' : 'Active'}
                      </span>
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-muted-foreground font-mono text-xs">
                      {runtime.metadata.creationTimestamp
                        ? new Date(runtime.metadata.creationTimestamp).toLocaleDateString()
                        : '-'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </main>
    </div>
  )
}
