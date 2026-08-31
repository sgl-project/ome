package coordination

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// servingPortName is the container-port NAME a runtime uses to mark its
// customer-traffic listener. It is a Kubernetes naming convention, not a
// behavioral value: it selects among the ports a runtime already declares
// and never contributes a port number of its own. A runner that declares
// no port under this name falls through to its first declared port.
const servingPortName = "http"

// ErrNoServingPort reports that a Component's effective serving template has
// no container port, so no per-revision routing Service can be built. A
// ClusterIP Service must publish at least one port, and publishing a port
// the runner does not listen on produces a Service whose EndpointSlice
// reads ready while every request is dropped — pod readiness is a property
// of the pod, not of the published port. Callers skip the routing Service
// instead of inventing a number.
var ErrNoServingPort = errors.New("component serving template declares no container port")

// isvcGroupVersionKind identifies the InferenceService owner reference
// on each per-revision Service. Declared locally so this subpackage
// stays importable from either the top-level reconciler or its caller
// without closing an import cycle.
var isvcGroupVersionKind = v1beta1.SchemeGroupVersion.WithKind("InferenceService")

// PerRevisionService describes the routing + headless pair OMENative creates
// per (component, revisionHash). Both share the revision selector; routing may
// add leader and ordinal filters.
type PerRevisionService struct {
	// RoutingName is the ClusterIP Service name —
	// `<isvc>-<component>-rev-<hash>`. Used as the backend target by
	// the HTTPRoute weighted-backendRef consumer. Empty when
	// EnsurePerRevisionServices skipped the routing Service because the
	// Component's serving template declares no container port.
	RoutingName string

	// HeadlessName is the headless Service name —
	// `<isvc>-<component>-rev-<hash>-headless`. Used for pod-level
	// DNS within the revision (multi-node tensor-parallel groups need
	// per-pod addressability among one revision's pods).
	HeadlessName string
}

// RevisionRoutingSelector controls the optional pod filters on one revision's
// routing Service. PodOrdinal is applied only when LeaderOnly is also true.
type RevisionRoutingSelector struct {
	LeaderOnly bool
	PodOrdinal bool
}

// PerRevisionServiceName returns the per-revision Service name for one
// (component, revisionHash) pair. Format: `<isvc>-<component>-rev-<hash>`,
// bounded to the DNS1035 label limit. Delegates to the workload/query
// copy so the coordination-created name and the dispatch-path lookup
// (workload/ops) are always byte-identical.
func PerRevisionServiceName(isvcName string, component v1beta1.ComponentType, revisionHash string) string {
	return query.PerRevisionServiceName(isvcName, workload.ComponentType(component), revisionHash)
}

// PerRevisionHeadlessServiceName returns the per-revision headless
// Service name. Format: `<isvc>-<component>-rev-<hash>-headless`, bounded
// to the DNS1035 label limit.
func PerRevisionHeadlessServiceName(isvcName string, component v1beta1.ComponentType, revisionHash string) string {
	return query.PerRevisionHeadlessServiceName(isvcName, workload.ComponentType(component), revisionHash)
}

// PerRevisionServiceNames returns the routing + headless names for a
// (component, revisionHash) pair.
func PerRevisionServiceNames(isvcName string, component v1beta1.ComponentType, revisionHash string) PerRevisionService {
	return PerRevisionService{
		RoutingName:  PerRevisionServiceName(isvcName, component, revisionHash),
		HeadlessName: PerRevisionHeadlessServiceName(isvcName, component, revisionHash),
	}
}

// BuildPerRevisionRoutingService produces the ClusterIP per-revision
// Service for one (component, revisionHash). Pure compute; idempotent.
//
// `runnerPorts` are the effective serving container ports; the published
// port is resolved from them (see
// perRevisionRoutingPorts). Returns ErrNoServingPort when the set is
// empty — the Service is not buildable without a port the pods listen on.
//
// `routing` may restrict the selector to leader pods and, when supported by
// the observed revision, leader ordinal 0. Revisions without ordinal labels
// retain a reachable leader-only selector.
//
// The routing Service's label map intentionally excludes optional runner and
// ordinal filters. Labels describe the Service; the Selector gates pod membership.
func BuildPerRevisionRoutingService(isvc *v1beta1.InferenceService, component v1beta1.ComponentType, revisionHash string, routing RevisionRoutingSelector, runnerPorts []corev1.ContainerPort) (*corev1.Service, error) {
	if isvc == nil {
		return nil, fmt.Errorf("nil InferenceService")
	}
	if revisionHash == "" {
		return nil, fmt.Errorf("empty revisionHash")
	}
	ports, err := perRevisionRoutingPorts(runnerPorts)
	if err != nil {
		return nil, err
	}
	labels := perRevisionServiceSelector(isvc.Name, component, revisionHash)
	selector := labels
	if routing.LeaderOnly {
		selector = make(map[string]string, len(labels)+2)
		for k, v := range labels {
			selector[k] = v
		}
		selector[query.LabelRunner] = string(v1beta1.RunnerNameLeader)
		if routing.PodOrdinal {
			// Ordinals are numbered per runner, so runner=leader is required
			// alongside ordinal 0 to exclude worker ordinal 0.
			selector[query.LabelPodOrdinal] = "0"
		}
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PerRevisionServiceName(isvc.Name, component, revisionHash),
			Namespace: isvc.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(isvc, isvcGroupVersionKind),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeClusterIP,
			Selector:                 selector,
			PublishNotReadyAddresses: false,
			Ports:                    ports,
		},
	}, nil
}

// perRevisionRoutingPorts maps a Component's effective serving container ports
// onto the single ServicePort the routing Service publishes. HTTPRoute
// backendRef consumers route to that port, so it must be the one the
// runner listens on. Runtimes routinely declare several ports (serving,
// distributed-init rendezvous, PD bootstrap, metrics); the customer-traffic
// Service publishes only the serving one — the port named `http`, else the
// first declared port.
//
// TargetPort follows the container port's NAME when it has one. Kubernetes
// resolves a named targetPort per endpoint pod, so a revision whose pods
// were rendered from an older spec still receives traffic on the port that
// revision actually opened; a numeric targetPort would pin every revision
// to the current spec's number. Unnamed ports fall back to the number.
//
// ClusterIP Services require at least one port (k8s validation), so an
// empty runner-port set yields ErrNoServingPort. The headless variant
// takes no ports at all — peer DNS resolves pod IPs directly.
func perRevisionRoutingPorts(runnerPorts []corev1.ContainerPort) ([]corev1.ServicePort, error) {
	serving, ok := servingContainerPort(runnerPorts)
	if !ok {
		return nil, ErrNoServingPort
	}
	protocol := serving.Protocol
	if protocol == "" {
		protocol = corev1.ProtocolTCP
	}
	target := intstr.FromInt32(serving.ContainerPort)
	if serving.Name != "" {
		target = intstr.FromString(serving.Name)
	}
	return []corev1.ServicePort{{
		Name:       serving.Name,
		Port:       serving.ContainerPort,
		TargetPort: target,
		Protocol:   protocol,
	}}, nil
}

// servingContainerPort picks the customer-traffic port out of a runner's
// declared container ports: the one named `http`, else the first declared.
// Reports false when nothing is declared.
func servingContainerPort(ports []corev1.ContainerPort) (corev1.ContainerPort, bool) {
	if len(ports) == 0 {
		return corev1.ContainerPort{}, false
	}
	for _, p := range ports {
		if p.Name == servingPortName {
			return p, true
		}
	}
	return ports[0], true
}

// BuildPerRevisionHeadlessService produces the headless per-revision
// Service. Like the routing variant but with ClusterIP=None and
// PublishNotReadyAddresses=true so peer DNS resolves during gang init.
//
// Headless Service intentionally selects ALL pods of the revision —
// including workers — regardless of multi-pod-ness. Peer discovery
// for distributed init (NCCL all-gather, vLLM ray-init, etc.)
// requires every pod to be addressable via per-pod DNS. Only the
// per-revision routing Service adds leader and ordinal filters.
func BuildPerRevisionHeadlessService(isvc *v1beta1.InferenceService, component v1beta1.ComponentType, revisionHash string) (*corev1.Service, error) {
	if isvc == nil {
		return nil, fmt.Errorf("nil InferenceService")
	}
	if revisionHash == "" {
		return nil, fmt.Errorf("empty revisionHash")
	}
	selector := perRevisionServiceSelector(isvc.Name, component, revisionHash)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PerRevisionHeadlessServiceName(isvc.Name, component, revisionHash),
			Namespace: isvc.Namespace,
			Labels:    selector,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(isvc, isvcGroupVersionKind),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeClusterIP,
			ClusterIP:                corev1.ClusterIPNone,
			Selector:                 selector,
			PublishNotReadyAddresses: true,
		},
	}, nil
}

// perRevisionServiceSelector is the label set per-revision Services
// use as their .spec.selector. The revision-hash label narrows the
// selector to one ControllerRevision's pods; the OMENative-managed
// label keeps the selector from picking up legacy pods on the
// same Component.
func perRevisionServiceSelector(isvcName string, component v1beta1.ComponentType, revisionHash string) map[string]string {
	return map[string]string{
		constants.InferenceServicePodLabelKey: isvcName,
		constants.OMEComponentLabel:           string(component),
		query.LabelRevisionHash:               revisionHash,
		query.LabelManagedBy:                  query.ManagedByOMENative,
	}
}

// EnsurePerRevisionServices ensures both the routing and headless
// Services exist for one (component, revisionHash). Idempotent and
// drift-correcting on the selector / labels / owner reference. Returns
// the names so the coordination layer can record them on
// Status.Components.<c>.Traffic[].
//
// `routing` controls the optional leader and ordinal filters. The headless
// Service is unaffected because workers must stay discoverable for peer DNS.
//
// `runnerPorts` are the Component's effective serving container ports. When
// they are empty the routing Service is skipped and logged rather than
// published on a guessed port: a routing Service on a port nothing listens
// on is indistinguishable from a healthy one (its EndpointSlice still reads
// ready) and silently drops every request. The headless Service is still
// ensured — peer DNS for distributed init does not depend on the serving
// port. The reconcile does not fail: coordination is a producer layer, and
// one Component's missing port declaration must not wedge the rest of the
// InferenceService.
func EnsurePerRevisionServices(ctx context.Context, c client.Client, isvc *v1beta1.InferenceService, component v1beta1.ComponentType, revisionHash string, routing RevisionRoutingSelector, runnerPorts []corev1.ContainerPort) (PerRevisionService, error) {
	out := PerRevisionServiceNames(isvc.Name, component, revisionHash)
	if c == nil {
		return out, fmt.Errorf("EnsurePerRevisionServices: nil client")
	}

	routingBuild := func(i *v1beta1.InferenceService, comp v1beta1.ComponentType, hash string) (*corev1.Service, error) {
		return BuildPerRevisionRoutingService(i, comp, hash, routing, runnerPorts)
	}
	switch err := ensureService(ctx, c, isvc, routingBuild, component, revisionHash, out.RoutingName, false); {
	case errors.Is(err, ErrNoServingPort):
		out.RoutingName = ""
		log.FromContext(ctx).Info("Skipping per-revision routing Service: component serving template declares no container port",
			"component", component, "revisionHash", revisionHash, "service", PerRevisionServiceName(isvc.Name, component, revisionHash))
	case err != nil:
		return out, err
	}
	if err := ensureService(ctx, c, isvc, BuildPerRevisionHeadlessService, component, revisionHash, out.HeadlessName, true); err != nil {
		return out, err
	}
	return out, nil
}

// ensureService is the shared CreateOrUpdate helper for the routing /
// headless pair. headless distinguishes the ClusterIP semantics so the
// patch back to the desired shape doesn't try to flip an immutable
// ClusterIP after create.
func ensureService(
	ctx context.Context,
	c client.Client,
	isvc *v1beta1.InferenceService,
	build func(*v1beta1.InferenceService, v1beta1.ComponentType, string) (*corev1.Service, error),
	component v1beta1.ComponentType,
	revisionHash, name string,
	headless bool,
) error {
	desired, err := build(isvc, component, revisionHash)
	if err != nil {
		return fmt.Errorf("build per-revision service %s: %w", name, err)
	}
	target := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desired.Name,
			Namespace: desired.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, c, target, func() error {
		if target.CreationTimestamp.IsZero() {
			if headless {
				target.Spec.ClusterIP = corev1.ClusterIPNone
			}
			target.Spec.Type = desired.Spec.Type
		}
		target.Spec.PublishNotReadyAddresses = desired.Spec.PublishNotReadyAddresses
		target.Spec.Selector = desired.Spec.Selector
		target.Spec.Ports = desired.Spec.Ports
		target.Labels = desired.Labels
		target.OwnerReferences = desired.OwnerReferences
		return nil
	})
	if err != nil {
		return fmt.Errorf("CreateOrUpdate per-revision service %s/%s: %w", target.Namespace, target.Name, err)
	}
	return nil
}

// isNamespaceTerminating reports whether err is the apiserver's rejection of a
// create because the target namespace is being deleted. Nothing new can be
// created in a terminating namespace, and the ISVC is going away with it, so
// coordination treats this as a benign stop rather than a retryable error.
func isNamespaceTerminating(err error) bool {
	return apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause)
}

// GCPerRevisionServices deletes both per-revision Services for one
// (component, revisionHash). Called when the revision is past
// retention AND no live pod carries the hash. NotFound is treated as
// success — the caller may invoke GC on stale entries.
func GCPerRevisionServices(ctx context.Context, c client.Client, namespace, isvcName string, component v1beta1.ComponentType, revisionHash string) error {
	if c == nil {
		return fmt.Errorf("GCPerRevisionServices: nil client")
	}
	names := PerRevisionServiceNames(isvcName, component, revisionHash)
	for _, name := range []string{names.RoutingName, names.HeadlessName} {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
		if err := c.Delete(ctx, svc); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete per-revision service %s/%s: %w", namespace, name, err)
		}
	}
	return nil
}
