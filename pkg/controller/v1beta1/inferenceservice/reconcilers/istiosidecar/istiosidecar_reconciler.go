package istiosidecar

import (
	"context"
	"strconv"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	istiov1beta1 "istio.io/api/networking/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var log = ctrl.Log.WithName("IstioSidecarReconciler")

type IstioSidecarReconciler struct {
	client  kclient.Client
	scheme  *runtime.Scheme
	Sidecar *istioclientv1beta1.Sidecar
	enabled bool
}

func NewIstioSidecarReconciler(client kclient.Client, scheme *runtime.Scheme, componentMeta metav1.ObjectMeta, enabled bool) *IstioSidecarReconciler {
	return &IstioSidecarReconciler{
		client:  client,
		scheme:  scheme,
		Sidecar: createSidecar(componentMeta),
		enabled: enabled,
	}
}

func createSidecar(componentMeta metav1.ObjectMeta) *istioclientv1beta1.Sidecar {
	portInt, _ := strconv.Atoi(constants.InferenceServiceDefaultHttpPort)
	return &istioclientv1beta1.Sidecar{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentMeta.Name,
			Namespace: componentMeta.Namespace,
		},
		Spec: istiov1beta1.Sidecar{
			Egress: []*istiov1beta1.IstioEgressListener{
				{
					Hosts: []string{"./*"}, // Handles traffic to any destination
					Port: &istiov1beta1.Port{
						Number:   uint32(portInt),
						Protocol: "HTTP",
					},
				},
			},
			Ingress: []*istiov1beta1.IstioIngressListener{
				{
					Port: &istiov1beta1.Port{
						Number:   uint32(portInt),
						Protocol: "HTTP",
					},
				},
			},
			WorkloadSelector: &istiov1beta1.WorkloadSelector{
				Labels: map[string]string{
					constants.InferenceServiceLabel: componentMeta.Name,
				},
			},
		},
	}
}

func (r *IstioSidecarReconciler) checkSidecarExist(client kclient.Client) (constants.CheckResultType, *istioclientv1beta1.Sidecar, error) {
	existing := &istioclientv1beta1.Sidecar{}
	// Use client to get the Sidecar resource from the cluster
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: r.Sidecar.Namespace,
		Name:      r.Sidecar.Name,
	}, existing)
	if err != nil {
		if apierr.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}
	return constants.CheckResultExisted, existing, nil
}

func (r *IstioSidecarReconciler) Reconcile() (*istioclientv1beta1.Sidecar, error) {
	// Check if the Sidecar resource exists
	if !r.enabled {
		return nil, nil
	}
	result, existing, err := r.checkSidecarExist(r.client)
	log.Info("Istio Sidecar reconcile", "checkResult", result, "err", err)
	if err != nil {
		return nil, err
	}
	var opErr error
	switch result {
	case constants.CheckResultCreate:
		opErr = r.client.Create(context.TODO(), r.Sidecar)
	case constants.CheckResultUpdate:
		opErr = r.client.Update(context.TODO(), r.Sidecar)
	default:
		return existing, nil
	}
	if opErr != nil {
		return nil, opErr
	}
	return r.Sidecar, nil
}
