// Package kueue renders an AcceleratorQuota tree into stock Kueue objects and
// applies them.
//
// Rendering is pure and lives in render.go; everything that touches a cluster
// lives in backend.go. The split is what lets the mapping — which is where the
// subtle Kueue rules live — be pinned by table tests with no apiserver.
package kueue

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Options are the deploy-time decisions the renderer cannot make for itself.
// None has an in-code default: absent configuration disables materialization
// rather than inventing a value that would show up as a real quota.
type Options struct {
	// FieldManager owns the fields this controller applies. It is also the
	// value of the managed-by label, so the objects a given manager owns are
	// selectable, and two managers pointed at one cluster do not silently
	// fight over the same objects.
	FieldManager string

	// CoverResources are the non-accelerator resources every rendered
	// ClusterQueue must fund, with the amount to fund them at.
	//
	// This exists because Kueue refuses to admit a workload whose request
	// names a resource the ClusterQueue does not cover, and every serving pod
	// requests cpu and memory. A queue budgeted only for accelerators would
	// therefore admit nothing, with no object in an error state to say why.
	// The values are not a budget — they are a ceiling high enough not to be
	// one — which is why they are configured rather than authored on the CR.
	CoverResources map[corev1.ResourceName]resource.Quantity
}

// Enabled reports whether materialization is configured. Absent configuration
// disables it, so a cluster that has not opted in writes nothing.
func (o Options) Enabled() bool {
	return o.FieldManager != "" && len(o.CoverResources) > 0
}

// Validate rejects configuration that would render objects Kueue cannot use.
// Called at startup so a misconfiguration is a failed launch rather than a
// tenant whose quota silently never admits.
func (o Options) Validate() error {
	if o.FieldManager == "" && len(o.CoverResources) == 0 {
		return nil
	}
	if o.FieldManager == "" {
		return fmt.Errorf("field manager is required when cover resources are set")
	}
	if len(o.CoverResources) == 0 {
		return fmt.Errorf("at least one cover resource is required when a field manager is set")
	}
	for name, qty := range o.CoverResources {
		if name == "" {
			return fmt.Errorf("cover resource name must not be empty")
		}
		if qty.Sign() <= 0 {
			return fmt.Errorf("cover resource %q must be positive, got %s", name, qty.String())
		}
	}
	return nil
}
