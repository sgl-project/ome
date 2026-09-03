// Package autoscalerpolicy validates AutoscalerPolicy writes and guards
// against in-use deletion.
//
// CREATE/UPDATE run the full cluster-state-free spec validation (structure,
// reserved shapes, template allowlist, sample render, PromQL parse) so an
// invalid policy never lands where a consumer could reference it. DELETE is
// denied while any InferenceService component in the namespace references
// the policy: a webhook refusal is immediate and names its reason, whereas a
// finalizer would accept the delete and park the object in Terminating,
// wedging GitOps prune and teaching operators to strip finalizers.
package autoscalerpolicy

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/autoscaler"
	"sigs.k8s.io/ome/pkg/validation"
)

var log = logf.Log.WithName("autoscalerpolicy-validation-webhook")

// maxNamedReferencingComponents caps how many referencing components the
// in-use deletion error enumerates. The denial itself covers every
// consumer; the cap only keeps the message readable when a policy serves a
// large fleet.
const maxNamedReferencingComponents = 10

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-ome-io-v1beta1-autoscalerpolicy,mutating=false,failurePolicy=fail,groups=ome.io,resources=autoscalerpolicies,versions=v1beta1,name=autoscalerpolicy.ome-webhook-server.validator,sideEffects=None,admissionReviewVersions=v1

// +kubebuilder:object:generate=false

// Validator validates AutoscalerPolicy admission requests. Admission is
// UX, not the invariant: break-glass, version skew, and webhook-outage
// windows can still remove a referenced policy, and the consumer-side
// fail-closed freeze covers those — this handler exists so the ordinary
// path fails fast with a named reason instead.
type Validator struct {
	Client  client.Reader
	Decoder admission.Decoder
}

func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Operation {
	case admissionv1.Create, admissionv1.Update:
		return v.validateWrite(req)
	case admissionv1.Delete:
		return v.validateDelete(ctx, req)
	default:
		return admission.Allowed("")
	}
}

// validateWrite rejects a CREATE/UPDATE carrying any spec issue. All
// findings are joined into one denial so the operator sees the full list
// in a single rejected write.
func (v *Validator) validateWrite(req admission.Request) admission.Response {
	policy := &v1beta1.AutoscalerPolicy{}
	if err := v.Decoder.Decode(req, policy); err != nil {
		log.Error(err, "Failed to decode AutoscalerPolicy", "name", req.Name, "namespace", req.Namespace)
		return admission.Errored(http.StatusBadRequest, err)
	}
	if err := validation.ValidateAutoscalerPolicySpec(&policy.Spec); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("")
}

// validateDelete denies deletion while any InferenceService component in
// the namespace references the policy. Exemptions, each of which admits:
// zero references; the break-glass annotation (set by a prior, reviewable
// update); a terminating or absent namespace (teardown deletes objects in
// arbitrary order and must not wedge); the policy itself already gone.
func (v *Validator) validateDelete(ctx context.Context, req admission.Request) admission.Response {
	policy, earlyResponse := v.deletedPolicy(ctx, req)
	if earlyResponse != nil {
		return *earlyResponse
	}

	if policy.Annotations[constants.AutoscalerPolicyAllowInUseDelete] == "true" {
		return admission.Allowed(fmt.Sprintf("in-use deletion allowed by the %s annotation", constants.AutoscalerPolicyAllowInUseDelete))
	}

	namespace := &corev1.Namespace{}
	if err := v.Client.Get(ctx, types.NamespacedName{Name: policy.Namespace}, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			return admission.Allowed("namespace is gone; nothing can reference the policy")
		}
		log.Error(err, "Failed to get namespace for AutoscalerPolicy deletion", "name", policy.Name, "namespace", policy.Namespace)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if namespace.Status.Phase == corev1.NamespaceTerminating || !namespace.DeletionTimestamp.IsZero() {
		return admission.Allowed("namespace is terminating; teardown deletes objects in arbitrary order")
	}

	referencing, err := v.referencingComponents(ctx, policy)
	if err != nil {
		log.Error(err, "Failed to list InferenceServices for AutoscalerPolicy deletion", "name", policy.Name, "namespace", policy.Namespace)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if len(referencing) == 0 {
		return admission.Allowed("")
	}

	named := referencing
	overflow := ""
	if len(referencing) > maxNamedReferencingComponents {
		named = referencing[:maxNamedReferencingComponents]
		overflow = fmt.Sprintf(" and %d more", len(referencing)-maxNamedReferencingComponents)
	}
	return admission.Denied(fmt.Sprintf(
		"AutoscalerPolicy %q is referenced by %d component(s): %s%s; remove the refs first, or set the %s=%q annotation to force deletion",
		policy.Name, len(referencing), strings.Join(named, ", "), overflow,
		constants.AutoscalerPolicyAllowInUseDelete, "true",
	))
}

// deletedPolicy materializes the policy under deletion. The API server
// usually sends the object in OldObject, but a DELETE request may arrive
// with an empty payload — then the policy is fetched by name, and a
// NotFound admits outright (deleting an already-gone object must never
// wedge). A non-nil response short-circuits the caller.
func (v *Validator) deletedPolicy(ctx context.Context, req admission.Request) (*v1beta1.AutoscalerPolicy, *admission.Response) {
	policy := &v1beta1.AutoscalerPolicy{}
	if len(req.OldObject.Raw) > 0 {
		if err := v.Decoder.DecodeRaw(req.OldObject, policy); err != nil {
			log.Error(err, "Failed to decode AutoscalerPolicy under deletion", "name", req.Name, "namespace", req.Namespace)
			response := admission.Errored(http.StatusBadRequest, err)
			return nil, &response
		}
	} else {
		if err := v.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, policy); err != nil {
			if apierrors.IsNotFound(err) {
				response := admission.Allowed("policy is already gone")
				return nil, &response
			}
			log.Error(err, "Failed to get AutoscalerPolicy under deletion", "name", req.Name, "namespace", req.Namespace)
			response := admission.Errored(http.StatusInternalServerError, err)
			return nil, &response
		}
	}
	// The request coordinates are authoritative; a decoded object may lack
	// them when the payload carries no metadata.
	if policy.Name == "" {
		policy.Name = req.Name
	}
	if policy.Namespace == "" {
		policy.Namespace = req.Namespace
	}
	return policy, nil
}

// referencingComponents lists every same-namespace InferenceService
// component whose autoscalerPolicyRef names this policy, as
// "<isvc>/<component>" in stable order (ISVCs in list order, components
// engine, decoder, router).
func (v *Validator) referencingComponents(ctx context.Context, policy *v1beta1.AutoscalerPolicy) ([]string, error) {
	consumers := &v1beta1.InferenceServiceList{}
	if err := v.Client.List(ctx, consumers, client.InNamespace(policy.Namespace)); err != nil {
		return nil, err
	}
	var referencing []string
	for i := range consumers.Items {
		isvc := &consumers.Items[i]
		for _, componentType := range []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent, v1beta1.RouterComponent} {
			ref := autoscaler.ComponentPolicyRef(isvc, componentType)
			if ref == nil || ref.Name != policy.Name {
				continue
			}
			// A reserved cluster-scoped kind cannot name this namespaced
			// policy; only the namespaced kind (or its empty default) counts.
			if ref.Kind != "" && ref.Kind != constants.AutoscalerPolicyKind {
				continue
			}
			referencing = append(referencing, fmt.Sprintf("%s/%s", isvc.Name, componentType))
		}
	}
	return referencing, nil
}
