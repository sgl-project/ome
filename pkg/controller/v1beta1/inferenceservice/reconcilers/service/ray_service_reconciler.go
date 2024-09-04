package service

import (
	"context"
	"fmt"
	"strconv"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RayServiceReconciler is the struct of Raw K8S Object
type RayServiceReconciler struct {
	client  client.Client
	scheme  *runtime.Scheme
	Service *corev1.Service
}

func NewRayServiceReconciler(client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	podSpec *corev1.PodSpec) *RayServiceReconciler {
	return &RayServiceReconciler{
		client:  client,
		scheme:  scheme,
		Service: createRayHeadService(componentMeta, podSpec),
	}
}

func createRayHeadService(componentMeta metav1.ObjectMeta, podSpec *corev1.PodSpec) *corev1.Service {
	servicePorts := createServicePorts(podSpec, componentMeta.Name)
	addDefaultServicePorts(&servicePorts)

	service := &corev1.Service{
		ObjectMeta: componentMeta,
		Spec: corev1.ServiceSpec{
			Selector: getRayHeadSelectorLabels(componentMeta),
			Ports:    servicePorts,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	return service
}

func createServicePorts(podSpec *corev1.PodSpec, componentName string) []corev1.ServicePort {
	var servicePorts []corev1.ServicePort

	if len(podSpec.Containers) == 0 {
		return servicePorts
	}

	container := podSpec.Containers[0]
	if len(container.Ports) > 0 {
		for _, port := range container.Ports {
			servicePort := corev1.ServicePort{
				Name: port.Name,
				Port: port.ContainerPort,
				TargetPort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: port.ContainerPort,
				},
				Protocol: getProtocol(port.Protocol),
			}
			servicePorts = append(servicePorts, servicePort)
		}
	} else {
		defaultPort := createDefaultServicePort(componentName)
		servicePorts = append(servicePorts, defaultPort)
	}

	return servicePorts
}

func getProtocol(protocol corev1.Protocol) corev1.Protocol {
	if protocol == "" {
		return corev1.ProtocolTCP
	}
	return protocol
}

func createDefaultServicePort(name string) corev1.ServicePort {
	port, _ := strconv.Atoi(constants.InferenceServiceDefaultHttpPort)
	return corev1.ServicePort{
		Name: name,
		Port: 8080,
		TargetPort: intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: int32(port), // #nosec G109
		},
		Protocol: corev1.ProtocolTCP,
	}
}

func addDefaultServicePorts(servicePorts *[]corev1.ServicePort) {
	defaultPorts := []corev1.ServicePort{
		{
			Name: "dashboard", Port: 8265,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 8265,
			},
		},
		{
			Name: "metrics", Port: 8000,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 8000,
			},
		},
		{
			Name: "redis", Port: 6379,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 6379,
			},
		},
	}

	*servicePorts = append(*servicePorts, defaultPorts...)
}

func getRayHeadSelectorLabels(componentMeta metav1.ObjectMeta) map[string]string {
	headLabel := fmt.Sprintf("%s-head", componentMeta.Name)
	if len(headLabel) > 63 {
		headLabel = headLabel[len(headLabel)-63:]
	}

	labels := map[string]string{
		"app.kubernetes.io/created-by":  "kuberay-operator",
		"app.kubernetes.io/name":        "kuberay",
		"ray.io/node-type":              "head",
		constants.InferenceServiceLabel: componentMeta.Name,
	}

	return labels
}

// checkServiceExist checks if the service exists?
func (r *RayServiceReconciler) checkServiceExist(client client.Client) (constants.CheckResultType, *corev1.Service, error) {
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
func (r *RayServiceReconciler) Reconcile() (*corev1.Service, error) {
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
