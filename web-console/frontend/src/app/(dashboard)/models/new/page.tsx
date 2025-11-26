'use client'

import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  clusterBaseModelSchema,
  baseModelSchema,
  type ClusterBaseModelFormData,
  type BaseModelFormData,
} from '@/lib/validation/model-schema'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { useState, useEffect } from 'react'
import { modelsApi, baseModelsApi } from '@/lib/api/models'
import { useQueryClient } from '@tanstack/react-query'
import { useNamespaces } from '@/lib/hooks/useNamespaces'
import {
  HuggingFaceStorage,
  S3Storage,
  GCSStorage,
  OCIStorage,
  PVCStorage,
  AzureStorage,
  GitHubStorage,
} from '@/components/forms/storage'

type StorageType = 'oci' | 'pvc' | 'hf' | 's3' | 'az' | 'gs' | 'github' | 'local' | 'vendor'
type ModelScope = 'cluster' | 'namespace'

export default function CreateModelPage() {
  const router = useRouter()
  const queryClient = useQueryClient()
  const { data: namespacesData, isLoading: namespacesLoading } = useNamespaces()
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [modelScope, setModelScope] = useState<ModelScope>('cluster')
  const [namespace, setNamespace] = useState('default')
  const [storageType, setStorageType] = useState<StorageType>('hf')

  // Storage URI component state
  const [ociNamespace, setOciNamespace] = useState('')
  const [ociBucket, setOciBucket] = useState('')
  const [ociPrefix, setOciPrefix] = useState('')
  const [pvcName, setPvcName] = useState('')
  const [pvcSubPath, setPvcSubPath] = useState('')
  const [hfModelId, setHfModelId] = useState('')
  const [hfBranch, setHfBranch] = useState('main')
  const [huggingfaceToken, setHuggingfaceToken] = useState('')
  const [s3Bucket, setS3Bucket] = useState('')
  const [s3Prefix, setS3Prefix] = useState('')
  const [s3Region, setS3Region] = useState('')
  const [azAccount, setAzAccount] = useState('')
  const [azContainer, setAzContainer] = useState('')
  const [azBlobPath, setAzBlobPath] = useState('')
  const [gcsBucket, setGcsBucket] = useState('')
  const [gcsObject, setGcsObject] = useState('')
  const [githubOwner, setGithubOwner] = useState('')
  const [githubRepo, setGithubRepo] = useState('')
  const [githubTag, setGithubTag] = useState('latest')
  const [localPath, setLocalPath] = useState('')
  const [vendorName, setVendorName] = useState('')
  const [vendorResourceType, setVendorResourceType] = useState('')
  const [vendorResourcePath, setVendorResourcePath] = useState('')

  // Collapsible sections state
  const [showModelFormat, setShowModelFormat] = useState(false)
  const [showModelFramework, setShowModelFramework] = useState(false)
  const [showLabels, setShowLabels] = useState(false)
  const [showAnnotations, setShowAnnotations] = useState(false)

  // Labels and Annotations state (key-value pairs)
  const [labels, setLabels] = useState<Array<{ key: string; value: string }>>([])
  const [annotations, setAnnotations] = useState<Array<{ key: string; value: string }>>([])

  // Use appropriate schema based on scope
  const schema = modelScope === 'cluster' ? clusterBaseModelSchema : baseModelSchema

  const {
    register,
    handleSubmit,
    setValue,
    formState: { errors },
  } = useForm<ClusterBaseModelFormData | BaseModelFormData>({
    resolver: zodResolver(schema) as any,
    defaultValues: {
      apiVersion: 'ome.io/v1beta1',
      kind: modelScope === 'cluster' ? 'ClusterBaseModel' : 'BaseModel',
      metadata: {
        name: '',
        ...(modelScope === 'namespace' && { namespace }),
      },
      spec: {
        vendor: '',
        modelFormat: {
          name: '',
        },
        storage: {
          storageUri: '',
          path: '',
        },
      },
    },
  })

  // Set default namespace when namespaces load
  useEffect(() => {
    if (namespacesData?.items && namespacesData.items.length > 0) {
      // Set to 'default' if it exists, otherwise use the first namespace
      const defaultNs = namespacesData.items.includes('default')
        ? 'default'
        : namespacesData.items[0]
      setNamespace(defaultNs)
    }
  }, [namespacesData])

  // Update form when scope changes
  useEffect(() => {
    setValue('kind', modelScope === 'cluster' ? 'ClusterBaseModel' : 'BaseModel')
    if (modelScope === 'namespace') {
      setValue('metadata.namespace' as any, namespace)
    }
  }, [modelScope, namespace, setValue])

  // Build storage URI from components
  useEffect(() => {
    let uri = ''
    switch (storageType) {
      case 'oci':
        if (ociNamespace && ociBucket) {
          uri = `oci://n/${ociNamespace}/b/${ociBucket}/o/${ociPrefix || ''}`
        }
        break
      case 'pvc':
        if (pvcName && pvcSubPath) {
          uri = `pvc://${pvcName}/${pvcSubPath}`
        }
        break
      case 'hf':
        if (hfModelId) {
          uri = `hf://${hfModelId}${hfBranch !== 'main' ? '@' + hfBranch : ''}`
        }
        break
      case 's3':
        if (s3Bucket) {
          uri = `s3://${s3Bucket}${s3Region ? '@' + s3Region : ''}/${s3Prefix || ''}`
        }
        break
      case 'az':
        if (azAccount && azContainer) {
          uri = `az://${azAccount}/${azContainer}/${azBlobPath || ''}`
        }
        break
      case 'gs':
        if (gcsBucket) {
          uri = `gs://${gcsBucket}/${gcsObject || ''}`
        }
        break
      case 'github':
        if (githubOwner && githubRepo) {
          uri = `github://${githubOwner}/${githubRepo}${githubTag !== 'latest' ? '@' + githubTag : ''}`
        }
        break
      case 'local':
        if (localPath) {
          uri = `local://${localPath}`
        }
        break
      case 'vendor':
        if (vendorName && vendorResourceType && vendorResourcePath) {
          uri = `vendor://${vendorName}/${vendorResourceType}/${vendorResourcePath}`
        }
        break
    }
    setValue('spec.storage.storageUri', uri)
  }, [
    storageType,
    ociNamespace,
    ociBucket,
    ociPrefix,
    pvcName,
    pvcSubPath,
    hfModelId,
    hfBranch,
    s3Bucket,
    s3Prefix,
    s3Region,
    azAccount,
    azContainer,
    azBlobPath,
    gcsBucket,
    gcsObject,
    githubOwner,
    githubRepo,
    githubTag,
    localPath,
    vendorName,
    vendorResourceType,
    vendorResourcePath,
    setValue,
  ])

  const onSubmit = async (data: ClusterBaseModelFormData | BaseModelFormData) => {
    try {
      setError(null)
      setIsSubmitting(true)

      // Convert labels and annotations arrays to record format
      const labelsRecord: Record<string, string> = {}
      labels.forEach(({ key, value }) => {
        if (key && value) {
          labelsRecord[key] = value
        }
      })

      const annotationsRecord: Record<string, string> = {}
      annotations.forEach(({ key, value }) => {
        if (key && value) {
          annotationsRecord[key] = value
        }
      })

      // Add labels and annotations to metadata if they exist
      const submissionData = {
        ...data,
        metadata: {
          ...data.metadata,
          ...(Object.keys(labelsRecord).length > 0 && { labels: labelsRecord }),
          ...(Object.keys(annotationsRecord).length > 0 && { annotations: annotationsRecord }),
        },
      }

      if (modelScope === 'cluster') {
        await modelsApi.create({
          model: submissionData as ClusterBaseModelFormData,
          huggingfaceToken: storageType === 'hf' && huggingfaceToken ? huggingfaceToken : undefined,
        })
      } else {
        const baseModelData = submissionData as BaseModelFormData
        await baseModelsApi.create(baseModelData.metadata.namespace, {
          model: baseModelData,
          huggingfaceToken: storageType === 'hf' && huggingfaceToken ? huggingfaceToken : undefined,
        })
      }

      await queryClient.invalidateQueries({ queryKey: ['models'] })
      router.push('/models')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create model')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <Link
            href="/models"
            className="text-sm text-blue-600 hover:text-blue-800 mb-2 inline-block"
          >
            ← Back to Models
          </Link>
          <h1 className="text-3xl font-bold text-gray-900">Create New Model</h1>
          <p className="mt-1 text-sm text-gray-500">
            Define a new{' '}
            {modelScope === 'cluster'
              ? 'ClusterBaseModel (cluster-scoped)'
              : 'BaseModel (namespace-scoped)'}{' '}
            resource
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
          {/* Scope Selection */}
          <div className="rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">Model Scope</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Scope *</label>
                <div className="space-y-2">
                  <label className="flex items-center">
                    <input
                      type="radio"
                      value="cluster"
                      checked={modelScope === 'cluster'}
                      onChange={(e) => setModelScope(e.target.value as ModelScope)}
                      className="h-4 w-4 text-blue-600 focus:ring-blue-500"
                    />
                    <span className="ml-2 text-sm text-gray-700">
                      <span className="font-medium">Cluster-scoped</span> - Available to all
                      namespaces (ClusterBaseModel)
                    </span>
                  </label>
                  <label className="flex items-center">
                    <input
                      type="radio"
                      value="namespace"
                      checked={modelScope === 'namespace'}
                      onChange={(e) => setModelScope(e.target.value as ModelScope)}
                      className="h-4 w-4 text-blue-600 focus:ring-blue-500"
                    />
                    <span className="ml-2 text-sm text-gray-700">
                      <span className="font-medium">Namespace-scoped</span> - Only available in
                      specified namespace (BaseModel)
                    </span>
                  </label>
                </div>
              </div>

              {modelScope === 'namespace' && (
                <div>
                  <label htmlFor="namespace" className="block text-sm font-medium text-gray-700">
                    Namespace *
                  </label>
                  {namespacesLoading ? (
                    <div className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm bg-gray-50 text-gray-500">
                      Loading namespaces...
                    </div>
                  ) : (
                    <select
                      id="namespace"
                      value={namespace}
                      onChange={(e) => setNamespace(e.target.value)}
                      className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                    >
                      {namespacesData?.items.map((ns) => (
                        <option key={ns} value={ns}>
                          {ns}
                        </option>
                      ))}
                    </select>
                  )}
                  {(errors.metadata as any)?.namespace && (
                    <p className="mt-1 text-sm text-red-600">
                      {(errors.metadata as any).namespace.message}
                    </p>
                  )}
                  <p className="mt-1 text-xs text-gray-500">
                    Select an existing namespace. This does not create a new namespace.
                  </p>
                </div>
              )}
            </div>
          </div>

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
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                  placeholder="my-model"
                />
                {errors.metadata?.name && (
                  <p className="mt-1 text-sm text-red-600">{errors.metadata.name.message}</p>
                )}
              </div>

              <div>
                <label htmlFor="vendor" className="block text-sm font-medium text-gray-700">
                  Vendor *
                </label>
                <input
                  type="text"
                  id="vendor"
                  {...register('spec.vendor')}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                  placeholder="e.g., meta, openai, anthropic"
                />
                {errors.spec?.vendor && (
                  <p className="mt-1 text-sm text-red-600">{errors.spec.vendor.message}</p>
                )}
              </div>
            </div>

            <div className="mt-6">
              <label
                htmlFor="modelParameterSize"
                className="block text-sm font-medium text-gray-700"
              >
                Model Parameter Size
              </label>
              <input
                type="text"
                id="modelParameterSize"
                {...register('spec.modelParameterSize')}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                placeholder="e.g., 7B, 13B, 70B"
              />
            </div>
          </div>

          {/* Storage - REQUIRED */}
          <div className="rounded-lg bg-white p-6 shadow">
            <h2 className="mb-4 text-lg font-medium text-gray-900">
              Storage <span className="text-red-600">*</span>
            </h2>
            <p className="mb-4 text-sm text-gray-500">
              Specify the storage backend where the model files are located
            </p>

            <div className="space-y-4">
              {/* Storage Type Selector */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Storage Type *
                </label>
                <select
                  value={storageType}
                  onChange={(e) => setStorageType(e.target.value as StorageType)}
                  className="block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                >
                  <option value="hf">HuggingFace (hf://)</option>
                  <option value="oci">OCI Object Storage (oci://)</option>
                  <option value="pvc">Persistent Volume (pvc://)</option>
                  <option value="s3">AWS S3 (s3://)</option>
                  <option value="gs">Google Cloud Storage (gs://)</option>
                  <option value="az">Azure Blob (az://)</option>
                  <option value="github">GitHub Releases (github://)</option>
                  <option value="local">Local Filesystem (local://)</option>
                  <option value="vendor">Vendor Storage (vendor://)</option>
                </select>
              </div>

              {/* HuggingFace Fields */}
              {storageType === 'hf' && (
                <HuggingFaceStorage
                  modelId={hfModelId}
                  branch={hfBranch}
                  token={huggingfaceToken}
                  onModelIdChange={setHfModelId}
                  onBranchChange={setHfBranch}
                  onTokenChange={setHuggingfaceToken}
                />
              )}

              {/* OCI Object Storage Fields */}
              {storageType === 'oci' && (
                <OCIStorage
                  namespace={ociNamespace}
                  bucket={ociBucket}
                  prefix={ociPrefix}
                  onNamespaceChange={setOciNamespace}
                  onBucketChange={setOciBucket}
                  onPrefixChange={setOciPrefix}
                />
              )}

              {/* PVC Fields */}
              {storageType === 'pvc' && (
                <PVCStorage
                  name={pvcName}
                  subPath={pvcSubPath}
                  onNameChange={setPvcName}
                  onSubPathChange={setPvcSubPath}
                />
              )}

              {/* S3 Fields */}
              {storageType === 's3' && (
                <S3Storage
                  bucket={s3Bucket}
                  region={s3Region}
                  prefix={s3Prefix}
                  onBucketChange={setS3Bucket}
                  onRegionChange={setS3Region}
                  onPrefixChange={setS3Prefix}
                />
              )}

              {/* GCS Fields */}
              {storageType === 'gs' && (
                <GCSStorage
                  bucket={gcsBucket}
                  object={gcsObject}
                  onBucketChange={setGcsBucket}
                  onObjectChange={setGcsObject}
                />
              )}

              {/* Azure Fields */}
              {storageType === 'az' && (
                <AzureStorage
                  account={azAccount}
                  container={azContainer}
                  blobPath={azBlobPath}
                  onAccountChange={setAzAccount}
                  onContainerChange={setAzContainer}
                  onBlobPathChange={setAzBlobPath}
                />
              )}

              {/* GitHub Fields */}
              {storageType === 'github' && (
                <GitHubStorage
                  owner={githubOwner}
                  repo={githubRepo}
                  tag={githubTag}
                  onOwnerChange={setGithubOwner}
                  onRepoChange={setGithubRepo}
                  onTagChange={setGithubTag}
                />
              )}

              {/* Local Storage Fields */}
              {storageType === 'local' && (
                <div>
                  <label className="block text-sm font-medium text-gray-700">Local Path *</label>
                  <input
                    type="text"
                    value={localPath}
                    onChange={(e) => setLocalPath(e.target.value)}
                    className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                    placeholder="/data/models/my-model"
                  />
                  <p className="mt-1 text-xs text-gray-500">
                    Absolute path to model files on the host filesystem
                  </p>
                </div>
              )}

              {/* Vendor Storage Fields */}
              {storageType === 'vendor' && (
                <>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Vendor Name *</label>
                    <input
                      type="text"
                      value={vendorName}
                      onChange={(e) => setVendorName(e.target.value)}
                      className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                      placeholder="my-vendor"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">
                      Resource Type *
                    </label>
                    <input
                      type="text"
                      value={vendorResourceType}
                      onChange={(e) => setVendorResourceType(e.target.value)}
                      className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                      placeholder="models"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700">
                      Resource Path *
                    </label>
                    <input
                      type="text"
                      value={vendorResourcePath}
                      onChange={(e) => setVendorResourcePath(e.target.value)}
                      className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                      placeholder="my-model/v1"
                    />
                  </div>
                </>
              )}

              {/* Storage Path - REQUIRED */}
              <div>
                <label htmlFor="storagePath" className="block text-sm font-medium text-gray-700">
                  Storage Path *{' '}
                  <span className="text-gray-500 text-xs">
                    (e.g., /raid/models/microsoft/phi-4-gguf)
                  </span>
                </label>
                <input
                  type="text"
                  id="storagePath"
                  {...register('spec.storage.path')}
                  className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                  placeholder="/raid/models/microsoft/phi-4-gguf"
                />
                {errors.spec?.storage?.path && (
                  <p className="mt-1 text-sm text-red-600">{errors.spec.storage.path.message}</p>
                )}
                <p className="mt-1 text-xs text-gray-500">
                  Local filesystem path where the model will be stored or is located
                </p>
              </div>

              {/* Generated URI Display */}
              <div className="mt-4 rounded-lg bg-gray-50 p-4">
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Generated Storage URI
                </label>
                <input
                  type="text"
                  {...register('spec.storage.storageUri')}
                  readOnly
                  className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2 font-mono text-sm shadow-sm"
                />
                {errors.spec?.storage?.storageUri && (
                  <p className="mt-1 text-sm text-red-600">
                    {errors.spec.storage.storageUri.message}
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* Model Format - Optional, Collapsible */}
          <div className="rounded-lg bg-white shadow">
            <button
              type="button"
              onClick={() => setShowModelFormat(!showModelFormat)}
              className="w-full flex items-center justify-between p-6 text-left hover:bg-gray-50 transition-colors"
            >
              <div>
                <h2 className="text-lg font-medium text-gray-900">
                  Model Format <span className="text-sm text-gray-500 font-normal">(Optional)</span>
                </h2>
                <p className="mt-1 text-sm text-gray-500">Specify the model file format</p>
              </div>
              <svg
                className={`w-5 h-5 text-gray-500 transition-transform ${showModelFormat ? 'rotate-180' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M19 9l-7 7-7-7"
                />
              </svg>
            </button>
            {showModelFormat && (
              <div className="px-6 pb-6 border-t border-gray-200">
                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 mt-4">
                  <div>
                    <label htmlFor="formatName" className="block text-sm font-medium text-gray-700">
                      Format Name
                    </label>
                    <select
                      id="formatName"
                      {...register('spec.modelFormat.name')}
                      className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                    >
                      <option value="">Select format...</option>
                      <option value="safetensors">SafeTensors</option>
                      <option value="pytorch">PyTorch</option>
                      <option value="gguf">GGUF</option>
                      <option value="ggml">GGML</option>
                      <option value="onnx">ONNX</option>
                      <option value="tensorflow">TensorFlow</option>
                      <option value="huggingface">HuggingFace</option>
                    </select>
                    {errors.spec?.modelFormat?.name && (
                      <p className="mt-1 text-sm text-red-600">
                        {errors.spec.modelFormat.name.message}
                      </p>
                    )}
                  </div>

                  <div>
                    <label
                      htmlFor="formatVersion"
                      className="block text-sm font-medium text-gray-700"
                    >
                      Format Version
                    </label>
                    <input
                      type="text"
                      id="formatVersion"
                      {...register('spec.modelFormat.version')}
                      className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                      placeholder="e.g., 1.0"
                    />
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Model Framework - Optional, Collapsible */}
          <div className="rounded-lg bg-white shadow">
            <button
              type="button"
              onClick={() => setShowModelFramework(!showModelFramework)}
              className="w-full flex items-center justify-between p-6 text-left hover:bg-gray-50 transition-colors"
            >
              <div>
                <h2 className="text-lg font-medium text-gray-900">
                  Model Framework{' '}
                  <span className="text-sm text-gray-500 font-normal">(Optional)</span>
                </h2>
                <p className="mt-1 text-sm text-gray-500">Specify the ML framework used</p>
              </div>
              <svg
                className={`w-5 h-5 text-gray-500 transition-transform ${showModelFramework ? 'rotate-180' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M19 9l-7 7-7-7"
                />
              </svg>
            </button>
            {showModelFramework && (
              <div className="px-6 pb-6 border-t border-gray-200">
                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 mt-4">
                  <div>
                    <label
                      htmlFor="frameworkName"
                      className="block text-sm font-medium text-gray-700"
                    >
                      Framework Name
                    </label>
                    <select
                      id="frameworkName"
                      {...register('spec.modelFramework.name')}
                      className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                    >
                      <option value="">Select framework...</option>
                      <option value="transformers">Transformers</option>
                      <option value="pytorch">PyTorch</option>
                      <option value="tensorflow">TensorFlow</option>
                      <option value="jax">JAX</option>
                      <option value="onnx-runtime">ONNX Runtime</option>
                      <option value="llama-cpp">llama.cpp</option>
                    </select>
                    {errors.spec?.modelFramework?.name && (
                      <p className="mt-1 text-sm text-red-600">
                        {errors.spec.modelFramework.name.message}
                      </p>
                    )}
                  </div>

                  <div>
                    <label
                      htmlFor="frameworkVersion"
                      className="block text-sm font-medium text-gray-700"
                    >
                      Framework Version
                    </label>
                    <input
                      type="text"
                      id="frameworkVersion"
                      {...register('spec.modelFramework.version')}
                      className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                      placeholder="e.g., 2.0"
                    />
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Labels - Optional, Collapsible */}
          <div className="rounded-lg bg-white shadow">
            <button
              type="button"
              onClick={() => setShowLabels(!showLabels)}
              className="w-full flex items-center justify-between p-6 text-left hover:bg-gray-50 transition-colors"
            >
              <div>
                <h2 className="text-lg font-medium text-gray-900">
                  Labels <span className="text-sm text-gray-500 font-normal">(Optional)</span>
                </h2>
                <p className="mt-1 text-sm text-gray-500">
                  Key-value pairs for organizing and categorizing
                </p>
              </div>
              <svg
                className={`w-5 h-5 text-gray-500 transition-transform ${showLabels ? 'rotate-180' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M19 9l-7 7-7-7"
                />
              </svg>
            </button>
            {showLabels && (
              <div className="px-6 pb-6 border-t border-gray-200">
                <div className="space-y-4 mt-4">
                  {labels.map((label, index) => (
                    <div key={index} className="flex gap-2 items-start">
                      <div className="flex-1">
                        <input
                          type="text"
                          value={label.key}
                          onChange={(e) => {
                            const newLabels = [...labels]
                            newLabels[index].key = e.target.value
                            setLabels(newLabels)
                          }}
                          className="block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                          placeholder="Key (e.g., env)"
                        />
                      </div>
                      <div className="flex-1">
                        <input
                          type="text"
                          value={label.value}
                          onChange={(e) => {
                            const newLabels = [...labels]
                            newLabels[index].value = e.target.value
                            setLabels(newLabels)
                          }}
                          className="block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                          placeholder="Value (e.g., production)"
                        />
                      </div>
                      <button
                        type="button"
                        onClick={() => {
                          const newLabels = labels.filter((_, i) => i !== index)
                          setLabels(newLabels)
                        }}
                        className="px-3 py-2 text-red-600 hover:text-red-800 hover:bg-red-50 rounded-md transition-colors"
                      >
                        Remove
                      </button>
                    </div>
                  ))}
                  <button
                    type="button"
                    onClick={() => setLabels([...labels, { key: '', value: '' }])}
                    className="text-sm text-blue-600 hover:text-blue-800 font-medium"
                  >
                    + Add Label
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Annotations - Optional, Collapsible */}
          <div className="rounded-lg bg-white shadow">
            <button
              type="button"
              onClick={() => setShowAnnotations(!showAnnotations)}
              className="w-full flex items-center justify-between p-6 text-left hover:bg-gray-50 transition-colors"
            >
              <div>
                <h2 className="text-lg font-medium text-gray-900">
                  Annotations <span className="text-sm text-gray-500 font-normal">(Optional)</span>
                </h2>
                <p className="mt-1 text-sm text-gray-500">
                  Key-value pairs for storing additional metadata
                </p>
              </div>
              <svg
                className={`w-5 h-5 text-gray-500 transition-transform ${showAnnotations ? 'rotate-180' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M19 9l-7 7-7-7"
                />
              </svg>
            </button>
            {showAnnotations && (
              <div className="px-6 pb-6 border-t border-gray-200">
                <div className="space-y-4 mt-4">
                  {annotations.map((annotation, index) => (
                    <div key={index} className="flex gap-2 items-start">
                      <div className="flex-1">
                        <input
                          type="text"
                          value={annotation.key}
                          onChange={(e) => {
                            const newAnnotations = [...annotations]
                            newAnnotations[index].key = e.target.value
                            setAnnotations(newAnnotations)
                          }}
                          className="block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                          placeholder="Key (e.g., description)"
                        />
                      </div>
                      <div className="flex-1">
                        <input
                          type="text"
                          value={annotation.value}
                          onChange={(e) => {
                            const newAnnotations = [...annotations]
                            newAnnotations[index].value = e.target.value
                            setAnnotations(newAnnotations)
                          }}
                          className="block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
                          placeholder="Value (e.g., Production model)"
                        />
                      </div>
                      <button
                        type="button"
                        onClick={() => {
                          const newAnnotations = annotations.filter((_, i) => i !== index)
                          setAnnotations(newAnnotations)
                        }}
                        className="px-3 py-2 text-red-600 hover:text-red-800 hover:bg-red-50 rounded-md transition-colors"
                      >
                        Remove
                      </button>
                    </div>
                  ))}
                  <button
                    type="button"
                    onClick={() => setAnnotations([...annotations, { key: '', value: '' }])}
                    className="text-sm text-blue-600 hover:text-blue-800 font-medium"
                  >
                    + Add Annotation
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Submit Button */}
          <div className="flex justify-end gap-4">
            <Link
              href="/models"
              className="rounded-lg border border-gray-300 px-6 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={isSubmitting}
              className="rounded-lg bg-blue-600 px-6 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:bg-gray-400"
            >
              {isSubmitting ? 'Creating...' : 'Create Model'}
            </button>
          </div>
        </form>
      </main>
    </div>
  )
}
