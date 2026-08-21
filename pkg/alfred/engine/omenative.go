package engine

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"

	"sigs.k8s.io/ome/pkg/alfred/config"
)

// OMENativeDiscovery builds the observation loop's executor-availability
// hook. No OMENative controller ships yet, so availability is a heuristic:
// the operator has the surface enabled AND the InferenceReplica CRD is
// registered (the strategy's footprint on the cluster). check is injected —
// cmd/alfred passes a discovery-client probe — so the gating logic stays
// unit-testable. Errors read as unavailable: degrading to recommend-only on
// a flaky discovery beats dispatching a migration verb nobody consumes.
func OMENativeDiscovery(store *config.Store, check func(ctx context.Context) (bool, error), log logr.Logger) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		if !*store.Get().OMENativeMigrationEnabled {
			return false
		}
		available, err := check(ctx)
		if err != nil {
			log.Error(err, "OMENative discovery failed; treating executor as unavailable")
			return false
		}
		return available
	}
}

// CachedProbe rate-limits a discovery probe: successful results are held for
// ttl, so the API server is not asked on every observation pass yet an
// executor installed later is noticed without a restart. Errors are never
// cached — the next call re-probes. now is injectable for tests; nil uses
// the wall clock.
func CachedProbe(ttl time.Duration, now func() time.Time, probe func(ctx context.Context) (bool, error)) func(ctx context.Context) (bool, error) {
	if now == nil {
		now = time.Now
	}
	var mu sync.Mutex
	var last time.Time
	var cached, seeded bool
	return func(ctx context.Context) (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		if seeded && now().Sub(last) < ttl {
			return cached, nil
		}
		result, err := probe(ctx)
		if err != nil {
			return false, err
		}
		cached, last, seeded = result, now(), true
		return result, nil
	}
}
