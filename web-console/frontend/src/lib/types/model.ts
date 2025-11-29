// Import shared types from common
import { ObjectMeta, ResourceRequirements } from './common'

// Re-export for backwards compatibility
export type { ResourceRequirements } from './common'

export interface ModelFormat {
  name: string
  version?: string
  operator?: 'Equal' | 'GreaterThan' | 'GreaterThanOrEqual'
  weight?: number
}

export interface ModelFrameworkSpec {
  name: string
  version?: string
  operator?: 'Equal' | 'GreaterThan' | 'GreaterThanOrEqual'
  weight?: number
}

export interface StorageSpec {
  path?: string
  schemaPath?: string
  parameters?: Record<string, string>
  key?: string
  storageUri?: string
  nodeSelector?: Record<string, string>
}

export interface ModelConfigurationSpec {
  architecture?: string
  model_type?: string
  context_length?: number
  torch_dtype?: string
  transformers_version?: string
  has_vision?: boolean
}

export interface ModelExtensionSpec {
  displayName?: string
  version?: string
  disabled?: boolean
  vendor?: string
  compartmentID?: string
}

export interface BaseModelSpec {
  modelFormat: ModelFormat
  modelType?: string
  modelFramework?: ModelFrameworkSpec
  modelArchitecture?: string
  quantization?: string
  modelParameterSize?: string
  modelCapabilities?: string[]
  modelConfiguration?: ModelConfigurationSpec
  storage?: StorageSpec
  resources?: ResourceRequirements
  displayName?: string
  version?: string
  disabled?: boolean
  vendor?: string
  compartmentID?: string
  servingMode?: string[]
  maxTokens?: number
  additionalMetadata?: Record<string, string>
}

export interface ModelStatusSpec {
  lifecycle?: string
  state: 'Creating' | 'Importing' | 'In_Transit' | 'In_Training' | 'Ready' | 'Failed'
  nodesReady?: string[]
  nodesFailed?: string[]
}

export interface ClusterBaseModel {
  apiVersion: string
  kind: string
  metadata: ObjectMeta
  spec: BaseModelSpec
  status?: ModelStatusSpec
}

// BaseModel is namespace-scoped
export interface BaseModel {
  apiVersion: string
  kind: string
  metadata: ObjectMeta & { namespace: string }
  spec: BaseModelSpec
  status?: ModelStatusSpec
}

// Union type for both model types
export type Model = ClusterBaseModel | BaseModel

// Type guard to check if model is namespace-scoped
export function isNamespaceScoped(model: Model): model is BaseModel {
  return 'namespace' in model.metadata && model.metadata.namespace !== undefined
}

// HuggingFace types
export interface HuggingFaceModelSearchResult {
  id: string
  modelId: string
  author: string
  sha: string
  lastModified: string
  private: boolean
  gated: boolean
  disabled: boolean
  downloads: number
  likes: number
  tags: string[]
  pipeline_tag?: string
  library_name?: string
}

export interface HuggingFaceFileSibling {
  rfilename: string
  size?: number
}

export interface HuggingFaceModelInfo {
  id: string
  modelId: string
  author: string
  sha: string
  lastModified: string
  private: boolean
  gated: boolean
  disabled: boolean
  downloads: number
  likes: number
  tags: string[]
  pipeline_tag?: string
  library_name?: string
  siblings?: HuggingFaceFileSibling[]
  config?: HuggingFaceModelConfig
  cardData?: HuggingFaceCardData
}

export interface HuggingFaceCardData {
  license?: string
  language?: string | string[]
  tags?: string[]
  datasets?: string[]
  library_name?: string
  pipeline_tag?: string
  base_model?: string
}

export interface HuggingFaceModelConfig {
  architectures?: string[]
  model_type?: string
  task_specific_params?: Record<string, unknown>
  max_position_embeddings?: number
  vocab_size?: number
  hidden_size?: number
  num_hidden_layers?: number
  num_attention_heads?: number
  torch_dtype?: string
  quantization_config?: QuantizationConfig
}

export interface QuantizationConfig {
  quant_method?: string
  bits?: number
  group_size?: number
  desc_act?: boolean
  sym?: boolean
  checkpoint_format?: string
}

export interface HuggingFaceModelInfoResponse {
  model: HuggingFaceModelInfo
  detectedFormat?: string
  estimatedSize?: number
}

export interface HuggingFaceSearchParams {
  q?: string
  author?: string
  filter?: string
  sort?: 'downloads' | 'likes' | 'lastModified'
  direction?: 'asc' | 'desc'
  limit?: number
  tags?: string[]
}

// Model scope enum
export enum ModelScope {
  Cluster = 'cluster',
  Namespace = 'namespace',
}

// Create model form data
export interface CreateModelFormData {
  scope: ModelScope
  namespace?: string // Required when scope is Namespace
  name: string
  spec: BaseModelSpec
}

// Model event from K8s
export interface ModelEvent {
  type: 'Normal' | 'Warning'
  reason: string // e.g., DownloadProgress, FinalizerAdded
  message: string
  count: number
  firstTimestamp: string
  lastTimestamp: string
  source: string
}

// Parsed download progress from event message
export interface DownloadProgress {
  node: string
  percentage: number
  completedBytes: number
  totalBytes: number
  speedBytesPerSec: number
  eta?: string
}

// Helper to parse download progress from event message
export function parseDownloadProgress(message: string): DownloadProgress | null {
  // Format: "[10.0.119.255] Download progress: 42.3% (17.3 GB/40.9 GB) at 24.8 MB/s, ETA: 16m 13s"
  const regex =
    /\[([^\]]+)\] Download progress: ([\d.]+)% \(([\d.]+ [A-Z]+)\/([\d.]+ [A-Z]+)\) at ([\d.]+ [A-Z]+\/s)(?:, ETA: (.+))?/
  const match = message.match(regex)
  if (!match) return null

  const parseBytes = (str: string): number => {
    const [value, unit] = str.split(' ')
    const num = parseFloat(value)
    switch (unit) {
      case 'B':
        return num
      case 'KB':
        return num * 1024
      case 'MB':
        return num * 1024 * 1024
      case 'GB':
        return num * 1024 * 1024 * 1024
      case 'TB':
        return num * 1024 * 1024 * 1024 * 1024
      default:
        return num
    }
  }

  const parseSpeed = (str: string): number => {
    const speedStr = str.replace('/s', '')
    return parseBytes(speedStr)
  }

  return {
    node: match[1],
    percentage: parseFloat(match[2]),
    completedBytes: parseBytes(match[3]),
    totalBytes: parseBytes(match[4]),
    speedBytesPerSec: parseSpeed(match[5]),
    eta: match[6] || undefined,
  }
}
