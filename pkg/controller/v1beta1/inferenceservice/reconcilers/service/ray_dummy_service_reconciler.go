package service

import (
	"context"
	"fmt"
	"strconv"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceReconciler is the struct of Raw K8S Object
type RayDummyServiceReconciler struct {
	client       client.Client
	scheme       *runtime.Scheme
	Service      *corev1.Service
	componentExt *v1beta1.ComponentExtensionSpec
}

func NewRayDummyServiceReconciler(client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
	podSpec *corev1.PodSpec) *RayDummyServiceReconciler {
	return &RayDummyServiceReconciler {
		client:       client,
		scheme:       scheme,
		Service:      createRayDummyService(componentMeta, componentExt, podSpec),
		componentExt: componentExt,
	}
}

func createRayDummyService(componentMeta metav1.ObjectMeta, componentExt *v1beta1.ComponentExtensionSpec,
	podSpec *corev1.PodSpec) *corev1.Service {
	var servicePorts []corev1.ServicePort
	if len(podSpec.Containers) != 0 {
		container := podSpec.Containers[0]
		if len(container.Ports) > 0 {
			for _, port := range container.Ports {
				var servicePort corev1.ServicePort
				if port.Protocol == "" {
					port.Protocol = corev1.ProtocolTCP
				}
				servicePort = corev1.ServicePort{
					Name: port.Name,
					Port: port.ContainerPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: port.ContainerPort,
					},
					Protocol: corev1.ProtocolTCP,
				}
				servicePorts = append(servicePorts, servicePort)
			}
		} else {
			port, _ := strconv.Atoi(constants.InferenceServiceDefaultHttpPort)
			servicePorts = append(servicePorts, corev1.ServicePort{
				Name: componentMeta.Name,
				Port: 8080,
				TargetPort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: int32(port), // #nosec G109
				},
				Protocol: corev1.ProtocolTCP,
			})
		}
	}

	service := &corev1.Service{
		ObjectMeta: componentMeta,
		Spec: corev1.ServiceSpec{
			Selector: getRayDummySelectorLabels(componentMeta),
			Ports: servicePorts,
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	
	return service
}

func getRayDummySelectorLabels(componentMeta metav1.ObjectMeta) map[string]string {
	headLabel := fmt.Sprintf("%s-head", componentMeta.Name)
	if len(headLabel) > 63 {
		headLabel = headLabel[len(headLabel)-63:]
	}

	labels := map[string]string {
		"app.kubernetes.io/created-by": "kuberay-operator",
		"app.kubernetes.io/name": "kuberay",
		"ray.io/node-type": "head",
		"ray.io/cluster": componentMeta.Name,
		"ray.io/identifier": headLabel,
	}

	return labels
}

// checkServiceExist checks if the service exists?
func (r *RayDummyServiceReconciler) checkServiceExist(client client.Client) (constants.CheckResultType, *corev1.Service, error) {
	// get service
	existingService := &corev1.Service{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: r.Service.Namespace,
		Name:      r.Service.Name,
	}, existingService)
	if err != nil {
		if apierr.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}

	// existed, check equivalent
	if semanticServiceEquals(r.Service, existingService) {
		return constants.CheckResultExisted, existingService, nil
	}
	return constants.CheckResultUpdate, existingService, nil
}

// Reconcile ...
func (r *RayDummyServiceReconciler) Reconcile() (*corev1.Service, error) {
	// reconcile Service
	checkResult, existingService, err := r.checkServiceExist(r.client)
	log.Info("service reconcile", "checkResult", checkResult, "err", err)
	if err != nil {
		return nil, err
	}

	var opErr error
	switch checkResult {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.Service)
	case constants.CheckResultUpdate:
		opErr = r.client.Update(context.TODO(), r.Service)
	default:
		return existingService, nil
	}

	if opErr != nil {
		return nil, opErr
	}

	return r.Service, nil
}