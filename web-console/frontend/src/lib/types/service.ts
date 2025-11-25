// InferenceService TypeScript types

// Import shared types from common
import { ResourceRequirements, Condition } from './common'

// Re-export for backwards compatibility
export type { ResourceRequirements, Condition } from './common'

export interface InferenceService {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace?: string
    creationTimestamp?: string
    [key: string]: unknown
  }
  spec: InferenceServiceSpec
  status?: InferenceServiceStatus
}

export interface InferenceServiceSpec {
  predictor: PredictorSpec
  [key: string]: unknown
}

export interface PredictorSpec {
  model?: string
  runtime?: string
  replicas?: number
  minReplicas?: number
  maxReplicas?: number
  resources?: ResourceRequirements
  [key: string]: unknown
}

export interface InferenceServiceStatus {
  state?: string
  url?: string
  conditions?: Condition[]
  [key: string]: unknown
}
