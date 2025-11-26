// ClusterServingRuntime TypeScript types

// Import shared types from common
import {
  ObjectMeta,
  ResourceRequirements,
  EnvVar,
  VolumeMount,
  Volume,
  Container,
  Toleration,
  LocalObjectReference,
  PodAffinity,
} from './common'

// Re-export for backwards compatibility
export type {
  ResourceRequirements,
  EnvVar,
  VolumeMount,
  Volume,
  Container,
  Toleration,
  LocalObjectReference,
} from './common'

export interface ClusterServingRuntime {
  apiVersion: string
  kind: string
  metadata: ObjectMeta
  spec: ServingRuntimeSpec
  status?: ServingRuntimeStatus
}

export interface ServingRuntimeSpec {
  supportedModelFormats?: SupportedModelFormat[]
  modelSizeRange?: ModelSizeRange
  disabled?: boolean
  multiModel?: boolean
  routerConfig?: RouterConfig
  engineConfig?: EngineConfig
  decoderConfig?: DecoderConfig
  protocolVersions?: string[]
  // ServingRuntimePodSpec inline fields
  containers?: ContainerSpec[]
  volumes?: Volume[]
  nodeSelector?: Record<string, string>
  affinity?: PodAffinity
  tolerations?: Toleration[]
  labels?: Record<string, string>
  annotations?: Record<string, string>
  imagePullSecrets?: LocalObjectReference[]
  schedulerName?: string
  hostIPC?: boolean
  dnsPolicy?: string
  hostNetwork?: boolean
  // WorkerPodSpec
  workers?: WorkerPodSpec
  // AcceleratorRequirements
  acceleratorRequirements?: AcceleratorRequirements
}

export interface SupportedModelFormat {
  name: string
  modelFormat?: ModelFormat
  modelType?: string
  version?: string
  modelFramework?: ModelFramework
  modelArchitecture?: string
  quantization?: string
  autoSelect?: boolean
  priority?: number
  acceleratorConfig?: Record<string, AcceleratorModelConfig>
}

export interface ModelFormat {
  name: string
  version?: string
  operator?: string
  weight?: number
}

export interface ModelFramework {
  name: string
  version?: string
  operator?: string
  weight?: number
}

export interface AcceleratorModelConfig {
  minMemoryPerBillionParams?: number
  tensorParallelismOverride?: TensorParallelismConfig
  runtimeArgsOverride?: string[]
  environmentOverride?: Record<string, string>
}

export interface TensorParallelismConfig {
  tensorParallelSize?: number
  pipelineParallelSize?: number
  dataParallelSize?: number
}

export interface ModelSizeRange {
  min?: string
  max?: string
}

export interface RouterConfig {
  runner?: RunnerSpec
  config?: Record<string, string>
  minReplicas?: number
  maxReplicas?: number
  scaleTarget?: number
  scaleMetric?: string
  volumes?: Volume[]
  initContainers?: Container[]
  sidecars?: Container[]
}

export interface EngineConfig {
  runner?: RunnerSpec
  leader?: LeaderSpec
  worker?: WorkerSpec
  minReplicas?: number
  maxReplicas?: number
  scaleTarget?: number
  scaleMetric?: string
  volumes?: Volume[]
  initContainers?: Container[]
  sidecars?: Container[]
  acceleratorOverride?: AcceleratorSelector
}

export interface DecoderConfig {
  runner?: RunnerSpec
  leader?: LeaderSpec
  worker?: WorkerSpec
  minReplicas?: number
  maxReplicas?: number
  scaleTarget?: number
  scaleMetric?: string
  volumes?: Volume[]
  initContainers?: Container[]
  sidecars?: Container[]
  acceleratorOverride?: AcceleratorSelector
}

export interface RunnerSpec {
  name?: string
  image?: string
  command?: string[]
  args?: string[]
  env?: EnvVar[]
  resources?: ResourceRequirements
  volumeMounts?: VolumeMount[]
  imagePullPolicy?: 'Always' | 'Never' | 'IfNotPresent'
  workingDir?: string
}

export interface LeaderSpec {
  runner?: RunnerSpec
  volumes?: Volume[]
  nodeSelector?: Record<string, string>
}

export interface WorkerSpec {
  size?: number
  runner?: RunnerSpec
  volumes?: Volume[]
  nodeSelector?: Record<string, string>
}

export interface WorkerPodSpec {
  size?: number
  containers?: ContainerSpec[]
  volumes?: Volume[]
  nodeSelector?: Record<string, string>
  labels?: Record<string, string>
  annotations?: Record<string, string>
}

export interface AcceleratorRequirements {
  acceleratorClasses?: string[]
  minMemory?: number
  minComputeCapability?: number
  requiredFeatures?: string[]
  preferredPrecisions?: string[]
}

export interface AcceleratorSelector {
  acceleratorClass?: string
  constraints?: AcceleratorConstraints
  policy?: string
}

export interface AcceleratorConstraints {
  minMemory?: number
  maxMemory?: number
  minComputeCapability?: number
  requiredFeatures?: string[]
  excludedClasses?: string[]
  architectureFamilies?: string[]
}

export interface ContainerSpec {
  name: string
  image: string
  command?: string[]
  args?: string[]
  env?: EnvVar[]
  resources?: ResourceRequirements
  volumeMounts?: VolumeMount[]
  ports?: { containerPort: number; protocol?: string; name?: string }[]
  imagePullPolicy?: 'Always' | 'Never' | 'IfNotPresent'
  workingDir?: string
}

export interface AdapterSpec {
  serverType?: string
  runtimeManagementPort?: number
  memBufferBytes?: number
  modelLoadingTimeoutMillis?: number
}

export interface ServingRuntimeStatus {
  replicas?: number
  readyReplicas?: number
  availableReplicas?: number
  conditions?: { type: string; status: string; reason?: string; message?: string }[]
}

// Runtime Intelligence Types

export interface RuntimeMatch {
  runtime: ClusterServingRuntime
  score: number
  compatibleWith: string[]
  reasons: string[]
  warnings?: string[]
  recommendation: string
}

export interface CompatibilityCheck {
  compatible: boolean
  reasons: string[]
  warnings?: string[]
  score: number
}

export interface RuntimeValidationResult {
  valid: boolean
  errors: string[]
  warnings: string[]
}
