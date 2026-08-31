// Package acceleratorquota validates AcceleratorQuota writes against the tree
// they would produce.
//
// No single CR can be judged alone: whether a parent resolves, whether a cycle
// forms, whether a budget fits inside its parent's, and whether two leaves claim
// a namespace are all properties of the whole set. So the validator lists every
// node, splices in the object under review, and rebuilds the tree.
//
// It judges a write by what it ADDS, not by the state it inherits. A tree can
// already be violating when the write arrives — concurrent admissions reach that
// state, which is why the controller re-checks at all — and an admin has to be
// able to repair it. Rejecting on "the resulting tree has violations" would lock
// them out of their own fix.
package acceleratorquota

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/quota/tree"
)

var log = logf.Log.WithName("acceleratorquota-validation-webhook")

// +kubebuilder:object:generate=false

// Validator rejects an AcceleratorQuota write that would break the quota tree.
//
// It is fast feedback, not the authority: admission cannot serialize concurrent
// writes, so the controller re-checks the same invariants at reconcile and
// freezes what slips through. That division is why this handler can afford to be
// permissive about pre-existing damage.
type Validator struct {
	// Client is an uncached reader. The tree must be spliced against what the
	// apiserver has right now: judging against a lagging cache would miss a
	// sibling admitted moments ago and let two writes jointly bust a parent.
	Client  client.Reader
	Decoder admission.Decoder
	// Options carry the reserved root name and depth bound, the same values the
	// controller uses. A mismatch would make admission and reconcile disagree.
	Options tree.Options
}

func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Operation {
	case admissionv1.Create, admissionv1.Update, admissionv1.Delete:
	default:
		return admission.Allowed("")
	}

	var live v1beta1.AcceleratorQuotaList
	if err := v.Client.List(ctx, &live); err != nil {
		// Fail closed. This webhook is registered failurePolicy=Fail, so a
		// silent Allowed here would be a worse outcome than a rejected write:
		// it would admit exactly the tree-breaking change the handler exists to
		// stop, at the moment the apiserver is already unhealthy.
		log.Error(err, "listing AcceleratorQuotas for tree assembly")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	proposed, err := v.proposedSet(req, live.Items)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	_, before, err := tree.Build(live.Items, v.Options)
	if err != nil {
		// A configuration failure, not a property of the write. Rejecting is
		// right — with no root name every node is unplaceable — but the message
		// must point at the operator's config, not at the admin's object.
		return admission.Errored(http.StatusInternalServerError, err)
	}
	_, after, err := tree.Build(proposed, v.Options)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if newly := after.Diff(before); len(newly) > 0 {
		return admission.Denied(newly.Error())
	}
	return admission.Allowed("")
}

// proposedSet returns the node set as it would be if this request were admitted.
func (v *Validator) proposedSet(req admission.Request, live []v1beta1.AcceleratorQuota) ([]v1beta1.AcceleratorQuota, error) {
	if req.Operation == admissionv1.Delete {
		// Deleting a node can orphan a whole subtree, which is invisible from
		// the deleted object alone — the damage is on the children left behind.
		return tree.Without(live, req.Name), nil
	}
	incoming := &v1beta1.AcceleratorQuota{}
	if err := v.Decoder.Decode(req, incoming); err != nil {
		return nil, fmt.Errorf("decoding AcceleratorQuota: %w", err)
	}
	return tree.Splice(live, *incoming), nil
}
