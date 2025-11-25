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

export interface ResourceRequirements {
  requests?: Record<string, string>
  limits?: Record<string, string>
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
  metadata: {
    name: string
    creationTimestamp?: string
    [key: string]: any
  }
  spec: BaseModelSpec
  status?: ModelStatusSpec
}

// BaseModel is namespace-scoped
export interface BaseModel {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    namespace: string
    creationTimestamp?: string
    [key: string]: any
  }
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
  config?: Record<string, any>
  cardData?: Record<string, any>
}

export interface HuggingFaceModelConfig {
  architectures?: string[]
  model_type?: string
  task_specific_params?: Record<string, any>
  max_position_embeddings?: number
  vocab_size?: number
  hidden_size?: number
  num_hidden_layers?: number
  num_attention_heads?: number
  torch_dtype?: string
  quantization_config?: Record<string, any>
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
