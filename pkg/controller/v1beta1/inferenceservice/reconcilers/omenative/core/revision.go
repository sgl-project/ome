package core

import (
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// isvcGVK is the GroupVersionKind stamped on owner references emitted
// for the parent InferenceService. Centralized so an API version bump
// flips this in one place. Consumed by render.go.
var isvcGVK = v1beta1.SchemeGroupVersion.WithKind("InferenceService")
