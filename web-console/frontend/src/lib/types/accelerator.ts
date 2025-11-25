// AcceleratorClass TypeScript types

export interface AcceleratorClass {
  apiVersion: string
  kind: string
  metadata: {
    name: string
    creationTimestamp?: string
    [key: string]: any
  }
  spec: AcceleratorClassSpec
  status?: AcceleratorClassStatus
}

export interface AcceleratorClassSpec {
  acceleratorType?: string
  acceleratorCount?: number
  memoryGB?: number
  [key: string]: any
}

export interface AcceleratorClassStatus {
  [key: string]: any
}
