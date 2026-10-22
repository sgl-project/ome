// Package ownedscaler brings a scaler object OME owns for a Component, a
// HorizontalPodAutoscaler or a KEDA ScaledObject, to its desired shape. Both
// kinds share one lifecycle: read the live object, then create, update,
// delete or keep it. A Kind supplies the few rules that differ.
package ownedscaler

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/scalermetadata"
)

// Kind describes one scaler kind: its name in messages, the autoscaler class
// that asks for it, and the rules that differ between kinds.
type Kind[T client.Object] struct {
	// Name is the object kind as errors and logs name it.
	Name string
	// Class is the autoscaler class whose Components get this kind.
	Class constants.AutoscalerClassType
	// Log receives one line per pass with the decision taken, and the error
	// of a failed write. The pass line logs at the verbosity this logger
	// carries.
	Log logr.Logger
	// New returns an empty object to read the live one into.
	New func() T
	// Creatable reports whether desired is complete enough to create. Nil
	// means every desired object is.
	Creatable func(desired T) bool
	// SpecEqual reports whether the spec fields OME manages are the same on
	// both objects.
	SpecEqual func(desired, existing T) bool
	// CopySpec writes the spec of from onto to.
	CopySpec func(from, to T)
}

// Matches reports whether the live object already carries the autoscaler
// class, the managed labels and annotations, the controller owner and the
// spec of the desired one. Live metadata OME does not manage is ignored.
func (k Kind[T]) Matches(desired, existing T) bool {
	if desired.GetAnnotations()[constants.AutoscalerClass] != existing.GetAnnotations()[constants.AutoscalerClass] {
		return false
	}
	return scalermetadata.Contains(existing.GetLabels(), existing.GetAnnotations(), desired.GetLabels(), desired.GetAnnotations()) &&
		k.SpecEqual(desired, existing) &&
		equality.Semantic.DeepEqual(metav1.GetControllerOf(desired), metav1.GetControllerOf(existing))
}

// checkController refuses a live object whose controller is not the desired
// one, so OME never rewrites a scaler another owner created. A desired object
// with no controller accepts any live object.
func (k Kind[T]) checkController(desired, existing T) error {
	expected := metav1.GetControllerOf(desired)
	if expected == nil {
		return nil
	}
	actual := metav1.GetControllerOf(existing)
	if actual != nil && actual.UID == expected.UID {
		return nil
	}
	if actual == nil {
		return fmt.Errorf("%s %s/%s is not controlled by expected owner %s %s (UID %q): object has no controller owner", k.Name, existing.GetNamespace(), existing.GetName(), expected.Kind, expected.Name, expected.UID)
	}
	return fmt.Errorf("%s %s/%s is not controlled by expected owner %s %s (UID %q): controller is %s %s (UID %q)", k.Name, existing.GetNamespace(), existing.GetName(), expected.Kind, expected.Name, expected.UID, actual.Kind, actual.Name, actual.UID)
}

// forUpdate returns the live object with the desired spec, the managed
// metadata merged in, and the desired controller owner in place of the live
// one. Other owner references and unmanaged metadata stay as they are.
func (k Kind[T]) forUpdate(desired, existing T) T {
	updated := existing.DeepCopyObject().(T)
	k.CopySpec(desired, updated)
	labels, annotations := scalermetadata.Merge(
		existing.GetLabels(),
		existing.GetAnnotations(),
		desired.GetLabels(),
		desired.GetAnnotations(),
	)
	if _, present := desired.GetAnnotations()[constants.AutoscalerClass]; !present {
		delete(annotations, constants.AutoscalerClass)
	}
	updated.SetLabels(labels)
	updated.SetAnnotations(annotations)
	updated.SetOwnerReferences(mergeOwnerReferences(existing.GetOwnerReferences(), desired.GetOwnerReferences()))
	return updated
}

// mergeOwnerReferences keeps the live object's plain owner references and
// takes the controller reference from the desired object.
func mergeOwnerReferences(existing, desired []metav1.OwnerReference) []metav1.OwnerReference {
	merged := make([]metav1.OwnerReference, 0, len(existing)+len(desired))
	for _, ref := range existing {
		if ref.Controller == nil || !*ref.Controller {
			merged = append(merged, ref)
		}
	}
	for _, ref := range desired {
		if ref.Controller != nil && *ref.Controller {
			merged = append(merged, ref)
		}
	}
	return merged
}

// Reconciler brings one owned scaler object to its desired shape.
type Reconciler[T client.Object] struct {
	client client.Client
	kind   Kind[T]
	// Desired is the object to reach. After an update it is the object as
	// written: the live object with the desired spec, metadata and owner.
	Desired T
}

// New returns a Reconciler that brings desired into being through c.
func New[T client.Object](c client.Client, kind Kind[T], desired T) *Reconciler[T] {
	return &Reconciler[T]{client: c, kind: kind, Desired: desired}
}

// Reconcile reads the live object and creates, updates or deletes it so that
// it matches Desired. Nothing is written when the live object already
// matches, when the desired class names another kind, or when the kind rules
// the desired object out. A live object another controller owns, or one that
// is being deleted, is an error.
func (r *Reconciler[T]) Reconcile(ctx context.Context) error {
	result, existing, err := r.plan(ctx)
	r.kind.Log.Info("Reconciling "+r.kind.Name, "namespace", r.Desired.GetNamespace(), "name", r.Desired.GetName(), "checkResult", result.String())
	if err != nil {
		return err
	}

	var writeErr error
	switch result {
	case constants.CheckResultCreate:
		writeErr = r.client.Create(ctx, r.Desired)
	case constants.CheckResultUpdate:
		r.Desired = r.kind.forUpdate(r.Desired, existing)
		writeErr = r.client.Update(ctx, r.Desired)
	case constants.CheckResultDelete:
		writeErr = r.client.Delete(ctx, r.Desired)
	default:
		return nil
	}
	if writeErr != nil {
		r.kind.Log.Error(writeErr, "Failed to reconcile "+r.kind.Name, "namespace", r.Desired.GetNamespace(), "name", r.Desired.GetName())
		return writeErr
	}
	return nil
}

// plan reads the live object and decides what Reconcile writes.
func (r *Reconciler[T]) plan(ctx context.Context) (constants.CheckResultType, T, error) {
	var none T
	existing := r.kind.New()
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(r.Desired), existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return constants.CheckResultUnknown, none, err
		}
		if r.shouldCreate() {
			return constants.CheckResultCreate, none, nil
		}
		return constants.CheckResultSkipped, none, nil
	}
	if err := r.kind.checkController(r.Desired, existing); err != nil {
		return constants.CheckResultUnknown, existing, err
	}
	if existing.GetDeletionTimestamp() != nil {
		return constants.CheckResultUnknown, existing, fmt.Errorf("%s %s/%s is terminating", r.kind.Name, existing.GetNamespace(), existing.GetName())
	}
	if r.kind.Matches(r.Desired, existing) {
		return constants.CheckResultExisted, existing, nil
	}
	if r.shouldDelete() {
		return constants.CheckResultDelete, existing, nil
	}
	return constants.CheckResultUpdate, existing, nil
}

// shouldCreate is true when the desired class is this kind's class, or no
// class is named, and the kind accepts the desired object.
func (r *Reconciler[T]) shouldCreate() bool {
	if r.kind.Creatable != nil && !r.kind.Creatable(r.Desired) {
		return false
	}
	class := r.desiredClass()
	return class == "" || class == r.kind.Class
}

// shouldDelete is true when the desired class is external: another
// controller scales the Component, so OME's object goes.
func (r *Reconciler[T]) shouldDelete() bool {
	return r.desiredClass() == constants.AutoscalerClassExternal
}

func (r *Reconciler[T]) desiredClass() constants.AutoscalerClassType {
	return constants.AutoscalerClassType(r.Desired.GetAnnotations()[constants.AutoscalerClass])
}
