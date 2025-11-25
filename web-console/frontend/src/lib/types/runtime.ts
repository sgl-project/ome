// ClusterServingRuntime TypeScript types

export interface ClusterServingRuntime {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    creationTimestamp?: string
    [key: string]: any
  }
  spec: ServingRuntimeSpec
  status?: ServingRuntimeStatus
}

export interface ServingRuntimeSpec {
  supportedModelFormats?: SupportedModelFormat[]
  modelSizeRange?: ModelSizeRange
  disabled?: boolean
  routerConfig?: RouterConfig
  engineConfig?: EngineConfig
  decoderConfig?: DecoderConfig
  protocolVersions?: string[]
  // ServingRuntimePodSpec inline fields
  containers?: ContainerSpec[]
  volumes?: Volume[]
  nodeSelector?: Record<string, string>
  affinity?: any
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
  [key: string]: any
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
  [key: string]: any
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

export interface Volume {
  name: string
  [key: string]: any
}

export interface VolumeMount {
  name: string
  mountPath: string
  readOnly?: boolean
  [key: string]: any
}

export interface Container {
  name: string
  image: string
  command?: string[]
  args?: string[]
  env?: EnvVar[]
  resources?: ResourceRequirements
  volumeMounts?: VolumeMount[]
  [key: string]: any
}

export interface Toleration {
  key?: string
  operator?: string
  value?: string
  effect?: string
  [key: string]: any
}

export interface LocalObjectReference {
  name: string
}

export interface ContainerSpec {
  name: string
  image: string
  command?: string[]
  args?: string[]
  env?: EnvVar[]
  resources?: ResourceRequirements
  [key: string]: any
}

export interface AdapterSpec {
  serverType?: string
  runtimeManagementPort?: number
  memBufferBytes?: number
  modelLoadingTimeoutMillis?: number
  [key: string]: any
}

export interface EnvVar {
  name: string
  value?: string
  valueFrom?: any
}

export interface ResourceRequirements {
  limits?: Record<string, string>
  requests?: Record<string, string>
}

export interface ServingRuntimeStatus {
  [key: string]: any
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
