package multinode

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	knapis "knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	lwsSpec "sigs.k8s.io/lws/api/leaderworkerset/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/services"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/lws"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/podmonitor"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/service"
)

type MultiNodeReconciler struct {
	client     client.Client
	scheme     *runtime.Scheme
	LWS        *lws.LWSReconciler
	URL        *knapis.URL
	Service    *service.ServiceReconciler
	PodMonitor *podmonitor.PodMonitorReconciler
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
	selector := map[string]string{lwsSpec.WorkerIndexLabelKey: "0"}

	return &MultiNodeReconciler{
		client:     client,
		scheme:     scheme,
		LWS:        lws.NewLWSReconciler(client, scheme, headPodSpec, workerPodSpec, int32(workerSize), componentExt, componentMeta),
		URL:        url,
		Service:    service.NewServiceReconciler(client, scheme, componentMeta, componentExt, headPodSpec, selector),
		PodMonitor: podmonitor.NewPodMonitorReconciler(client, scheme, componentMeta, headPodSpec),
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
	if _, err := r.Service.Reconcile(); err != nil {
		return nil, err
	}
	if _, err := r.PodMonitor.Reconcile(); err != nil {
		return nil, err
	}
	return existingLWS, nil
}
