package strategies

import (
	"context"
	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/builders"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
	"k8s.io/klog/v2"
)

// KubernetesIngressStrategy handles Kubernetes Ingress (raw deployment mode)
type KubernetesIngressStrategy struct {
	client        client.Client
	scheme        *runtime.Scheme
	ingressConfig *controllerconfig.IngressConfig
	isvcConfig    *controllerconfig.InferenceServicesConfig
	domainService interfaces.DomainService
	pathService   interfaces.PathService
	builder       interfaces.IngressBuilder
}

// NewKubernetesIngressStrategy creates a new Kubernetes Ingress strategy
func NewKubernetesIngressStrategy(opts interfaces.ReconcilerOptions, domainService interfaces.DomainService, pathService interfaces.PathService) interfaces.IngressStrategy {
	builder := builders.NewIngressBuilder(opts.Scheme, opts.IngressConfig, opts.IsvcConfig, domainService, pathService)

	return &KubernetesIngressStrategy{
		client:        opts.Client,
		scheme:        opts.Scheme,
		ingressConfig: opts.IngressConfig,
		isvcConfig:    opts.IsvcConfig,
		domainService: domainService,
		pathService:   pathService,
		builder:       builder,
	}
}

func (k *KubernetesIngressStrategy) GetName() string {
	return "KubernetesIngress"
}

func (k *KubernetesIngressStrategy) Reconcile(ctx context.Context, isvc *v1beta1.InferenceService) error {
	var err error

	if !k.ingressConfig.DisableIngressCreation {
		// Use builder to create the ingress
		desired, err := k.builder.BuildIngress(ctx, isvc)
		if err != nil {
			return fmt.Errorf("builder error: %w", err)
		}
		if desired == nil {
			// Log this case to understand why builder returns nil
			klog.Info("Builder returned nil ingress - likely no target service found", "isvc", isvc.Name)
			return nil
		}

		ingress, ok := desired.(*netv1.Ingress)
		if !ok {
			return fmt.Errorf("builder returned unexpected type %T, expected *netv1.Ingress", desired)
		}

		// reconcile ingress
		existingIngress := &netv1.Ingress{}
		err = k.client.Get(ctx, types.NamespacedName{
			Namespace: isvc.Namespace,
			Name:      isvc.Name,
		}, existingIngress)
		if err != nil {
			if apierr.IsNotFound(err) {
				err = k.client.Create(ctx, ingress)
			} else {
				return err
			}
		} else {
			if !k.semanticIngressEquals(ingress, existingIngress) {
				// Set ResourceVersion which is required for update operation
				ingress.ResourceVersion = existingIngress.ResourceVersion
				err = k.client.Update(ctx, ingress)
			}
		}
		if err != nil {
			return err
		}
	}

	// Set status URL and Address
	isvc.Status.URL, err = k.createRawURL(isvc)
	if err != nil {
		return err
	}
	isvc.Status.Address = &duckv1.Addressable{
		URL: &apis.URL{
			Host:   k.getRawServiceHost(isvc),
			Scheme: k.ingressConfig.UrlScheme,
			Path:   "",
		},
	}
	isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
		Type:   v1beta1.IngressReady,
		Status: corev1.ConditionTrue,
	})
	return nil
}

func (k *KubernetesIngressStrategy) createRawURL(isvc *v1beta1.InferenceService) (*apis.URL, error) {
	var err error
	url := &apis.URL{}
	url.Scheme = k.ingressConfig.UrlScheme
	url.Host, err = k.domainService.GenerateDomainName(isvc.Name, isvc.ObjectMeta, k.ingressConfig)
	if err != nil {
		return nil, err
	}
	return url, nil
}

func (k *KubernetesIngressStrategy) getRawServiceHost(isvc *v1beta1.InferenceService) string {
	if isvc.Spec.Router != nil {
		routerName := isvc.Name + "-router"
		return routerName + "." + isvc.Namespace + ".svc.cluster.local"
	}
	engineName := isvc.Name
	return engineName + "." + isvc.Namespace + ".svc.cluster.local"
}

func (k *KubernetesIngressStrategy) semanticIngressEquals(desired, existing *netv1.Ingress) bool {
	return equality.Semantic.DeepEqual(desired.Spec, existing.Spec)
}
