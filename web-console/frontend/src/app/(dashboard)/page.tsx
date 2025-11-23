'use client'

import { useModels } from '@/lib/hooks/useModels'
import { useRuntimes } from '@/lib/hooks/useRuntimes'
import { useServices } from '@/lib/hooks/useServices'
import Link from 'next/link'

export default function DashboardPage() {
  const { data: modelsData, isLoading: modelsLoading } = useModels()
  const { data: runtimesData, isLoading: runtimesLoading } = useRuntimes()
  const { data: servicesData, isLoading: servicesLoading } = useServices()

  if (modelsLoading || runtimesLoading || servicesLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-lg">Loading...</div>
      </div>
    )
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
    <div className="min-h-screen">
      {/* Header */}
      <header className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <h1 className="text-3xl font-bold text-gray-900">Dashboard</h1>
          <p className="mt-1 text-sm text-gray-500">
            Overview of your OME resources
          </p>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        {/* Stats Grid */}
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {stats.map((stat) => (
            <Link
              key={stat.name}
              href={stat.href}
              className="overflow-hidden rounded-lg bg-white shadow transition-shadow hover:shadow-lg"
            >
              <div className="p-5">
                <div className="flex items-center">
                  <div className="flex-1">
                    <dt className="truncate text-sm font-medium text-gray-500">
                      {stat.name}
                    </dt>
                    <dd className={`mt-1 text-3xl font-semibold ${stat.textColor}`}>
                      {stat.value}
                    </dd>
                  </div>
                  <div className={`ml-4 h-12 w-12 rounded-lg ${stat.color} opacity-20`}></div>
                </div>
              </div>
            </Link>
          ))}
        </div>

        {/* Recent Activity */}
        <div className="mt-8">
          <div className="overflow-hidden rounded-lg bg-white shadow">
            <div className="border-b border-gray-200 px-4 py-5 sm:px-6">
              <h3 className="text-lg font-medium leading-6 text-gray-900">
                Recent Models
              </h3>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      Name
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      Vendor
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      Framework
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      Status
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200 bg-white">
                  {modelsData?.items.slice(0, 5).map((model) => (
                    <tr key={model.metadata.name} className="hover:bg-gray-50">
                      <td className="whitespace-nowrap px-6 py-4 text-sm font-medium text-gray-900">
                        <Link
                          href={`/models`}
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
                      <td className="whitespace-nowrap px-6 py-4">
                        <span
                          className={`inline-flex rounded-full px-2 text-xs font-semibold leading-5 ${
                            model.status?.state === 'Ready'
                              ? 'bg-green-100 text-green-800'
                              : model.status?.state === 'Failed'
                              ? 'bg-red-100 text-red-800'
                              : model.status?.state === 'In_Transit'
                              ? 'bg-yellow-100 text-yellow-800'
                              : 'bg-gray-100 text-gray-800'
                          }`}
                        >
                          {model.status?.state || 'Unknown'}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="bg-gray-50 px-4 py-3 text-right sm:px-6">
              <Link
                href="/models"
                className="text-sm font-medium text-blue-600 hover:text-blue-900"
              >
                View all models →
              </Link>
            </div>
          </div>
        </div>
      </main>
    </div>
  )
}
