package v1beta1

import (
	opensourcev1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Type aliases for opensource types referenced by local type definitions.
// These allow local types (TrainingJob, ReplicationJob, InferenceGraph) to
// embed or reference types defined in the opensource module without changing
// their struct field types.

type StorageSpec = opensourcev1beta1.StorageSpec
type ScaleMetric = opensourcev1beta1.ScaleMetric
