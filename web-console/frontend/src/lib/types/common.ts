// Common shared types for Kubernetes-style resources
// This file consolidates duplicate type definitions from model.ts, service.ts, and runtime.ts

/**
 * Standard Kubernetes ObjectMeta
 */
export interface ObjectMeta {
  name: string
  namespace?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
  creationTimestamp?: string
  [key: string]: unknown
}

/**
 * Kubernetes-style resource requirements for CPU/memory/GPU
 */
export interface ResourceRequirements {
  limits?: Record<string, string>
  requests?: Record<string, string>
}

/**
 * Environment variable definition
 */
export interface EnvVar {
  name: string
  value?: string
  valueFrom?: EnvVarSource
}

/**
 * Source for environment variable value
 */
export interface EnvVarSource {
  configMapKeyRef?: KeyRef
  secretKeyRef?: KeyRef
  fieldRef?: { fieldPath: string }
}

/**
 * Reference to a key in a ConfigMap or Secret
 */
export interface KeyRef {
  name: string
  key: string
  optional?: boolean
}

/**
 * Volume mount specification
 */
export interface VolumeMount {
  name: string
  mountPath: string
  readOnly?: boolean
  subPath?: string
  [key: string]: unknown
}

/**
 * Volume specification
 */
export interface Volume {
  name: string
  configMap?: { name: string; items?: { key: string; path: string }[] }
  secret?: { secretName: string; items?: { key: string; path: string }[] }
  emptyDir?: Record<string, unknown>
  persistentVolumeClaim?: { claimName: string; readOnly?: boolean }
  hostPath?: { path: string; type?: string }
  [key: string]: unknown
}

/**
 * Container port specification
 */
export interface ContainerPort {
  name?: string
  containerPort: number
  protocol?: 'TCP' | 'UDP'
  hostPort?: number
}

/**
 * Container specification
 */
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
  [key: string]: unknown
}

/**
 * Security context for containers
 */
export interface SecurityContext {
  runAsUser?: number
  runAsGroup?: number
  runAsNonRoot?: boolean
  readOnlyRootFilesystem?: boolean
  privileged?: boolean
  capabilities?: {
    add?: string[]
    drop?: string[]
  }
}

/**
 * Kubernetes condition type (used in status)
 */
export interface Condition {
  type: string
  status: 'True' | 'False' | 'Unknown'
  lastTransitionTime?: string
  reason?: string
  message?: string
}

/**
 * Pod toleration
 */
export interface Toleration {
  key?: string
  operator?: 'Exists' | 'Equal'
  value?: string
  effect?: 'NoSchedule' | 'PreferNoSchedule' | 'NoExecute'
  tolerationSeconds?: number
  [key: string]: unknown
}

/**
 * Local object reference (e.g., for imagePullSecrets)
 */
export interface LocalObjectReference {
  name: string
}

/**
 * API response wrapper for list operations
 */
export interface ListResponse<T> {
  items: T[]
  total: number
}

/**
 * Common resource states across different resource types
 */
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
