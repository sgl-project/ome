import { z } from 'zod'

// Supported Model Format schema
export const supportedModelFormatSchema = z.object({
  name: z.string().min(1, 'Format name is required'),
  version: z.string().optional(),
  autoSelect: z.boolean().optional(),
  priority: z.number().optional(),
})

// Environment Variable schema
export const envVarSchema = z.object({
  name: z.string().min(1, 'Environment variable name is required'),
  value: z.string().optional(),
})

// Container Resource Requirements schema
export const resourceRequirementsSchema = z.object({
  requests: z.record(z.string()).optional(),
  limits: z.record(z.string()).optional(),
})

// Container schema
export const containerSchema = z.object({
  name: z.string().min(1, 'Container name is required'),
  image: z.string().min(1, 'Container image is required'),
  command: z.array(z.string()).optional(),
  args: z.array(z.string()).optional(),
  env: z.array(envVarSchema).optional(),
  resources: resourceRequirementsSchema.optional(),
})

// Built-in Adapter schema
export const builtInAdapterSchema = z.object({
  serverType: z.string().optional(),
  runtimeManagementPort: z.number().optional(),
  memBufferBytes: z.number().optional(),
  modelLoadingTimeoutMillis: z.number().optional(),
})

// Serving Runtime Spec schema
export const servingRuntimeSpecSchema = z.object({
  supportedModelFormats: z.array(supportedModelFormatSchema).min(1, 'At least one supported model format is required'),
  containers: z.array(containerSchema).optional(),
  builtInAdapter: builtInAdapterSchema.optional(),
  replicas: z.number().optional(),
  grpcEndpoint: z.string().optional(),
  grpcDataEndpoint: z.string().optional(),
  httpEndpoint: z.string().optional(),
  multiModel: z.boolean().optional(),
  disabled: z.boolean().optional(),
  protocolVersions: z.array(z.string()).optional(),
})

// Full Runtime schema
export const clusterServingRuntimeSchema = z.object({
  apiVersion: z.string().default('ome.io/v1beta1'),
  kind: z.string().default('ClusterServingRuntime'),
  metadata: z.object({
    name: z.string()
      .min(1, 'Name is required')
      .regex(/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/, 'Name must be lowercase alphanumeric with dashes'),
    namespace: z.string().optional(),
  }),
  spec: servingRuntimeSpecSchema,
})

export type ClusterServingRuntimeFormData = z.infer<typeof clusterServingRuntimeSchema>
export type SupportedModelFormatFormData = z.infer<typeof supportedModelFormatSchema>
export type ContainerFormData = z.infer<typeof containerSchema>
