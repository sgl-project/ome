// InferenceService TypeScript types

export interface InferenceService {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace?: string
    creationTimestamp?: string
    [key: string]: any
  }
  spec: InferenceServiceSpec
  status?: InferenceServiceStatus
}

export interface InferenceServiceSpec {
  predictor: PredictorSpec
  [key: string]: any
}

export interface PredictorSpec {
  model?: string
  runtime?: string
  replicas?: number
  minReplicas?: number
  maxReplicas?: number
  resources?: ResourceRequirements
  [key: string]: any
}

export interface ResourceRequirements {
  limits?: Record<string, string>
  requests?: Record<string, string>
}

export interface InferenceServiceStatus {
  state?: string
  url?: string
  conditions?: Condition[]
  [key: string]: any
}

export interface Condition {
  type: string
  status: string
  lastTransitionTime?: string
  reason?: string
  message?: string
}
