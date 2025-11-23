'use client'

import { useForm, useFieldArray } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { clusterServingRuntimeSchema, type ClusterServingRuntimeFormData } from '@/lib/validation/runtime-schema'
import { useCreateRuntime } from '@/lib/hooks/useRuntimes'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { useState } from 'react'

export default function CreateRuntimePage() {
  const router = useRouter()
  const createRuntime = useCreateRuntime()
  const [error, setError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    control,
    formState: { errors, isSubmitting },
  } = useForm<ClusterServingRuntimeFormData>({
    resolver: zodResolver(clusterServingRuntimeSchema),
    defaultValues: {
      apiVersion: 'ome.io/v1beta1',
      kind: 'ClusterServingRuntime',
      metadata: {
        name: '',
      },
      spec: {
        supportedModelFormats: [{ name: '' }],
        multiModel: false,
        disabled: false,
      },
    },
  })

  const { fields: formatFields, append: appendFormat, remove: removeFormat } = useFieldArray({
    control,
    name: 'spec.supportedModelFormats',
  })

  const onSubmit = async (data: ClusterServingRuntimeFormData) => {
    try {
      setError(null)
      await createRuntime.mutateAsync(data)
      router.push('/runtimes')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create runtime')
    }
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <Link href="/runtimes" className="text-sm text-purple-600 hover:text-purple-800 mb-2 inline-block">
            ← Back to Runtimes
          </Link>
          <h1 className="text-3xl font-bold text-gray-900">Create New Runtime</h1>
          <p className="mt-1 text-sm text-gray-500">
            Define a new ClusterServingRuntime resource
          </p>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-4xl px-4 py-6 sm:px-6 lg:px-8">
        {error && (
          <div className="mb-6 rounded-lg bg-red-50 p-4">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
          {/* Basic Information */}
          <div className="rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Basic Information</h2>
            <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
              <div>
                <label htmlFor="name" className="block text-sm font-medium text-gray-700">
                  Name *
                </label>
                <input
                  type="text"
                  id="name"
                  {...register('metadata.name')}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-purple-500"
                  placeholder="my-runtime"
                />
                {errors.metadata?.name && (
                  <p className="mt-1 text-sm text-red-600">{errors.metadata.name.message}</p>
                )}
              </div>

              <div>
                <label htmlFor="namespace" className="block text-sm font-medium text-gray-700">
                  Namespace
                </label>
                <input
                  type="text"
                  id="namespace"
                  {...register('metadata.namespace')}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-purple-500"
                  placeholder="default"
                />
              </div>

              <div>
                <label htmlFor="multiModel" className="flex items-center">
                  <input
                    type="checkbox"
                    id="multiModel"
                    {...register('spec.multiModel')}
                    className="h-4 w-4 rounded border-gray-300 text-purple-600 focus:ring-purple-500"
                  />
                  <span className="ml-2 text-sm text-gray-700">Multi-Model Support</span>
                </label>
              </div>

              <div>
                <label htmlFor="disabled" className="flex items-center">
                  <input
                    type="checkbox"
                    id="disabled"
                    {...register('spec.disabled')}
                    className="h-4 w-4 rounded border-gray-300 text-gray-600 focus:ring-gray-500"
                  />
                  <span className="ml-2 text-sm text-gray-700">Disabled</span>
                </label>
              </div>
            </div>
          </div>

          {/* Supported Model Formats */}
          <div className="rounded-lg bg-white p-6 shadow">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-medium text-gray-900">Supported Model Formats</h2>
              <button
                type="button"
                onClick={() => appendFormat({ name: '', version: '' })}
                className="rounded-md bg-purple-600 px-3 py-1 text-sm text-white hover:bg-purple-700"
              >
                Add Format
              </button>
            </div>

            <div className="space-y-4">
              {formatFields.map((field, index) => (
                <div key={field.id} className="rounded-lg border border-gray-200 p-4">
                  <div className="mb-2 flex items-center justify-between">
                    <h3 className="text-sm font-medium text-gray-700">Format {index + 1}</h3>
                    {formatFields.length > 1 && (
                      <button
                        type="button"
                        onClick={() => removeFormat(index)}
                        className="text-sm text-red-600 hover:text-red-800"
                      >
                        Remove
                      </button>
                    )}
                  </div>

                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                    <div>
                      <label className="block text-sm font-medium text-gray-700">
                        Format Name *
                      </label>
                      <select
                        {...register(`spec.supportedModelFormats.${index}.name` as const)}
                        className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-purple-500"
                      >
                        <option value="">Select format...</option>
                        <option value="safetensors">SafeTensors</option>
                        <option value="pytorch">PyTorch</option>
                        <option value="onnx">ONNX</option>
                        <option value="tensorflow">TensorFlow</option>
                        <option value="huggingface">HuggingFace</option>
                      </select>
                      {errors.spec?.supportedModelFormats?.[index]?.name && (
                        <p className="mt-1 text-sm text-red-600">
                          {errors.spec.supportedModelFormats[index]?.name?.message}
                        </p>
                      )}
                    </div>

                    <div>
                      <label className="block text-sm font-medium text-gray-700">
                        Version
                      </label>
                      <input
                        type="text"
                        {...register(`spec.supportedModelFormats.${index}.version` as const)}
                        className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-purple-500"
                        placeholder="1.0"
                      />
                    </div>

                    <div>
                      <label className="flex items-center">
                        <input
                          type="checkbox"
                          {...register(`spec.supportedModelFormats.${index}.autoSelect` as const)}
                          className="h-4 w-4 rounded border-gray-300 text-purple-600 focus:ring-purple-500"
                        />
                        <span className="ml-2 text-sm text-gray-700">Auto Select</span>
                      </label>
                    </div>

                    <div>
                      <label className="block text-sm font-medium text-gray-700">
                        Priority
                      </label>
                      <input
                        type="number"
                        {...register(`spec.supportedModelFormats.${index}.priority` as const, {
                          valueAsNumber: true,
                        })}
                        className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-purple-500"
                        placeholder="0"
                      />
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Configuration */}
          <div className="rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Configuration (Optional)</h2>
            <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
              <div>
                <label htmlFor="replicas" className="block text-sm font-medium text-gray-700">
                  Replicas
                </label>
                <input
                  type="number"
                  id="replicas"
                  {...register('spec.replicas', { valueAsNumber: true })}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-purple-500"
                  placeholder="1"
                />
              </div>

              <div>
                <label htmlFor="grpcEndpoint" className="block text-sm font-medium text-gray-700">
                  gRPC Endpoint
                </label>
                <input
                  type="text"
                  id="grpcEndpoint"
                  {...register('spec.grpcEndpoint')}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-purple-500"
                  placeholder="localhost:8081"
                />
              </div>

              <div>
                <label htmlFor="httpEndpoint" className="block text-sm font-medium text-gray-700">
                  HTTP Endpoint
                </label>
                <input
                  type="text"
                  id="httpEndpoint"
                  {...register('spec.httpEndpoint')}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-purple-500 focus:outline-none focus:ring-purple-500"
                  placeholder="localhost:8080"
                />
              </div>
            </div>
          </div>

          {/* Form Actions */}
          <div className="flex justify-end gap-3">
            <Link
              href="/runtimes"
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={isSubmitting}
              className="rounded-lg bg-purple-600 px-4 py-2 text-sm font-medium text-white hover:bg-purple-700 disabled:bg-purple-400"
            >
              {isSubmitting ? 'Creating...' : 'Create Runtime'}
            </button>
          </div>
        </form>
      </main>
    </div>
  )
}
