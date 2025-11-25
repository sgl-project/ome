import { z } from 'zod'

// Environment Variable schema
export const envVarSchema = z.object({
  name: z.string().min(1, 'Environment variable name is required'),
  value: z.string().optional(),
})

// Container Resource Requirements schema
export const resourceRequirementsSchema = z.object({
  requests: z.record(z.string(), z.string()).optional(),
  limits: z.record(z.string(), z.string()).optional(),
})

// Volume Mount schema
export const volumeMountSchema = z.object({
  name: z.string().min(1, 'Volume mount name is required'),
  mountPath: z.string().min(1, 'Mount path is required'),
  readOnly: z.boolean().optional(),
})

// Container schema
export const containerSchema = z.object({
  name: z.string().min(1, 'Container name is required'),
  image: z.string().min(1, 'Container image is required'),
  command: z.array(z.string()).optional(),
  args: z.array(z.string()).optional(),
  env: z.array(envVarSchema).optional(),
  resources: resourceRequirementsSchema.optional(),
  volumeMounts: z.array(volumeMountSchema).optional(),
})

// Runner spec schema
export const runnerSpecSchema = z
  .object({
    name: z.string().optional(),
    image: z.string().optional(),
    command: z.array(z.string()).optional(),
    args: z.array(z.string()).optional(),
    env: z.array(envVarSchema).optional(),
    resources: resourceRequirementsSchema.optional(),
    volumeMounts: z.array(volumeMountSchema).optional(),
  })
  .passthrough()

// Worker spec schema
export const workerSpecSchema = z
  .object({
    size: z.number().optional(),
    runner: runnerSpecSchema.optional(),
  })
  .passthrough()

// Volume schema
export const volumeSchema = z
  .object({
    name: z.string().min(1, 'Volume name is required'),
  })
  .passthrough()

// Engine/Decoder/Router Config shared schema
const componentConfigSchema = z
  .object({
    runner: runnerSpecSchema.optional(),
    worker: workerSpecSchema.optional(),
    minReplicas: z.number().optional(),
    maxReplicas: z.number().optional(),
    scaleTarget: z.number().optional(),
    scaleMetric: z.string().optional(),
    config: z.record(z.string(), z.string()).optional(),
    volumes: z.array(volumeSchema).optional(),
    initContainers: z.array(containerSchema).optional(),
    sidecars: z.array(containerSchema).optional(),
  })
  .passthrough()

// Supported Model Format schema - expanded
export const supportedModelFormatSchema = z
  .object({
    name: z.string().min(1, 'Format name is required'),
    version: z.string().optional(),
    modelType: z.string().optional(),
    modelArchitecture: z.string().optional(),
    quantization: z.string().optional(),
    autoSelect: z.boolean().optional(),
    priority: z.number().optional(),
  })
  .passthrough()

// Model Size Range schema
export const modelSizeRangeSchema = z.object({
  min: z.string().optional(),
  max: z.string().optional(),
})

// Serving Runtime Spec schema - expanded
export const servingRuntimeSpecSchema = z
  .object({
    supportedModelFormats: z
      .array(supportedModelFormatSchema)
      .min(1, 'At least one supported model format is required'),
    modelSizeRange: modelSizeRangeSchema.optional(),
    disabled: z.boolean().optional(),
    protocolVersions: z.array(z.string()).optional(),
    engineConfig: componentConfigSchema.optional(),
    decoderConfig: componentConfigSchema.optional(),
    routerConfig: componentConfigSchema.optional(),
    containers: z.array(containerSchema).optional(),
  })
  .passthrough()

// Full Runtime schema
export const clusterServingRuntimeSchema = z.object({
  apiVersion: z.string().default('ome.io/v1beta1'),
  kind: z.string().default('ClusterServingRuntime'),
  metadata: z.object({
    name: z
      .string()
      .min(1, 'Name is required')
      .regex(/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/, 'Name must be lowercase alphanumeric with dashes'),
    namespace: z.string().optional(),
  }),
  spec: servingRuntimeSpecSchema,
})

export type ClusterServingRuntimeFormData = z.infer<typeof clusterServingRuntimeSchema>
export type SupportedModelFormatFormData = z.infer<typeof supportedModelFormatSchema>
export type ContainerFormData = z.infer<typeof containerSchema>
