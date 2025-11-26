// InferenceService TypeScript types

// Import shared types from common
import { ObjectMeta, ResourceRequirements, Condition } from './common'

// Re-export for backwards compatibility
export type { ResourceRequirements, Condition } from './common'

export interface InferenceService {
  apiVersion: string
  kind: string
  metadata: ObjectMeta
  spec: InferenceServiceSpec
  status?: InferenceServiceStatus
}

export interface InferenceServiceSpec {
  predictor: PredictorSpec
  transformer?: TransformerSpec
  explainer?: ExplainerSpec
}

export interface PredictorSpec {
  model?: string
  runtime?: string
  replicas?: number
  minReplicas?: number
  maxReplicas?: number
  resources?: ResourceRequirements
  nodeSelector?: Record<string, string>
  tolerations?: { key?: string; operator?: string; value?: string; effect?: string }[]
  labels?: Record<string, string>
  annotations?: Record<string, string>
}

export interface TransformerSpec {
  containers?: { name: string; image: string; resources?: ResourceRequirements }[]
  minReplicas?: number
  maxReplicas?: number
}

export interface ExplainerSpec {
  type?: string
  containers?: { name: string; image: string; resources?: ResourceRequirements }[]
  minReplicas?: number
  maxReplicas?: number
}

export interface InferenceServiceStatus {
  state?: string
  url?: string
  address?: {
    url?: string
    internal?: string
    external?: string
  }
  conditions?: Condition[]
  modelStatus?: {
    states?: Record<string, { state: string; reason?: string; message?: string }>
    transitionStatus?: string
  }
  components?: Record<
    string,
    {
      url?: string
      address?: { url?: string }
      traffic?: number
      latestCreatedRevision?: string
      latestReadyRevision?: string
    }
  >
}
