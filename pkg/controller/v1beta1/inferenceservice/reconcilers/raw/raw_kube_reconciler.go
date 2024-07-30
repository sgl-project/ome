package raw

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	knapis "knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	autoscaler "bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/autoscaler"
	deployment "bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/deployment"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress"
	service "bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/service"
)

// RawKubeReconciler reconciles the Native K8S Resources
type RawKubeReconciler struct {
	client     client.Client
	scheme     *runtime.Scheme
	Deployment *deployment.DeploymentReconciler
	Service    *service.ServiceReconciler
	Scaler     *autoscaler.AutoscalerReconciler
	URL        *knapis.URL
}

// NewRawKubeReconciler creates raw kubernetes resource reconciler.
func NewRawKubeReconciler(client client.Client,
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
	podSpec *corev1.PodSpec) (*RawKubeReconciler, error) {
	as, err := autoscaler.NewAutoscalerReconciler(client, scheme, componentMeta, componentExt)
	if err != nil {
		return nil, err
	}

	url, err := createRawURL(clientset, componentMeta)
	if err != nil {
		return nil, err
	}

	return &RawKubeReconciler{
		client:     client,
		scheme:     scheme,
		Deployment: deployment.NewDeploymentReconciler(client, scheme, componentMeta, componentExt, podSpec),
		Service:    service.NewServiceReconciler(client, scheme, componentMeta, componentExt, podSpec),
		Scaler:     as,
		URL:        url,
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

// Reconcile ...
func (r *RawKubeReconciler) Reconcile() (*appsv1.Deployment, error) {
	// reconcile Deployment
	deployment, err := r.Deployment.Reconcile()
	if err != nil {
		return nil, err
	}
	// reconcile Service
	_, err = r.Service.Reconcile()
	if err != nil {
		return nil, err
	}
	// reconcile HPA
	err = r.Scaler.Reconcile()
	if err != nil {
		return nil, err
	}
	return deployment, nil
}
