'use client'

import { useModels } from '@/lib/hooks/useModels'
import { useNamespaces } from '@/lib/hooks/useNamespaces'
import { useServerEvents } from '@/hooks/useServerEvents'
import { useQueryClient } from '@tanstack/react-query'
import Link from 'next/link'
import { useState, useMemo } from 'react'
import { LoadingState } from '@/components/ui/LoadingState'
import { ErrorState } from '@/components/ui/ErrorState'
import { StatusBadge } from '@/components/ui/StatusBadge'

type SortField = 'name' | 'vendor' | 'framework' | 'size' | 'status' | 'created'
type SortDirection = 'asc' | 'desc'

export default function ModelsPage() {
  const [selectedNamespace, setSelectedNamespace] = useState<string>('')
  const { data, isLoading, error } = useModels(selectedNamespace || undefined)
  const { data: namespacesData } = useNamespaces()
  const queryClient = useQueryClient()
  const [sortField, setSortField] = useState<SortField>('name')
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc')

  // Connect to SSE for real-time updates
  useServerEvents({
    onEvent: (event) => {
      if (event.resource === 'models') {
        // Invalidate models query to trigger refetch
        queryClient.invalidateQueries({ queryKey: ['models'] })
      }
    },
    onConnected: () => {
      console.log('Connected to real-time updates')
    },
  })

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortDirection('asc')
    }
  }

  const sortedModels = useMemo(() => {
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
        case 'vendor':
          aValue = a.spec.vendor?.toLowerCase() || ''
          bValue = b.spec.vendor?.toLowerCase() || ''
          break
        case 'framework':
          aValue = a.spec.modelFramework?.name?.toLowerCase() || ''
          bValue = b.spec.modelFramework?.name?.toLowerCase() || ''
          break
        case 'size':
          aValue = a.spec.modelParameterSize || ''
          bValue = b.spec.modelParameterSize || ''
          break
        case 'status':
          aValue = a.status?.state || ''
          bValue = b.status?.state || ''
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
      className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 cursor-pointer hover:text-gray-700 transition-colors select-none"
      onClick={() => handleSort(field)}
    >
      <div className="flex items-center gap-2">
        {children}
        <div className="flex flex-col">
          <svg
            className={`w-3 h-3 -mb-1 transition-colors ${sortField === field && sortDirection === 'asc' ? 'text-blue-600' : 'text-gray-400'}`}
            fill="currentColor"
            viewBox="0 0 20 20"
          >
            <path d="M5.293 9.707a1 1 0 010-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 01-1.414 1.414L10 6.414l-3.293 3.293a1 1 0 01-1.414 0z" />
          </svg>
          <svg
            className={`w-3 h-3 transition-colors ${sortField === field && sortDirection === 'desc' ? 'text-blue-600' : 'text-gray-400'}`}
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
    return <LoadingState message="Loading models..." />
  }

  if (error) {
    return <ErrorState error={error || new Error('Failed to load models')} />
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-3xl font-bold text-gray-900">Models</h1>
              <p className="mt-1 text-sm text-gray-500">
                Manage your ClusterBaseModel and BaseModel resources
              </p>
            </div>
            <div className="flex gap-3">
              <Link
                href="/models/import"
                className="rounded-lg border border-blue-600 px-4 py-2 text-sm font-medium text-blue-600 hover:bg-blue-50"
              >
                Import from HuggingFace
              </Link>
              <Link
                href="/models/new"
                className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
              >
                Create Model
              </Link>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        {/* Stats */}
        <div className="mb-6 grid grid-cols-1 gap-6 sm:grid-cols-4">
          <div className="overflow-hidden rounded-lg bg-white shadow">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-gray-500">
                    Total Models
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-gray-900">
                    {data?.total || 0}
                  </dd>
                </div>
              </div>
            </div>
          </div>

          <div className="overflow-hidden rounded-lg bg-white shadow">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-gray-500">
                    Ready
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-green-600">
                    {data?.items.filter((m) => m.status?.state === 'Ready').length || 0}
                  </dd>
                </div>
              </div>
            </div>
          </div>

          <div className="overflow-hidden rounded-lg bg-white shadow">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-gray-500">
                    In Transit
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-yellow-600">
                    {data?.items.filter((m) => m.status?.state === 'In_Transit').length || 0}
                  </dd>
                </div>
              </div>
            </div>
          </div>

          <div className="overflow-hidden rounded-lg bg-white shadow">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-1">
                  <dt className="truncate text-sm font-medium text-gray-500">
                    Failed
                  </dt>
                  <dd className="mt-1 text-3xl font-semibold text-red-600">
                    {data?.items.filter((m) => m.status?.state === 'Failed').length || 0}
                  </dd>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Models Table */}
        <div className="overflow-hidden rounded-lg bg-white shadow">
          <div className="border-b border-gray-200 px-4 py-5 sm:px-6">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-medium leading-6 text-gray-900">
                All Models
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
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <SortableHeader field="name">Name</SortableHeader>
                  <SortableHeader field="vendor">Vendor</SortableHeader>
                  <SortableHeader field="framework">Framework</SortableHeader>
                  <SortableHeader field="size">Size</SortableHeader>
                  <SortableHeader field="status">Status</SortableHeader>
                  <SortableHeader field="created">Created</SortableHeader>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 bg-white">
                {sortedModels.map((model) => (
                  <tr key={model.metadata.name} className="hover:bg-gray-50">
                    <td className="whitespace-nowrap px-6 py-4 text-sm font-medium">
                      <Link
                        href={`/models/${model.metadata.name}`}
                        className="text-blue-600 hover:text-blue-900"
                      >
                        {model.metadata.name}
                      </Link>
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                      {model.spec.vendor || '-'}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                      {model.spec.modelFramework?.name || '-'}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                      {model.spec.modelParameterSize || '-'}
                    </td>
                    <td className="whitespace-nowrap px-6 py-4">
                      <StatusBadge state={model.status?.state} />
                    </td>
                    <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                      {model.metadata.creationTimestamp
                        ? new Date(model.metadata.creationTimestamp).toLocaleDateString()
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
