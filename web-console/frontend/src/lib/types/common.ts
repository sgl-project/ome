// Common shared types for Kubernetes-style resources

export interface ObjectMeta {
  name: string
  namespace?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
  creationTimestamp?: string
  uid?: string
  resourceVersion?: string
}

export interface ResourceRequirements {
  limits?: Record<string, string>
  requests?: Record<string, string>
}

export interface EnvVar {
  name: string
  value?: string
  valueFrom?: EnvVarSource
}

export interface EnvVarSource {
  configMapKeyRef?: KeyRef
  secretKeyRef?: KeyRef
  fieldRef?: { fieldPath: string }
}

export interface KeyRef {
  name: string
  key: string
  optional?: boolean
}

export interface VolumeMount {
  name: string
  mountPath: string
  readOnly?: boolean
  subPath?: string
}

export interface Volume {
  name: string
  configMap?: { name: string; items?: { key: string; path: string }[] }
  secret?: { secretName: string; items?: { key: string; path: string }[] }
  emptyDir?: Record<string, unknown>
  persistentVolumeClaim?: { claimName: string; readOnly?: boolean }
  hostPath?: { path: string; type?: string }
}

export interface ContainerPort {
  name?: string
  containerPort: number
  protocol?: 'TCP' | 'UDP'
  hostPort?: number
}

export interface Container {
  name: string
  image: string
  command?: string[]
  args?: string[]
  env?: EnvVar[]
  resources?: ResourceRequirements
  volumeMounts?: VolumeMount[]
  ports?: ContainerPort[]
  workingDir?: string
  securityContext?: SecurityContext
}

export interface SecurityContext {
  runAsUser?: number
  runAsGroup?: number
  runAsNonRoot?: boolean
  readOnlyRootFilesystem?: boolean
  privileged?: boolean
  capabilities?: { add?: string[]; drop?: string[] }
}

export interface Condition {
  type: string
  status: 'True' | 'False' | 'Unknown'
  lastTransitionTime?: string
  reason?: string
  message?: string
}

export interface Toleration {
  key?: string
  operator?: 'Exists' | 'Equal'
  value?: string
  effect?: 'NoSchedule' | 'PreferNoSchedule' | 'NoExecute'
  tolerationSeconds?: number
}

export interface LocalObjectReference {
  name: string
}

export interface PodAffinity {
  nodeAffinity?: {
    requiredDuringSchedulingIgnoredDuringExecution?: { nodeSelectorTerms: NodeSelectorTerm[] }
    preferredDuringSchedulingIgnoredDuringExecution?: {
      weight: number
      preference: NodeSelectorTerm
    }[]
  }
  podAffinity?: PodAffinityTerm[]
  podAntiAffinity?: PodAffinityTerm[]
}

interface NodeSelectorTerm {
  matchExpressions?: { key: string; operator: string; values?: string[] }[]
  matchFields?: { key: string; operator: string; values?: string[] }[]
}

interface PodAffinityTerm {
  labelSelector?: {
    matchLabels?: Record<string, string>
    matchExpressions?: { key: string; operator: string; values?: string[] }[]
  }
  topologyKey: string
  namespaces?: string[]
}

export interface ListResponse<T> {
  items: T[]
  total: number
}

export type ResourceState =
  | 'Creating'
  | 'Ready'
  | 'Running'
  | 'Pending'
  | 'Failed'
  | 'Deleting'
  | 'Unknown'
  | 'Importing'
  | 'In_Transit'
  | 'In_Training'
