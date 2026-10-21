package controllerconfig

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// loadBlock reads the inferenceservice-config ConfigMap from the API server
// and parses one block of it. Start-up paths use it: the manager reads each
// block once and keeps the result for the life of the process.
func loadBlock[T any](clientset kubernetes.Interface, parse func(*v1.ConfigMap) (T, error)) (T, error) {
	return loadBlockCached(nil, clientset, parse)
}

// loadBlockCached reads the ConfigMap through the TTL cache and parses one
// block of it. Reconcile paths use it, so one pass issues one GET for every
// block it reads. A nil cache reads the API server directly.
func loadBlockCached[T any](cache *ConfigCache, clientset kubernetes.Interface, parse func(*v1.ConfigMap) (T, error)) (T, error) {
	configMap, err := cache.get(clientset)
	if err != nil {
		var none T
		return none, err
	}
	return parse(configMap)
}

// positiveDurationOrZero reads a duration the operator may leave out. Empty,
// malformed and non-positive text all read as zero, which every consumer
// treats as "not configured" and answers with its own documented fallback.
// Where a wrong value must stop the manager instead, a Validate method checks
// the text at start-up.
func positiveDurationOrZero(value string) time.Duration {
	if d, err := time.ParseDuration(value); err == nil && d > 0 {
		return d
	}
	return 0
}

// readPositiveDuration reads a duration the operator wrote under key and
// refuses text that does not parse or is not positive. The caller decides
// what an absent value means; the error names the key and the value as
// written.
func readPositiveDuration(key, value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, value, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid %s: must be > 0, got %s", key, d)
	}
	return d, nil
}

// integer is any signed integer kind a count key is decoded into.
type integer interface {
	~int | ~int32 | ~int64
}

// readPositiveCount reads a count the operator wrote under key and refuses
// zero or a negative number. The caller decides what an absent value means;
// the error names the key and the value.
func readPositiveCount[N integer](key string, value N) (N, error) {
	if value <= 0 {
		return 0, fmt.Errorf("invalid %s: must be > 0, got %d", key, value)
	}
	return value, nil
}
