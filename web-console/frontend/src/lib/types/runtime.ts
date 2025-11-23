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
  supportedModelFormats: SupportedModelFormat[]
  containers?: ContainerSpec[]
  builtInAdapter?: AdapterSpec
  replicas?: number
  grpcEndpoint?: string
  grpcDataEndpoint?: string
  httpEndpoint?: string
  multiModel?: boolean
  disabled?: boolean
  protocolVersions?: string[]
  [key: string]: any
}

export interface SupportedModelFormat {
  name: string
  version?: string
  autoSelect?: boolean
  priority?: number
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
