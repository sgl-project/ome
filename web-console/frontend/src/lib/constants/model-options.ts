/**
 * Shared options for model format and framework dropdowns
 * Used by both model creation and edit pages
 */

export const MODEL_FORMAT_OPTIONS = [
  { value: '', label: 'Select format...' },
  { value: 'safetensors', label: 'SafeTensors' },
  { value: 'pytorch', label: 'PyTorch' },
  { value: 'gguf', label: 'GGUF' },
  { value: 'ggml', label: 'GGML' },
  { value: 'onnx', label: 'ONNX' },
  { value: 'tensorflow', label: 'TensorFlow' },
  { value: 'huggingface', label: 'HuggingFace' },
] as const

export const MODEL_FRAMEWORK_OPTIONS = [
  { value: '', label: 'Select framework...' },
  { value: 'transformers', label: 'Transformers' },
  { value: 'pytorch', label: 'PyTorch' },
  { value: 'tensorflow', label: 'TensorFlow' },
  { value: 'jax', label: 'JAX' },
  { value: 'onnx-runtime', label: 'ONNX Runtime' },
  { value: 'llama-cpp', label: 'llama.cpp' },
] as const

export type ModelFormatValue = (typeof MODEL_FORMAT_OPTIONS)[number]['value']
export type ModelFrameworkValue = (typeof MODEL_FRAMEWORK_OPTIONS)[number]['value']
