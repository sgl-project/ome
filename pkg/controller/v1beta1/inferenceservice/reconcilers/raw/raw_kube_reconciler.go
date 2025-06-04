package raw

import (
	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/autoscaler"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/deployment"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/services"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/service"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	knapis "knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	inferenceServiceSpec *v1beta1.InferenceServiceSpec,
	podSpec *corev1.PodSpec,
) (*RawKubeReconciler, error) {
	as, err := autoscaler.NewAutoscalerReconciler(client, clientset, scheme, componentMeta, inferenceServiceSpec)
	if err != nil {
		return nil, err
	}

	url, err := createRawURL(clientset, componentMeta)
	if err != nil {
		return nil, err
	}

	// TODO: Remove this once we have a better way to handle the component extension spec
	// Ensure we are using the predictor's extension spec for the component reconcilers
	componentExt := &inferenceServiceSpec.Predictor.ComponentExtensionSpec

	return &RawKubeReconciler{
		client:     client,
		scheme:     scheme,
		Deployment: deployment.NewDeploymentReconciler(client, scheme, componentMeta, componentExt, podSpec),
		Service:    service.NewServiceReconciler(client, scheme, componentMeta, componentExt, podSpec, nil),
		Scaler:     as,
		URL:        url,
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
		return nil, err
	}

	return url, nil
}

// Reconcile ...
func (r *RawKubeReconciler) Reconcile() (*appsv1.Deployment, error) {
	// reconcile Deployments
	dply, err := r.Deployment.Reconcile()
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
	return dply, nil
}
