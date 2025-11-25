import { z } from 'zod'

// Model Format schema
export const modelFormatSchema = z.object({
  name: z.string().min(1, 'Model format name is required'),
  version: z.string().optional(),
})

// Model Framework schema
export const modelFrameworkSchema = z.object({
  name: z.string().min(1, 'Framework name is required'),
  version: z.string().optional(),
})

// Storage schema
export const storageSchema = z.object({
  storageUri: z.string().min(1, 'Storage URI is required'),
  path: z.string().min(1, 'Path is required'),
  storageKey: z.string().optional(),
})

// Resource Requirements schema
export const resourceRequirementsSchema = z.object({
  requests: z.record(z.string()).optional(),
  limits: z.record(z.string()).optional(),
})

// Base Model Spec schema
export const baseModelSpecSchema = z.object({
  vendor: z.string().min(1, 'Vendor is required'),
  modelParameterSize: z.string().optional(),
  modelFormat: modelFormatSchema.optional(),
  modelFramework: modelFrameworkSchema.optional(),
  storage: storageSchema,
  resources: resourceRequirementsSchema.optional(),
})

// ClusterBaseModel schema (cluster-scoped)
export const clusterBaseModelSchema = z.object({
  apiVersion: z.string().default('ome.io/v1beta1'),
  kind: z.string().default('ClusterBaseModel'),
  metadata: z.object({
    name: z
      .string()
      .min(1, 'Name is required')
      .regex(/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/, 'Name must be lowercase alphanumeric with dashes'),
    labels: z.record(z.string()).optional(),
    annotations: z.record(z.string()).optional(),
  }),
  spec: baseModelSpecSchema,
})

// BaseModel schema (namespace-scoped)
export const baseModelSchema = z.object({
  apiVersion: z.string().default('ome.io/v1beta1'),
  kind: z.string().default('BaseModel'),
  metadata: z.object({
    name: z
      .string()
      .min(1, 'Name is required')
      .regex(/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/, 'Name must be lowercase alphanumeric with dashes'),
    namespace: z.string().min(1, 'Namespace is required'),
    labels: z.record(z.string()).optional(),
    annotations: z.record(z.string()).optional(),
  }),
  spec: baseModelSpecSchema,
})

export type ClusterBaseModelFormData = z.infer<typeof clusterBaseModelSchema>
export type BaseModelFormData = z.infer<typeof baseModelSchema>
