package service

import (
	"context"

	"github.com/sgl-project/ome/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RayServiceReconciler reconciles Ray head Service objects
type RayServiceReconciler struct {
	client  client.Client
	scheme  *runtime.Scheme
	Service *corev1.Service
}

// NewRayServiceReconciler creates a new RayServiceReconciler instance
func NewRayServiceReconciler(client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	podSpec *corev1.PodSpec) *RayServiceReconciler {
	return &RayServiceReconciler{
		client:  client,
		scheme:  scheme,
		Service: buildRayHeadService(componentMeta, podSpec),
	}
}

// buildRayHeadService constructs a Ray head Service object from the given specifications
func buildRayHeadService(componentMeta metav1.ObjectMeta, podSpec *corev1.PodSpec) *corev1.Service {
	servicePorts := buildRayServicePorts(podSpec, componentMeta.Name)
	serviceType := determineServiceType(componentMeta)
	selector := buildRayHeadSelectorLabels(componentMeta)

	return buildServiceWithLoadBalancer(componentMeta, serviceType, servicePorts, selector)
}

// buildRayServicePorts creates service ports configuration for Ray head service
func buildRayServicePorts(podSpec *corev1.PodSpec, componentName string) []corev1.ServicePort {
	var servicePorts []corev1.ServicePort

	if len(podSpec.Containers) > 0 {
		container := podSpec.Containers[0]
		if len(container.Ports) > 0 {
			for _, port := range container.Ports {
				servicePorts = append(servicePorts, buildServicePort(port))
			}
		} else {
			servicePorts = append(servicePorts, buildDefaultServicePort(componentName))
		}
	}

	// Add Ray-specific default ports
	servicePorts = append(servicePorts, buildRayDefaultPorts()...)
	return servicePorts
}

// buildRayDefaultPorts creates default ports required for Ray head service
func buildRayDefaultPorts() []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name: "dashboard",
			Port: 8265,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 8265,
			},
		},
		{
			Name: "metrics",
			Port: 8000,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 8000,
			},
		},
		{
			Name: "redis",
			Port: 6379,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 6379,
			},
		},
	}
}

// buildRayHeadSelectorLabels creates selector labels for Ray head service
func buildRayHeadSelectorLabels(componentMeta metav1.ObjectMeta) map[string]string {
	return map[string]string{
		"app.kubernetes.io/created-by":  "kuberay-operator",
		"app.kubernetes.io/name":        "kuberay",
		"ray.io/node-type":              "head",
		constants.InferenceServiceLabel: componentMeta.Name,
	}
}

// Reconcile ensures the Ray head Service matches the desired state
func (r *RayServiceReconciler) Reconcile() (*corev1.Service, error) {
	checkResult, existingService, err := r.checkServiceState()
	log.Info("Reconcile ray service", "namespace", r.Service.Namespace, "name", r.Service.Name, "checkResult", checkResult)
	if err != nil {
		return nil, err
	}

	return r.handleReconcileAction(checkResult, existingService)
}

// checkServiceState checks the current state of the service
func (r *RayServiceReconciler) checkServiceState() (constants.CheckResultType, *corev1.Service, error) {
	existingService := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Namespace: r.Service.Namespace,
		Name:      r.Service.Name,
	}, existingService)

	if err != nil {
		if apierr.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	if semanticServiceEquals(r.Service, existingService) {
		return constants.CheckResultExisted, existingService, nil
	}
	return constants.CheckResultUpdate, existingService, nil
}

// handleReconcileAction performs the appropriate action based on the reconcile check result
func (r *RayServiceReconciler) handleReconcileAction(checkResult constants.CheckResultType, existingService *corev1.Service) (*corev1.Service, error) {
	ctx := context.TODO()

	switch checkResult {
	case constants.CheckResultCreate:
		if err := r.client.Create(ctx, r.Service); err != nil {
			return nil, err
		}
		return r.Service, nil
	case constants.CheckResultUpdate:
		if err := r.client.Update(ctx, r.Service); err != nil {
			return nil, err
		}
		return r.Service, nil
	default:
		return existingService, nil
	}
}
