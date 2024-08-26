package multinodevllm

import (
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress"
	raycluster "bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ray"
	"fmt"
	ray "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	"k8s.io/client-go/kubernetes"
	knapis "knative.dev/pkg/apis"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	service "bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/service"
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

	return &MultiNodeVllmReconciler{
		client:              client,
		scheme:              scheme,
		Ray:                 raycluster.NewRayReconciler(client, scheme, componentMeta, componentExt, podSpec),
		MultiNodeProber:     raycluster.NewMultiNodeProberReconciler(client, scheme, componentMeta, multinodeProberConfig, url),
		RawMultiNodeService: service.NewRayServiceReconciler(client, scheme, componentMeta, podSpec),
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

func (r *MultiNodeVllmReconciler) Reconcile() ([]*ray.RayCluster, error) {
	//reconcile Ray cluster
	rayCluster, err := r.Ray.Reconcile()
	if err != nil {
		return nil, err
	}
	// reconcile Raw service
	if r.RawMultiNodeService != nil {
		_, err := r.RawMultiNodeService.Reconcile()
		if err != nil {
			return nil, err
		}
	}
	// reconcile MultiNodeProber
	_, err = r.MultiNodeProber.Reconcile()
	if err != nil {
		return nil, err
	}
	return rayCluster, nil
}
