package keda

import (
	"context"
	"fmt"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/scalermetadata"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/utils"
)

var log = logf.Log.WithName("KEDAReconciler")

// defaultMinReplicas is the floor applied when the component spec does not
// declare MinReplicas > 0.
const defaultMinReplicas int32 = 1

// KEDAReconciler reconciles the ScaledObject resource.
//
// The generator is parameterized: the constructor accepts a caller-
// supplied ScaleTarget (Raw Deployment passes apps/v1 Deployment; the
// IR-managed path passes an InferenceReplica target) and a typed
// *KedaAutoscaler block whose Triggers / Advanced / PollingInterval /
// CooldownPeriod / IdleReplicaCount / Fallback fields are forwarded
// verbatim to the ScaledObject spec.
type KEDAReconciler struct {
	client       client.Client
	scheme       *runtime.Scheme
	ScaledObject *kedav1.ScaledObject
}

// NewKEDAReconciler creates a new KEDAReconciler whose generated ScaledObject
// targets the caller-supplied Deployment. Reads the per-Component Autoscaler
// block (Autoscaler.Keda) from componentExt for trigger configuration;
// MinReplicas / MaxReplicas on the ComponentExtensionSpec drive the bounds.
// See NewKEDAReconcilerForTarget for the IR-managed path.
func NewKEDAReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
) *KEDAReconciler {

	scaledObject := createScaledObjectFromComponentExt(componentMeta, componentExt)

	return &KEDAReconciler{
		client:       client,
		scheme:       scheme,
		ScaledObject: scaledObject,
	}
}

// NewKEDAReconcilerForTarget builds a KEDAReconciler whose generated
// ScaledObject targets the caller-supplied scaleTargetRef (typically an
// InferenceReplica for the IR-managed path). kedaSpec carries the
// resolved KedaAutoscaler block (resolver output); Triggers, Advanced,
// PollingInterval, CooldownPeriod, IdleReplicaCount, and Fallback are all
// forwarded verbatim to the ScaledObject spec.
//
// It is the entry point for the IR-managed dispatch and stays free of
// any ComponentExtensionSpec concerns so the two callers are completely
// decoupled.
//
// Shared autoscaler dispatch supplies a non-nil KEDA block with at least one
// trigger. Direct lower-level callers may still pass nil or empty input;
// Reconcile() defensively skips applying the resulting invalid object.
func NewKEDAReconcilerForTarget(
	client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	scaleTargetRef kedav1.ScaleTarget,
	kedaSpec *v1beta1.KedaAutoscaler,
	minReplicas int32,
	maxReplicas int32,
) *KEDAReconciler {
	return &KEDAReconciler{
		client:       client,
		scheme:       scheme,
		ScaledObject: createScaledObject(componentMeta, scaleTargetRef, kedaSpec, minReplicas, maxReplicas),
	}
}

// createScaledObject builds the desired ScaledObject from explicit inputs.
//
// scaleTargetRef is forwarded verbatim to the ScaledObject spec — callers
// supply `{apps/v1, Deployment, <name>}` for Raw Deployment dispatch and
// `{ome.io/v1beta1, InferenceReplica, <ir-name>}` for the IR-managed path.
//
// kedaSpec / minReplicas / maxReplicas come from the resolver or the Raw
// Deployment shim (createScaledObjectFromComponentExt). When kedaSpec is
// non-nil the Triggers, Advanced, PollingInterval, CooldownPeriod,
// IdleReplicaCount, and Fallback fields are all forwarded verbatim. Shared
// dispatch validates that at least one trigger is present. Direct lower-level
// callers with nil or empty input produce a triggerless object that Reconcile()
// defensively skips applying.
func createScaledObject(
	componentMeta metav1.ObjectMeta,
	scaleTargetRef kedav1.ScaleTarget,
	kedaSpec *v1beta1.KedaAutoscaler,
	minReplicas int32,
	maxReplicas int32,
) *kedav1.ScaledObject {
	filteredLabels := make(map[string]string)
	for key, value := range componentMeta.Labels {
		// Exclude the label that could prevent opening the edit window through lens
		if key != "k8slens-edit-resource-version" {
			filteredLabels[key] = value
		}
	}

	spec := kedav1.ScaledObjectSpec{
		ScaleTargetRef:  &scaleTargetRef,
		MinReplicaCount: &minReplicas,
		MaxReplicaCount: &maxReplicas,
	}

	if kedaSpec != nil {
		spec.Triggers = kedaSpec.Triggers
		spec.Advanced = kedaSpec.Advanced
		spec.PollingInterval = kedaSpec.PollingInterval
		spec.CooldownPeriod = kedaSpec.CooldownPeriod
		spec.IdleReplicaCount = kedaSpec.IdleReplicaCount
		spec.Fallback = kedaSpec.Fallback
	}

	return &kedav1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:        utils.GetScaledObjectName(componentMeta.Name),
			Namespace:   componentMeta.Namespace,
			Labels:      filteredLabels,
			Annotations: componentMeta.Annotations,
			// Shared IR and typed Raw dispatch provide the controller owner
			// through componentMeta. The legacy constructor applies its owner
			// through SetControllerReferences.
			OwnerReferences: componentMeta.OwnerReferences,
		},
		Spec: spec,
	}
}

// createScaledObjectFromComponentExt is the Raw Deployment bridge from the
// (componentMeta, componentExt) caller to the parameterized createScaledObject.
// Reads Autoscaler.Keda when present (the canonical authoring location) and
// otherwise leaves Triggers empty. The lower-level Reconcile() gate
// defensively skips applying that invalid object.
func createScaledObjectFromComponentExt(
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
) *kedav1.ScaledObject {
	scaleTargetRef := kedav1.ScaleTarget{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       componentMeta.Name,
	}
	minReplicas := calculateMinReplicas(componentExt)
	maxReplicas := calculateMaxReplicas(componentExt, minReplicas)

	var kedaSpec *v1beta1.KedaAutoscaler
	if componentExt.Autoscaler != nil && componentExt.Autoscaler.Keda != nil {
		kedaSpec = componentExt.Autoscaler.Keda
	}

	return createScaledObject(componentMeta, scaleTargetRef, kedaSpec, minReplicas, maxReplicas)
}

// calculateMinReplicas calculates the minimum replicas
func calculateMinReplicas(componentExt *v1beta1.ComponentExtensionSpec) int32 {
	if componentExt.MinReplicas != nil && *componentExt.MinReplicas > 0 {
		return int32(*componentExt.MinReplicas)
	}
	return defaultMinReplicas
}

// calculateMaxReplicas calculates the maximum replicas
func calculateMaxReplicas(componentExt *v1beta1.ComponentExtensionSpec, minReplicas int32) int32 {
	if componentExt.MaxReplicas > int(minReplicas) {
		return int32(componentExt.MaxReplicas)
	}
	return minReplicas
}

// checkScaledObjectExist checks if the ScaledObject exists and determines the action
func (r *KEDAReconciler) checkScaledObjectExist() (constants.CheckResultType, *kedav1.ScaledObject, error) {
	existingScaledObject := &kedav1.ScaledObject{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: r.ScaledObject.Namespace,
		Name:      r.ScaledObject.Name,
	}, existingScaledObject)

	if err != nil {
		if apierr.IsNotFound(err) {
			if shouldCreateScaledObject(r.ScaledObject) {
				return constants.CheckResultCreate, nil, nil
			}
			return constants.CheckResultSkipped, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}
	if err := validateScaledObjectControllerOwnership(r.ScaledObject, existingScaledObject); err != nil {
		return constants.CheckResultUnknown, existingScaledObject, err
	}
	if existingScaledObject.DeletionTimestamp != nil {
		return constants.CheckResultUnknown, existingScaledObject, fmt.Errorf("ScaledObject %s/%s is terminating", existingScaledObject.Namespace, existingScaledObject.Name)
	}

	if semanticScaledObjectEquals(r.ScaledObject, existingScaledObject) {
		return constants.CheckResultExisted, existingScaledObject, nil
	}
	if shouldDeleteScaledObject(r.ScaledObject) {
		return constants.CheckResultDelete, existingScaledObject, nil
	}
	return constants.CheckResultUpdate, existingScaledObject, nil
}

func validateScaledObjectControllerOwnership(desired, existing *kedav1.ScaledObject) error {
	expected := metav1.GetControllerOf(desired)
	if expected == nil {
		return nil
	}
	actual := metav1.GetControllerOf(existing)
	if actual != nil && actual.UID == expected.UID {
		return nil
	}
	if actual == nil {
		return fmt.Errorf("ScaledObject %s/%s is not controlled by expected owner %s %s (UID %q): object has no controller owner", existing.Namespace, existing.Name, expected.Kind, expected.Name, expected.UID)
	}
	return fmt.Errorf("ScaledObject %s/%s is not controlled by expected owner %s %s (UID %q): controller is %s %s (UID %q)", existing.Namespace, existing.Name, expected.Kind, expected.Name, expected.UID, actual.Kind, actual.Name, actual.UID)
}

// semanticScaledObjectEquals checks the metadata, controller owner, and spec
// fields OME manages on the desired and existing ScaledObjects.
func semanticScaledObjectEquals(desired, existing *kedav1.ScaledObject) bool {
	if desired.Annotations[constants.AutoscalerClass] != existing.Annotations[constants.AutoscalerClass] {
		return false
	}
	controllerChanged := !equality.Semantic.DeepEqual(metav1.GetControllerOf(desired), metav1.GetControllerOf(existing))
	return equality.Semantic.DeepEqual(desired.Spec, existing.Spec) &&
		scalermetadata.Contains(existing.Labels, existing.Annotations, desired.Labels, desired.Annotations) &&
		!controllerChanged
}

func scaledObjectForUpdate(desired, existing *kedav1.ScaledObject) *kedav1.ScaledObject {
	updated := existing.DeepCopy()
	updated.Spec = desired.Spec
	updated.Labels, updated.Annotations = scalermetadata.Merge(
		existing.Labels,
		existing.Annotations,
		desired.Labels,
		desired.Annotations,
	)
	if _, present := desired.Annotations[constants.AutoscalerClass]; !present {
		delete(updated.Annotations, constants.AutoscalerClass)
	}
	updated.OwnerReferences = mergeScaledObjectOwnerReferences(existing.OwnerReferences, desired.OwnerReferences)
	return updated
}

func mergeScaledObjectOwnerReferences(existing, desired []metav1.OwnerReference) []metav1.OwnerReference {
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

// shouldDeleteScaledObject determines if the ScaledObject should be deleted
func shouldDeleteScaledObject(desired *kedav1.ScaledObject) bool {
	desiredAutoscalerClass := desired.Annotations[constants.AutoscalerClass]
	return constants.AutoscalerClassType(desiredAutoscalerClass) == constants.AutoscalerClassExternal
}

// shouldCreateScaledObject determines if the ScaledObject should be created.
// Require at least one trigger; otherwise skip apply to avoid creating an
// invalid (zero-trigger) ScaledObject.
func shouldCreateScaledObject(desired *kedav1.ScaledObject) bool {
	if len(desired.Spec.Triggers) == 0 {
		return false
	}
	desiredAutoscalerClass := desired.Annotations[constants.AutoscalerClass]
	return desiredAutoscalerClass == "" || constants.AutoscalerClassType(desiredAutoscalerClass) == constants.AutoscalerClassKEDA
}

// Reconcile reconciles the ScaledObject resource
func (r *KEDAReconciler) Reconcile() error {
	checkResult, existingScaledObject, err := r.checkScaledObjectExist()
	log.Info("Reconciling ScaledObject", "namespace", r.ScaledObject.Namespace, "name", r.ScaledObject.Name, "checkResult", checkResult.String())
	if err != nil {
		return err
	}

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.ScaledObject)
	case constants.CheckResultUpdate:
		r.ScaledObject = scaledObjectForUpdate(r.ScaledObject, existingScaledObject)
		opErr = r.client.Update(context.TODO(), r.ScaledObject)
	case constants.CheckResultDelete:
		opErr = r.client.Delete(context.TODO(), r.ScaledObject)
	default:
		return nil
	}

	if opErr != nil {
		log.Error(opErr, "Failed to reconcile ScaledObject", "namespace", r.ScaledObject.Namespace, "name", r.ScaledObject.Name)
		return opErr
	}

	return nil
}

// SetControllerReferences sets the owner reference for the ScaledObject
func (r *KEDAReconciler) SetControllerReferences(owner metav1.Object, scheme *runtime.Scheme) error {
	return controllerutil.SetControllerReference(owner, r.ScaledObject, scheme)
}
