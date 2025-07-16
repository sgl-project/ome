package multinodevllm

import (
	"fmt"
	"time"

	ray "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	knapis "knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/services"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/istiosidecar"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/service"
)

type MultiNodeVllmReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	Ray    *RayReconciler
	URL    *knapis.URL
	//TODO - Add other reconcilers such as ingress and autoscaling
	RawMultiNodeService *service.RayServiceReconciler
	MultiNodeProber     *MultiNodeProberReconciler
	IstioSidecar        *istiosidecar.IstioSidecarReconciler
}

func NewMultiNodeVllmReconciler(client client.Client,
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
	podSpec *corev1.PodSpec) (*MultiNodeVllmReconciler, error) {

	url, err := createRawURL(clientset, componentMeta)
	if err != nil {
		return nil, err
	}

	multinodeProberConfig, err := controllerconfig.NewMultiNodeProberConfig(clientset)
	if err != nil {
		return nil, err
	}

	var enabled bool
	istioSidecarInjection, ok := componentMeta.Labels[constants.IstioSidecarInjectionLabel]
	if ok && istioSidecarInjection == "true" {
		enabled = true
	}

	return &MultiNodeVllmReconciler{
		client:              client,
		scheme:              scheme,
		Ray:                 NewRayReconciler(client, scheme, componentMeta, componentExt, podSpec, time.Duration(multinodeProberConfig.UnavailableThresholdSeconds)*time.Second),
		MultiNodeProber:     NewMultiNodeProberReconciler(client, scheme, componentMeta, componentExt, multinodeProberConfig),
		RawMultiNodeService: service.NewRayServiceReconciler(client, scheme, componentMeta, podSpec),
		IstioSidecar:        istiosidecar.NewIstioSidecarReconciler(client, scheme, componentMeta, enabled),
		URL:                 url,
	}, nil
}

func createRawURL(clientset kubernetes.Interface, metadata metav1.ObjectMeta) (*knapis.URL, error) {
	ingressConfig, err := controllerconfig.NewIngressConfig(clientset)
	if err != nil {
		return nil, err
	}

	domainService := services.NewDomainService()
	url := &knapis.URL{}
	url.Scheme = "http"
	url.Host, err = domainService.GenerateDomainName(metadata.Name, metadata, ingressConfig)
	if err != nil {
		return nil, fmt.Errorf("failed creating host name: %w", err)
	}

	return url, nil
}

func (r *MultiNodeVllmReconciler) Reconcile() ([]*ray.RayCluster, ctrl.Result, error) {
	//reconcile Ray cluster
	rayclusters, rayResult, err := r.Ray.Reconcile()
	if err != nil {
		return nil, rayResult, err
	}

	// reconcile Raw service
	if r.RawMultiNodeService != nil {
		_, err := r.RawMultiNodeService.Reconcile()
		if err != nil {
			return nil, ctrl.Result{}, err
		}
	}
	// reconcile MultiNodeProber
	err = r.MultiNodeProber.Reconcile()
	if err != nil {
		return nil, ctrl.Result{}, err
	}

	// Reconcile Istio Sidecar Resource
	if _, err := r.IstioSidecar.Reconcile(); err != nil {
		return nil, ctrl.Result{}, err
	}

	return rayclusters, rayResult, nil
}
