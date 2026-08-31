package raw

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	knapis "knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/deployment"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/services"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/pdb"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/podmonitor"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/service"
)

// RawKubeReconciler reconciles the Native K8S Resources
type RawKubeReconciler struct {
	reader              client.Reader
	Deployment          *deployment.DeploymentReconciler
	Service             *service.ServiceReconciler
	PodDisruptionBudget *pdb.PDBReconciler
	pdbRequest          pdb.Request
	PodMonitor          *podmonitor.PodMonitorReconciler
	URL                 *knapis.URL
}

// NewRawKubeReconciler creates raw kubernetes resource reconciler.
func NewRawKubeReconciler(client client.Client,
	reader client.Reader,
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	pdbRequest pdb.Request,
	componentMeta metav1.ObjectMeta,
	inferenceServiceSpec *v1beta1.InferenceServiceSpec,
	podSpec *corev1.PodSpec,
	resolvedAutoscaler *v1beta1.ComponentAutoscaler,
) (*RawKubeReconciler, error) {
	if reader == nil {
		reader = client
	}
	pm := podmonitor.NewPodMonitorReconciler(client, scheme, componentMeta, podSpec)
	url, err := createRawURL(clientset, componentMeta)
	if err != nil {
		return nil, err
	}

	// Component-level extension spec is carried on Spec.Engine by the
	// caller (deployment_reconciler.go) regardless of which actual
	// component is being reconciled.
	componentExt := &inferenceServiceSpec.Engine.ComponentExtensionSpec
	pdbReconciler := pdb.NewPDBReconcilerWithReader(client, reader, scheme)

	return &RawKubeReconciler{
		reader:              reader,
		Deployment:          deployment.NewDeploymentReconciler(client, scheme, componentMeta, componentExt, podSpec, resolvedAutoscaler),
		Service:             service.NewServiceReconciler(client, scheme, componentMeta, componentExt, podSpec, nil),
		PodDisruptionBudget: pdbReconciler,
		pdbRequest:          pdbRequest,
		PodMonitor:          pm,
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
	// Honor the configured urlScheme rather than hardcoding
	// http; NewIngressConfig defaults UrlScheme when unset, so this is never empty.
	url.Scheme = ingressConfig.UrlScheme
	url.Host, err = domainService.GenerateDomainName(metadata.Name, metadata, ingressConfig)
	if err != nil {
		return nil, err
	}

	return url, nil
}

// Reconcile ...
func (r *RawKubeReconciler) Reconcile(ctx context.Context) (*appsv1.Deployment, error) {
	// reconcile Deployments
	dply, err := r.Deployment.Reconcile()
	if err != nil {
		return nil, err
	}
	cutoverReady, err := pdb.RawDeploymentCutoverReady(ctx, r.reader, dply)
	if err != nil {
		return nil, err
	}
	r.pdbRequest.SelectorCutoverReady = cutoverReady
	// reconcile Service
	_, err = r.Service.Reconcile()
	if err != nil {
		return nil, err
	}
	// reconcile PDB
	_, err = r.PodDisruptionBudget.Reconcile(ctx, r.pdbRequest)
	if err != nil {
		return nil, err
	}
	// reconcile PodMonitor
	_, err = r.PodMonitor.Reconcile()
	if err != nil {
		return nil, err
	}
	return dply, nil
}
