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
import { CollapsibleSection } from '@/components/forms/CollapsibleSection'
import {
  sectionStyles,
  gridStyles,
  inputClassName,
  selectClassName,
  labelStyles,
  helpTextClassName,
  errorClassName,
  arrayFieldStyles,
} from '@/components/forms/styles'
import { MODEL_FORMAT_OPTIONS, MODEL_FRAMEWORK_OPTIONS } from '@/lib/constants/model-options'
import { Icons } from '@/components/ui/Icons'
import { exportAsYaml } from '@/lib/utils'

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

  // Collapsible sections state (only for initial default open state)
  const [showModelFormat] = useState(false)
  const [showModelFramework] = useState(false)

  // Labels and Annotations state (key-value pairs)
  const [labels, setLabels] = useState<Array<{ key: string; value: string }>>([])
  const [annotations, setAnnotations] = useState<Array<{ key: string; value: string }>>([])

  // Use appropriate schema based on scope
  const schema = modelScope === 'cluster' ? clusterBaseModelSchema : baseModelSchema

  const {
    register,
    handleSubmit,
    setValue,
    getValues,
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

  const handleExportYaml = () => {
    const data = getValues()
    const filename = data.metadata?.name || 'model'
    exportAsYaml(data, `${filename}.yaml`)
  }

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
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="border-b border-border bg-card shadow-sm">
        <div className="mx-auto max-w-4xl px-4 py-6 sm:px-6 lg:px-8">
          <Link
            href="/models"
            className="text-sm text-primary hover:text-primary/80 mb-2 inline-block"
          >
            ← Back to Models
          </Link>
          <h1 className="text-3xl font-bold text-foreground">Create New Model</h1>
          <p className="mt-1 text-sm text-muted-foreground">
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
          <div className="mb-6 rounded-lg bg-destructive/10 border border-destructive/20 p-4">
            <p className="text-sm text-destructive">{error}</p>
          </div>
        )}

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
          {/* Scope Selection */}
          <div className={sectionStyles.card}>
            <h2 className={sectionStyles.header}>Model Scope</h2>
            <div className="space-y-4">
              <div>
                <label className={labelStyles.base}>Scope *</label>
                <div className="space-y-2 mt-2">
                  <label className="flex items-center">
                    <input
                      type="radio"
                      value="cluster"
                      checked={modelScope === 'cluster'}
                      onChange={(e) => setModelScope(e.target.value as ModelScope)}
                      className="h-4 w-4 text-primary focus:ring-primary"
                    />
                    <span className="ml-2 text-sm text-foreground">
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
                      className="h-4 w-4 text-primary focus:ring-primary"
                    />
                    <span className="ml-2 text-sm text-foreground">
                      <span className="font-medium">Namespace-scoped</span> - Only available in
                      specified namespace (BaseModel)
                    </span>
                  </label>
                </div>
              </div>

              {modelScope === 'namespace' && (
                <div>
                  <label htmlFor="namespace" className={labelStyles.base}>
                    Namespace *
                  </label>
                  {namespacesLoading ? (
                    <div className={`${inputClassName} bg-muted text-muted-foreground`}>
                      Loading namespaces...
                    </div>
                  ) : (
                    <select
                      id="namespace"
                      value={namespace}
                      onChange={(e) => setNamespace(e.target.value)}
                      className={selectClassName}
                    >
                      {namespacesData?.items.map((ns) => (
                        <option key={ns} value={ns}>
                          {ns}
                        </option>
                      ))}
                    </select>
                  )}
                  {(errors.metadata as any)?.namespace && (
                    <p className={errorClassName}>{(errors.metadata as any).namespace.message}</p>
                  )}
                  <p className={helpTextClassName}>
                    Select an existing namespace. This does not create a new namespace.
                  </p>
                </div>
              )}
            </div>
          </div>

          {/* Basic Information */}
          <div className={sectionStyles.card}>
            <h2 className={sectionStyles.header}>Basic Information</h2>
            <div className={gridStyles.cols2}>
              <div>
                <label htmlFor="name" className={labelStyles.base}>
                  Name *
                </label>
                <input
                  type="text"
                  id="name"
                  {...register('metadata.name')}
                  className={inputClassName}
                  placeholder="my-model"
                />
                {errors.metadata?.name && (
                  <p className={errorClassName}>{errors.metadata.name.message}</p>
                )}
              </div>

              <div>
                <label htmlFor="vendor" className={labelStyles.base}>
                  Vendor *
                </label>
                <input
                  type="text"
                  id="vendor"
                  {...register('spec.vendor')}
                  className={inputClassName}
                  placeholder="e.g., meta, openai, anthropic"
                />
                {errors.spec?.vendor && (
                  <p className={errorClassName}>{errors.spec.vendor.message}</p>
                )}
              </div>
            </div>

            <div className="mt-6">
              <label htmlFor="modelParameterSize" className={labelStyles.base}>
                Model Parameter Size
              </label>
              <input
                type="text"
                id="modelParameterSize"
                {...register('spec.modelParameterSize')}
                className={inputClassName}
                placeholder="e.g., 7B, 13B, 70B"
              />
            </div>
          </div>

          {/* Storage - REQUIRED */}
          <div className={sectionStyles.card}>
            <h2 className={sectionStyles.header}>
              Storage <span className="text-destructive">*</span>
            </h2>
            <p className={helpTextClassName}>
              Specify the storage backend where the model files are located
            </p>

            <div className="space-y-4 mt-4">
              {/* Storage Type Selector */}
              <div>
                <label className={labelStyles.base}>Storage Type *</label>
                <select
                  value={storageType}
                  onChange={(e) => setStorageType(e.target.value as StorageType)}
                  className={selectClassName}
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
                  <label className={labelStyles.base}>Local Path *</label>
                  <input
                    type="text"
                    value={localPath}
                    onChange={(e) => setLocalPath(e.target.value)}
                    className={inputClassName}
                    placeholder="/data/models/my-model"
                  />
                  <p className={helpTextClassName}>
                    Absolute path to model files on the host filesystem
                  </p>
                </div>
              )}

              {/* Vendor Storage Fields */}
              {storageType === 'vendor' && (
                <>
                  <div>
                    <label className={labelStyles.base}>Vendor Name *</label>
                    <input
                      type="text"
                      value={vendorName}
                      onChange={(e) => setVendorName(e.target.value)}
                      className={inputClassName}
                      placeholder="my-vendor"
                    />
                  </div>
                  <div>
                    <label className={labelStyles.base}>Resource Type *</label>
                    <input
                      type="text"
                      value={vendorResourceType}
                      onChange={(e) => setVendorResourceType(e.target.value)}
                      className={inputClassName}
                      placeholder="models"
                    />
                  </div>
                  <div>
                    <label className={labelStyles.base}>Resource Path *</label>
                    <input
                      type="text"
                      value={vendorResourcePath}
                      onChange={(e) => setVendorResourcePath(e.target.value)}
                      className={inputClassName}
                      placeholder="my-model/v1"
                    />
                  </div>
                </>
              )}

              {/* Storage Path - REQUIRED */}
              <div>
                <label htmlFor="storagePath" className={labelStyles.base}>
                  Storage Path *{' '}
                  <span className="text-muted-foreground text-xs">
                    (e.g., /raid/models/microsoft/phi-4-gguf)
                  </span>
                </label>
                <input
                  type="text"
                  id="storagePath"
                  {...register('spec.storage.path')}
                  className={inputClassName}
                  placeholder="/raid/models/microsoft/phi-4-gguf"
                />
                {errors.spec?.storage?.path && (
                  <p className={errorClassName}>{errors.spec.storage.path.message}</p>
                )}
                <p className={helpTextClassName}>
                  Local filesystem path where the model will be stored or is located
                </p>
              </div>

              {/* Generated URI Display */}
              <div className="mt-4 rounded-lg bg-muted p-4">
                <label className={labelStyles.base}>Generated Storage URI</label>
                <input
                  type="text"
                  {...register('spec.storage.storageUri')}
                  readOnly
                  className={`${inputClassName} bg-card font-mono text-sm cursor-not-allowed`}
                />
                {errors.spec?.storage?.storageUri && (
                  <p className={errorClassName}>{errors.spec.storage.storageUri.message}</p>
                )}
              </div>
            </div>
          </div>

          {/* Model Format - Optional, Collapsible */}
          <CollapsibleSection
            title="Model Format"
            description="Specify the model file format"
            defaultOpen={showModelFormat}
          >
            <div className={gridStyles.cols2}>
              <div>
                <label htmlFor="formatName" className={labelStyles.base}>
                  Format Name
                </label>
                <select
                  id="formatName"
                  {...register('spec.modelFormat.name')}
                  className={selectClassName}
                >
                  {MODEL_FORMAT_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
                {errors.spec?.modelFormat?.name && (
                  <p className={errorClassName}>{errors.spec.modelFormat.name.message}</p>
                )}
              </div>

              <div>
                <label htmlFor="formatVersion" className={labelStyles.base}>
                  Format Version
                </label>
                <input
                  type="text"
                  id="formatVersion"
                  {...register('spec.modelFormat.version')}
                  className={inputClassName}
                  placeholder="e.g., 1.0"
                />
              </div>
            </div>
          </CollapsibleSection>

          {/* Model Framework - Optional, Collapsible */}
          <CollapsibleSection
            title="Model Framework"
            description="Specify the ML framework used"
            defaultOpen={showModelFramework}
          >
            <div className={gridStyles.cols2}>
              <div>
                <label htmlFor="frameworkName" className={labelStyles.base}>
                  Framework Name
                </label>
                <select
                  id="frameworkName"
                  {...register('spec.modelFramework.name')}
                  className={selectClassName}
                >
                  {MODEL_FRAMEWORK_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
                {errors.spec?.modelFramework?.name && (
                  <p className={errorClassName}>{errors.spec.modelFramework.name.message}</p>
                )}
              </div>

              <div>
                <label htmlFor="frameworkVersion" className={labelStyles.base}>
                  Framework Version
                </label>
                <input
                  type="text"
                  id="frameworkVersion"
                  {...register('spec.modelFramework.version')}
                  className={inputClassName}
                  placeholder="e.g., 2.0"
                />
              </div>
            </div>
          </CollapsibleSection>

          {/* Labels - Optional, Collapsible */}
          <CollapsibleSection
            title="Labels"
            description="Key-value pairs for organizing and categorizing"
            defaultOpen={labels.length > 0}
            badge={
              labels.length > 0 ? (
                <span className="text-xs bg-muted px-2 py-0.5 rounded-full">{labels.length}</span>
              ) : undefined
            }
          >
            <div className="space-y-3">
              {labels.map((label, index) => (
                <div key={index} className="flex items-center gap-2">
                  <input
                    type="text"
                    value={label.key}
                    onChange={(e) => {
                      const newLabels = [...labels]
                      newLabels[index].key = e.target.value
                      setLabels(newLabels)
                    }}
                    className={inputClassName}
                    placeholder="Key"
                  />
                  <input
                    type="text"
                    value={label.value}
                    onChange={(e) => {
                      const newLabels = [...labels]
                      newLabels[index].value = e.target.value
                      setLabels(newLabels)
                    }}
                    className={inputClassName}
                    placeholder="Value"
                  />
                  <button
                    type="button"
                    onClick={() => setLabels(labels.filter((_, i) => i !== index))}
                    className={arrayFieldStyles.removeButton}
                  >
                    Remove
                  </button>
                </div>
              ))}
              <button
                type="button"
                onClick={() => setLabels([...labels, { key: '', value: '' }])}
                className={arrayFieldStyles.addButton}
              >
                + Add Label
              </button>
            </div>
          </CollapsibleSection>

          {/* Annotations - Optional, Collapsible */}
          <CollapsibleSection
            title="Annotations"
            description="Key-value pairs for storing additional metadata"
            defaultOpen={annotations.length > 0}
            badge={
              annotations.length > 0 ? (
                <span className="text-xs bg-muted px-2 py-0.5 rounded-full">
                  {annotations.length}
                </span>
              ) : undefined
            }
          >
            <div className="space-y-3">
              {annotations.map((annotation, index) => (
                <div key={index} className="flex items-center gap-2">
                  <input
                    type="text"
                    value={annotation.key}
                    onChange={(e) => {
                      const newAnnotations = [...annotations]
                      newAnnotations[index].key = e.target.value
                      setAnnotations(newAnnotations)
                    }}
                    className={inputClassName}
                    placeholder="Key"
                  />
                  <input
                    type="text"
                    value={annotation.value}
                    onChange={(e) => {
                      const newAnnotations = [...annotations]
                      newAnnotations[index].value = e.target.value
                      setAnnotations(newAnnotations)
                    }}
                    className={inputClassName}
                    placeholder="Value"
                  />
                  <button
                    type="button"
                    onClick={() => setAnnotations(annotations.filter((_, i) => i !== index))}
                    className={arrayFieldStyles.removeButton}
                  >
                    Remove
                  </button>
                </div>
              ))}
              <button
                type="button"
                onClick={() => setAnnotations([...annotations, { key: '', value: '' }])}
                className={arrayFieldStyles.addButton}
              >
                + Add Annotation
              </button>
            </div>
          </CollapsibleSection>

          {/* Action Buttons */}
          <div className="flex items-center justify-between pt-4">
            {/* Export Button - Left side */}
            <button
              type="button"
              onClick={handleExportYaml}
              className="inline-flex items-center gap-2 rounded-lg border border-border bg-card px-4 py-2.5 text-sm font-medium text-foreground hover:bg-muted transition-colors"
            >
              <Icons.downloadFile size="sm" />
              Export YAML
            </button>

            {/* Cancel and Submit - Right side */}
            <div className="flex gap-4">
              <Link
                href="/models"
                className="rounded-lg border border-border bg-card px-6 py-2.5 text-sm font-medium text-foreground hover:bg-muted transition-colors"
              >
                Cancel
              </Link>
              <button
                type="submit"
                disabled={isSubmitting}
                className="rounded-lg bg-primary px-6 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-colors"
              >
                {isSubmitting ? 'Creating...' : 'Create Model'}
              </button>
            </div>
          </div>
        </form>
      </main>
    </div>
  )
}
