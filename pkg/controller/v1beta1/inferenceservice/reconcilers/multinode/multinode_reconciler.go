package multinode

import (
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/services"
	raycluster "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/istiosidecar"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/lws"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/service"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	knapis "knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	lwsSpec "sigs.k8s.io/lws/api/leaderworkerset/v1"
)

type MultiNodeReconciler struct {
	client       client.Client
	scheme       *runtime.Scheme
	LWS          *lws.LWSReconciler
	URL          *knapis.URL
	IstioSidecar *raycluster.IstioSidecarReconciler
	Service      *service.ServiceReconciler
}

func NewMultiNodeReconciler(client client.Client,
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
	headPodSpec *corev1.PodSpec,
	workerSize int,
	workerPodSpec *corev1.PodSpec) (*MultiNodeReconciler, error) {

	url, err := createRawURL(clientset, componentMeta)
	if err != nil {
		return nil, err
	}
	var enabled bool
	istioSidecarInjection, ok := componentMeta.Labels[constants.IstioSidecarInjectionLabel]
	if ok && istioSidecarInjection == "true" {
		enabled = true
	}
	selector := map[string]string{"ray.io/node-type": "head"}

	return &MultiNodeReconciler{
		client:       client,
		scheme:       scheme,
		LWS:          lws.NewLWSReconciler(client, scheme, headPodSpec, workerPodSpec, int32(workerSize), componentExt, componentMeta),
		URL:          url,
		IstioSidecar: raycluster.NewIstioSidecarReconciler(client, scheme, componentMeta, enabled),
		Service:      service.NewServiceReconciler(client, scheme, componentMeta, componentExt, headPodSpec, selector),
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

func (r *MultiNodeReconciler) Reconcile() (*lwsSpec.LeaderWorkerSet, error) {
	existingLWS, err := r.LWS.Reconcile()
	if err != nil {
		return nil, err
	}
	if _, err := r.IstioSidecar.Reconcile(); err != nil {
		return nil, err
	}
	if _, err := r.Service.Reconcile(); err != nil {
		return nil, err
	}
	return existingLWS, nil
}
