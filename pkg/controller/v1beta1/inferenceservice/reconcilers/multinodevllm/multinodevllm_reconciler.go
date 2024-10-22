package multinodevllm

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress"
	raycluster "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ray"
	"fmt"
	ray "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"k8s.io/client-go/kubernetes"
	knapis "knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/service"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MultiNodeVllmReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	Ray    *raycluster.RayReconciler
	URL    *knapis.URL
	//TODO - Add other reconcilers such as ingress and autoscaling
	RawMultiNodeService *service.RayServiceReconciler
	MultiNodeProber     *raycluster.MultiNodeProberReconciler
	IstioSidecar        *raycluster.IstioSidecarReconciler
	componentExt        *v1beta1.ComponentExtensionSpec
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

	multinodeProberConfig, err := v1beta1.NewMultiNodeProberConfig(clientset)
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
		Ray:                 raycluster.NewRayReconciler(client, scheme, componentMeta, componentExt, podSpec, time.Duration(multinodeProberConfig.UnavailableThresholdSeconds)*time.Second),
		MultiNodeProber:     raycluster.NewMultiNodeProberReconciler(client, scheme, componentMeta, componentExt, multinodeProberConfig),
		RawMultiNodeService: service.NewRayServiceReconciler(client, scheme, componentMeta, podSpec),
		IstioSidecar:        raycluster.NewIstioSidecarReconciler(client, scheme, componentMeta, enabled),
		URL:                 url,
	}, nil
}

func createRawURL(clientset kubernetes.Interface, metadata metav1.ObjectMeta) (*knapis.URL, error) {
	ingressConfig, err := v1beta1.NewIngressConfig(clientset)
	if err != nil {
		return nil, err
	}

	url := &knapis.URL{}
	url.Scheme = "http"
	url.Host, err = ingress.GenerateDomainName(metadata.Name, metadata, ingressConfig)
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
