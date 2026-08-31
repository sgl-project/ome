package kueue

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	kueuev1beta2 "sigs.k8s.io/kueue/apis/kueue/v1beta2"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/quota/backend"
	"sigs.k8s.io/ome/pkg/quota/tree"
	"sigs.k8s.io/ome/pkg/quota/usage"
)

var _ backend.UsageWatcher = (*Backend)(nil)

// WatchUsage wakes the controller when a queue it owns changes what it holds.
//
// Kueue rewrites ClusterQueue status for plenty of reasons that move no
// consumption — pending and reserving counts, condition restamps — and every
// admitted event costs a whole-tree pass, so the predicate compares the two
// lists this backend actually reads and admits nothing else.
//
// Create and Delete are admitted whatever they carry. A create is how the
// informer replays queues that already exist at startup, and a delete is a
// queue that has gone away underneath us: the objects carry no OwnerReference,
// deliberately, so nothing else would notice one being removed by hand.
func (b *Backend) WatchUsage() (client.Object, predicate.Predicate) {
	owned := func(obj client.Object) bool {
		return obj != nil &&
			obj.GetLabels()[v1beta1.AcceleratorQuotaManagedByLabel] == b.Options.FieldManager
	}

	return &kueuev1beta2.ClusterQueue{}, predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return owned(e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return owned(e.Object) },
		GenericFunc: func(e event.GenericEvent) bool { return owned(e.Object) },
		UpdateFunc: func(e event.UpdateEvent) bool {
			before, okBefore := e.ObjectOld.(*kueuev1beta2.ClusterQueue)
			after, okAfter := e.ObjectNew.(*kueuev1beta2.ClusterQueue)
			if !okBefore || !okAfter || !owned(after) {
				return false
			}
			return !equalFlavorUsage(before.Status.FlavorsUsage, after.Status.FlavorsUsage) ||
				!equalFlavorUsage(before.Status.FlavorsReservation, after.Status.FlavorsReservation)
		},
	}
}

// equalFlavorUsage compares two of Kueue's usage lists position by position.
//
// Position rather than by name: Kueue renders these from the queue's own
// resourceGroups, so the order is stable for a given spec, and a reordering
// means the spec moved — which is a change worth a pass in its own right.
func equalFlavorUsage(a, b []kueuev1beta2.FlavorUsage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || len(a[i].Resources) != len(b[i].Resources) {
			return false
		}
		for j := range a[i].Resources {
			x, y := a[i].Resources[j], b[i].Resources[j]
			if x.Name != y.Name || x.Total.Cmp(y.Total) != 0 || x.Borrowed.Cmp(y.Borrowed) != 0 {
				return false
			}
		}
	}
	return true
}

// ReadUsage reports what each leaf's ClusterQueue currently holds.
//
// One LIST rather than a Get per leaf: a fleet is a handful of queues, and the
// label selector is also the ownership filter, so a queue this manager did not
// write cannot contribute to a node's figures no matter what it is named. The
// alternative — Get by node name — would read a stranger's queue that happens
// to collide and report its consumption as a tenant's own.
//
// A leaf whose queue is missing is left out of the result rather than recorded
// as holding nothing. The two look identical in a status field and mean
// opposite things: one is a tenant using none of its budget, the other is a
// budget that was never made real.
func (b *Backend) ReadUsage(ctx context.Context, leaves []*tree.Node) (map[string]map[usage.Key]usage.Observed, error) {
	if len(leaves) == 0 {
		return nil, nil
	}

	var list kueuev1beta2.ClusterQueueList
	if err := b.Reader.List(ctx, &list, client.MatchingLabels{
		v1beta1.AcceleratorQuotaManagedByLabel: b.Options.FieldManager,
	}); err != nil {
		return nil, fmt.Errorf("listing ClusterQueues: %w", err)
	}

	// Indexed by the node label rather than by object name so the reverse
	// mapping stays the one the sweep and the materializer already use.
	byNode := make(map[string]*kueuev1beta2.ClusterQueue, len(list.Items))
	for i := range list.Items {
		if node := list.Items[i].Labels[v1beta1.AcceleratorQuotaNodeLabel]; node != "" {
			byNode[node] = &list.Items[i]
		}
	}

	out := make(map[string]map[usage.Key]usage.Observed, len(leaves))
	for _, leaf := range leaves {
		cq, ok := byNode[leaf.Name()]
		if !ok {
			continue
		}
		out[leaf.Name()] = observed(cq.Status)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// observed converts one ClusterQueue's status into the rollup's terms.
//
// Kueue reports the same (resource, flavor) pairs twice: flavorsUsage for what
// is admitted, flavorsReservation for what is reserved, which is the wider set
// because a workload reserves before it is admitted. Both are folded onto one
// key, so that split stays contained here instead of repeating in every caller.
func observed(status kueuev1beta2.ClusterQueueStatus) map[usage.Key]usage.Observed {
	out := map[usage.Key]usage.Observed{}

	for _, flavor := range status.FlavorsUsage {
		for _, r := range flavor.Resources {
			key := usage.Key{ResourceName: string(r.Name), ResourceFlavor: string(flavor.Name)}
			seen := out[key]
			// Deep-copied because Reader may be cached: a Quantity carries an
			// inf.Dec pointer, so handing the caller a shallow copy would let
			// the rollup's arithmetic write through into the informer's object.
			seen.Admitted = r.Total.DeepCopy()
			seen.Borrowed = r.Borrowed.DeepCopy()
			out[key] = seen
		}
	}

	for _, flavor := range status.FlavorsReservation {
		for _, r := range flavor.Resources {
			key := usage.Key{ResourceName: string(r.Name), ResourceFlavor: string(flavor.Name)}
			seen := out[key]
			seen.Reserved = r.Total.DeepCopy()
			out[key] = seen
		}
	}

	// Admitted work holds a reservation by definition, so reserved can never be
	// the smaller of the two. Taking the larger keeps that true for a backend
	// that reports usage but not reservations, instead of publishing a pair
	// that contradicts itself.
	for key, seen := range out {
		if seen.Reserved.Cmp(seen.Admitted) < 0 {
			seen.Reserved = seen.Admitted.DeepCopy()
			out[key] = seen
		}
	}

	return out
}
