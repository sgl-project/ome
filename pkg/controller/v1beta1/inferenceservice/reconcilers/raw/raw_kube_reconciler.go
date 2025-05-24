package raw

import (
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	knapis "knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"

	autoscaler "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/autoscaler"
	deployment "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/deployment"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress"
	service "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/service"
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
	inferenceServiceSepc *v1beta1.InferenceServiceSpec,
	podSpec *corev1.PodSpec,
) (*RawKubeReconciler, error) {
	as, err := autoscaler.NewAutoscalerReconciler(client, clientset, scheme, componentMeta, inferenceServiceSepc)
	if err != nil {
		return nil, err
	}

	url, err := createRawURL(clientset, componentMeta)
	if err != nil {
		return nil, err
	}

	componentExt := &inferenceServiceSepc.Predictor.ComponentExtensionSpec
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
