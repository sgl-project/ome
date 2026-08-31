package kueue

import (
	"context"
	"fmt"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kueuev1beta2 "sigs.k8s.io/kueue/apis/kueue/v1beta2"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/quota/backend"
)

// Backend applies rendered objects to one cluster.
//
// Writer performs the apply and must be a client whose reads are uncached: the
// collision check below is a read-then-write, and a cached read would let a
// queue created moments earlier be adopted as if it did not exist.
type Backend struct {
	Writer  client.Client
	Reader  client.Reader
	Options Options
}

// Name identifies this backend in conditions and logs.
func (b *Backend) Name() string { return "kueue" }

var _ backend.Backend = (*Backend)(nil)

// objectRef is enough to identify a rendered object for a collision check, a
// sweep, and an operator-facing message.
type objectRef struct {
	kind      string
	name      string
	namespace string
}

func (r objectRef) String() string {
	if r.namespace == "" {
		return r.kind + " " + r.name
	}
	return r.kind + " " + r.namespace + "/" + r.name
}

// Materialize renders the plan and applies it.
//
// A node whose objects cannot be written does not stop the pass: one leaf
// naming a flavor that does not exist, or colliding with an object somebody
// else authored, must not hold every other tenant's quota hostage. Those become
// per-node outcomes; a returned error means the pass itself could not run.
func (b *Backend) Materialize(ctx context.Context, plan backend.Plan) (backend.Result, error) {
	flavors, err := b.flavors(ctx)
	if err != nil {
		return backend.Result{}, err
	}

	objs := Render(plan, flavors, b.Options)
	result := backend.Result{Nodes: map[string]backend.NodeOutcome{}}

	for node, missing := range objs.Skipped {
		result.Note(node, v1beta1.AcceleratorQuotaReasonFlavorMissing,
			fmt.Sprintf("no ResourceFlavor named %v on this cluster; those budgets are not materialized",
				missing))
	}

	// Cohorts first, root-first as rendered: a ClusterQueue naming a Cohort
	// that does not exist yet is not rejected, it is silently attached to an
	// implicit cohort with no parent and no limits. Ordering keeps that
	// flattening from happening even transiently.
	for _, ac := range objs.Cohorts {
		b.applyOne(ctx, &result, *ac.Name, objectRef{kind: "Cohort", name: *ac.Name},
			&kueuev1beta2.Cohort{}, ac)
	}
	for _, ac := range objs.ClusterQueues {
		b.applyOne(ctx, &result, *ac.Name, objectRef{kind: "ClusterQueue", name: *ac.Name},
			&kueuev1beta2.ClusterQueue{}, ac)
	}
	for _, ac := range objs.LocalQueues {
		node := ac.Labels[v1beta1.AcceleratorQuotaNodeLabel]
		b.applyOne(ctx, &result, node,
			objectRef{kind: "LocalQueue", name: *ac.Name, namespace: *ac.Namespace},
			&kueuev1beta2.LocalQueue{}, ac)
	}
	return result, nil
}

// applyOne refuses a collision, then applies. Failures are recorded against the
// owning node rather than aborting.
func (b *Backend) applyOne(ctx context.Context, result *backend.Result, node string, ref objectRef, probe client.Object, ac runtime.ApplyConfiguration) {
	owned, err := b.ownedByUs(ctx, ref, probe)
	if err != nil {
		result.Note(node, v1beta1.AcceleratorQuotaReasonMaterializationFailed,
			fmt.Sprintf("cannot read %s: %v", ref, err))
		return
	}
	if !owned {
		// Adoption is refused rather than performed. Server-side apply will
		// happily take over an object it did not create, so this check is the
		// only thing standing between a hand-authored queue and a controller
		// silently rewriting it.
		result.Note(node, v1beta1.AcceleratorQuotaReasonObjectConflict,
			fmt.Sprintf("%s already exists and is not managed by %s; refusing to adopt it",
				ref, b.Options.FieldManager))
		return
	}
	if err := b.Writer.Apply(ctx, ac, client.FieldOwner(b.Options.FieldManager)); err != nil {
		result.Note(node, v1beta1.AcceleratorQuotaReasonMaterializationFailed,
			fmt.Sprintf("cannot apply %s: %v", ref, err))
		return
	}
	result.Applied++
}

// ownedByUs reports whether the object is absent (free to create) or already
// carries this manager's label. Anything else belongs to somebody else.
func (b *Backend) ownedByUs(ctx context.Context, ref objectRef, into client.Object) (bool, error) {
	key := types.NamespacedName{Name: ref.name, Namespace: ref.namespace}
	if err := b.Reader.Get(ctx, key, into); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	return into.GetLabels()[v1beta1.AcceleratorQuotaManagedByLabel] == b.Options.FieldManager, nil
}

// flavors is the set of ResourceFlavor names on this cluster.
//
// A cluster without Kueue installed has no flavors and is not an error: quota
// without Kueue materializes nothing, and the caller has already decided
// whether that is worth reporting.
func (b *Backend) flavors(ctx context.Context) (map[string]struct{}, error) {
	var list kueuev1beta2.ResourceFlavorList
	if err := b.Reader.List(ctx, &list); err != nil {
		if apimeta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make(map[string]struct{}, len(list.Items))
	for i := range list.Items {
		out[list.Items[i].Name] = struct{}{}
	}
	return out, nil
}

// Sweep deletes objects this manager owns whose node is no longer written.
//
// keep is every node the plan wrote plus every node it retained: a frozen
// subtree's objects must survive, which is exactly what distinguishes a freeze
// from a deletion. Objects without this manager's label are never touched, so a
// hand-authored queue outlives any sweep.
func (b *Backend) Sweep(ctx context.Context, keep sets.Set[string]) (int, error) {
	selector := client.MatchingLabels{v1beta1.AcceleratorQuotaManagedByLabel: b.Options.FieldManager}

	var deleted int
	var errs []string

	var localQueues kueuev1beta2.LocalQueueList
	if err := b.Reader.List(ctx, &localQueues, selector); err != nil && !apimeta.IsNoMatchError(err) {
		return deleted, err
	}
	for i := range localQueues.Items {
		lq := &localQueues.Items[i]
		if keep.Has(lq.Labels[v1beta1.AcceleratorQuotaNodeLabel]) {
			continue
		}
		if err := b.deleteObject(ctx, lq); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		deleted++
	}

	// ClusterQueues before Cohorts: deleting a parent first would reparent its
	// children onto an implicit cohort for as long as the sweep takes.
	var clusterQueues kueuev1beta2.ClusterQueueList
	if err := b.Reader.List(ctx, &clusterQueues, selector); err != nil && !apimeta.IsNoMatchError(err) {
		return deleted, err
	}
	for i := range clusterQueues.Items {
		cq := &clusterQueues.Items[i]
		if keep.Has(cq.Labels[v1beta1.AcceleratorQuotaNodeLabel]) {
			continue
		}
		if err := b.deleteObject(ctx, cq); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		deleted++
	}

	var cohorts kueuev1beta2.CohortList
	if err := b.Reader.List(ctx, &cohorts, selector); err != nil && !apimeta.IsNoMatchError(err) {
		return deleted, err
	}
	for i := range cohorts.Items {
		c := &cohorts.Items[i]
		if keep.Has(c.Labels[v1beta1.AcceleratorQuotaNodeLabel]) {
			continue
		}
		if err := b.deleteObject(ctx, c); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		deleted++
	}

	if len(errs) > 0 {
		sort.Strings(errs)
		return deleted, fmt.Errorf("sweep: %v", errs)
	}
	return deleted, nil
}

func (b *Backend) deleteObject(ctx context.Context, obj client.Object) error {
	if err := b.Writer.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete %T %s: %w", obj, obj.GetName(), err)
	}
	return nil
}
