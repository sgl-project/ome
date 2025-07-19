package strategies

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/testing/protocmp"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/network"
	"knative.dev/pkg/system"
	"knative.dev/serving/pkg/reconciler/route/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/builders"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
)

var log = logf.Log.WithName("ServerlessStrategy")

// ServerlessStrategy handles ingress for serverless (Knative) deployment mode
type ServerlessStrategy struct {
	client        client.Client
	clientset     kubernetes.Interface
	scheme        *runtime.Scheme
	ingressConfig *controllerconfig.IngressConfig
	isvcConfig    *controllerconfig.InferenceServicesConfig
	domainService interfaces.DomainService
	pathService   interfaces.PathService
	builder       interfaces.VirtualServiceBuilder
}

// NewServerlessStrategy creates a new serverless strategy
func NewServerlessStrategy(opts interfaces.ReconcilerOptions, clientset kubernetes.Interface, domainService interfaces.DomainService, pathService interfaces.PathService) interfaces.IngressStrategy {
	builder := builders.NewVirtualServiceBuilder(opts.IngressConfig, opts.IsvcConfig, domainService, pathService)

	return &ServerlessStrategy{
		client:        opts.Client,
		clientset:     clientset,
		scheme:        opts.Scheme,
		ingressConfig: opts.IngressConfig,
		isvcConfig:    opts.IsvcConfig,
		domainService: domainService,
		pathService:   pathService,
		builder:       builder,
	}
}

func (s *ServerlessStrategy) GetName() string {
	return "Serverless"
}

func (s *ServerlessStrategy) Reconcile(ctx context.Context, isvc *v1beta1.InferenceService) error {
	disableIstioVirtualHost := s.ingressConfig.DisableIstioVirtualHost

	if err := s.reconcileVirtualService(ctx, isvc); err != nil {
		return errors.Wrapf(err, "fails to reconcile virtual service")
	}

	if err := s.reconcileExternalService(ctx, isvc); err != nil {
		return errors.Wrapf(err, "fails to reconcile external name service")
	}

	serviceHost := s.getServiceHost(isvc)
	serviceUrl := s.getServiceUrl(isvc)
	if serviceHost == "" || serviceUrl == "" {
		log.Info("service host and serviceurl are empty, skipping updating the inference service")
		return nil
	}

	if url, err := apis.ParseURL(serviceUrl); err == nil {
		isvc.Status.URL = url
		hostPrefix := s.getHostPrefix(isvc, disableIstioVirtualHost)
		isvc.Status.Address = &duckv1.Addressable{
			URL: &apis.URL{
				Host:   network.GetServiceHostname(hostPrefix, isvc.Namespace),
				Scheme: "http",
			},
		}
		isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
			Type:   v1beta1.IngressReady,
			Status: corev1.ConditionTrue,
		})
		return nil
	} else {
		return errors.Wrapf(err, "fails to parse service url")
	}
}

func (s *ServerlessStrategy) reconcileVirtualService(ctx context.Context, isvc *v1beta1.InferenceService) error {
	disableIstioVirtualHost := s.ingressConfig.DisableIstioVirtualHost
	domainList := s.getDomainList(ctx)

	// Use builder to create the VirtualService
	desired, err := s.builder.BuildVirtualService(ctx, isvc, domainList)
	if err != nil {
		return err
	}

	existing := &istioclientv1beta1.VirtualService{}
	getExistingErr := s.client.Get(ctx, types.NamespacedName{Name: isvc.Name, Namespace: isvc.Namespace}, existing)

	if !disableIstioVirtualHost {
		if desired == nil {
			return nil
		}

		virtualService, ok := desired.(*istioclientv1beta1.VirtualService)
		if !ok {
			return fmt.Errorf("builder returned unexpected type %T, expected *istioclientv1beta1.VirtualService", desired)
		}

		if err := controllerutil.SetControllerReference(isvc, virtualService, s.scheme); err != nil {
			return errors.Wrapf(err, "fails to set owner reference for ingress")
		}

		if getExistingErr != nil {
			if apierr.IsNotFound(getExistingErr) {
				log.Info("Creating Ingress for isvc", "namespace", virtualService.Namespace, "name", virtualService.Name)
				if err := s.client.Create(ctx, virtualService); err != nil {
					log.Error(err, "Failed to create ingress", "namespace", virtualService.Namespace, "name", virtualService.Name)
					return err
				}
			}
		} else {
			if !s.routeSemanticEquals(virtualService, existing) {
				deepCopy := existing.DeepCopy()
				deepCopy.Spec = *virtualService.Spec.DeepCopy()
				deepCopy.Annotations = virtualService.Annotations
				deepCopy.Labels = virtualService.Labels
				log.Info("Update Ingress for isvc", "namespace", virtualService.Namespace, "name", virtualService.Name)
				if err := s.client.Update(ctx, deepCopy); err != nil {
					log.Error(err, "Failed to update ingress", "namespace", virtualService.Namespace, "name", virtualService.Name)
					return err
				}
			}
		}
	}

	// Delete the virtualservice
	if getExistingErr == nil {
		if len(existing.OwnerReferences) > 0 && existing.OwnerReferences[0].UID == isvc.UID {
			log.Info("The InferenceService ", isvc.Name, " is marked as stopped — delete its associated VirtualService")
			if err := s.client.Delete(ctx, existing); err != nil {
				return err
			}
		}
	} else if !apierr.IsNotFound(getExistingErr) {
		return getExistingErr
	}

	return nil
}

func (s *ServerlessStrategy) reconcileExternalService(ctx context.Context, isvc *v1beta1.InferenceService) error {
	disableIstioVirtualHost := s.ingressConfig.DisableIstioVirtualHost

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      isvc.Name,
			Namespace: isvc.Namespace,
		},
		Spec: corev1.ServiceSpec{
			ExternalName:    s.ingressConfig.LocalGatewayServiceName,
			Type:            corev1.ServiceTypeExternalName,
			SessionAffinity: corev1.ServiceAffinityNone,
		},
	}

	existing := &corev1.Service{}
	getExistingErr := s.client.Get(ctx, types.NamespacedName{Name: isvc.Name, Namespace: isvc.Namespace}, existing)

	if err := controllerutil.SetControllerReference(isvc, desired, s.scheme); err != nil {
		return err
	}

	if !disableIstioVirtualHost {
		if getExistingErr != nil {
			if apierr.IsNotFound(getExistingErr) {
				log.Info("Creating external name service", "namespace", desired.Namespace, "name", desired.Name)
				return s.client.Create(ctx, desired)
			}
			return getExistingErr
		}

		if equality.Semantic.DeepEqual(desired, existing) {
			return nil
		}

		diff, err := kmp.SafeDiff(desired.Spec, existing.Spec)
		if err != nil {
			return errors.Wrapf(err, "failed to diff external name service")
		}
		log.Info("Reconciling external service diff (-desired, +observed):", "diff", diff)
		log.Info("Updating external service", "namespace", existing.Namespace, "name", existing.Name)
		existing.Spec = desired.Spec
		existing.ObjectMeta.Labels = desired.ObjectMeta.Labels
		existing.ObjectMeta.Annotations = desired.ObjectMeta.Annotations
		err = s.client.Update(ctx, existing)
		if err != nil {
			return errors.Wrapf(err, "fails to update external name service")
		}
	}

	if getExistingErr == nil {
		if len(existing.OwnerReferences) > 0 && existing.OwnerReferences[0].UID == isvc.UID {
			log.Info("The InferenceService ", isvc.Name, " is marked as stopped — delete its associated Service")
			if err := s.client.Delete(ctx, existing); err != nil {
				return err
			}
		}
	} else if !apierr.IsNotFound(getExistingErr) {
		return getExistingErr
	}

	return nil
}

// Helper methods for ServerlessStrategy
func (s *ServerlessStrategy) getServiceHost(isvc *v1beta1.InferenceService) string {
	if isvc.Status.Components == nil {
		return ""
	}

	if isvc.Spec.Router != nil {
		if routerStatus, ok := isvc.Status.Components[v1beta1.RouterComponent]; !ok {
			return ""
		} else if routerStatus.URL == nil {
			return ""
		} else {
			if strings.Contains(routerStatus.URL.Host, "-default") {
				return strings.Replace(routerStatus.URL.Host, fmt.Sprintf("-%s-default", string(constants.Router)), "",
					1)
			} else {
				return strings.Replace(routerStatus.URL.Host, "-"+string(constants.Router), "",
					1)
			}
		}
	}

	if engineStatus, ok := isvc.Status.Components[v1beta1.EngineComponent]; !ok {
		return ""
	} else if engineStatus.URL == nil {
		return ""
	} else {
		if strings.Contains(engineStatus.URL.Host, "-default") {
			return strings.Replace(engineStatus.URL.Host, fmt.Sprintf("-%s-default", string(constants.Engine)), "",
				1)
		} else {
			return strings.Replace(engineStatus.URL.Host, "-"+string(constants.Engine), "",
				1)
		}
	}
}

func (s *ServerlessStrategy) getServiceUrl(isvc *v1beta1.InferenceService) string {
	url := s.getHostBasedServiceUrl(isvc)
	if url == "" {
		return ""
	}
	if s.ingressConfig.PathTemplate == "" {
		return url
	} else {
		return s.getPathBasedServiceUrl(isvc)
	}
}

func (s *ServerlessStrategy) getPathBasedServiceUrl(isvc *v1beta1.InferenceService) string {
	path, err := s.pathService.GenerateUrlPath(isvc.Name, isvc.Namespace, s.ingressConfig)
	if err != nil {
		log.Error(err, "Failed to generate URL path from pathTemplate")
		return ""
	}
	url := &apis.URL{}
	url.Scheme = s.ingressConfig.UrlScheme
	url.Host = s.ingressConfig.IngressDomain
	url.Path = path

	return url.String()
}

func (s *ServerlessStrategy) getHostBasedServiceUrl(isvc *v1beta1.InferenceService) string {
	urlScheme := s.ingressConfig.UrlScheme
	disableIstioVirtualHost := s.ingressConfig.DisableIstioVirtualHost
	if isvc.Status.Components == nil {
		return ""
	}

	if isvc.Spec.Router != nil {
		if routerStatus, ok := isvc.Status.Components[v1beta1.RouterComponent]; !ok {
			return ""
		} else if routerStatus.URL == nil {
			return ""
		} else {
			url := routerStatus.URL
			url.Scheme = urlScheme
			urlString := url.String()
			if !disableIstioVirtualHost {
				if strings.Contains(urlString, "-default") {
					return strings.Replace(urlString, fmt.Sprintf("-%s-default", string(constants.Router)), "", 1)
				} else {
					return strings.Replace(urlString, "-"+string(constants.Router), "", 1)
				}
			}
			return urlString
		}
	}

	if engineStatus, ok := isvc.Status.Components[v1beta1.EngineComponent]; !ok {
		return ""
	} else if engineStatus.URL == nil {
		return ""
	} else {
		url := engineStatus.URL
		url.Scheme = urlScheme
		urlString := url.String()
		if !disableIstioVirtualHost {
			if strings.Contains(urlString, "-default") {
				return strings.Replace(urlString, fmt.Sprintf("-%s-default", string(constants.Engine)), "", 1)
			} else {
				return strings.Replace(urlString, "-"+string(constants.Engine), "", 1)
			}
		}
		return urlString
	}
}

func (s *ServerlessStrategy) getHostPrefix(isvc *v1beta1.InferenceService, disableIstioVirtualHost bool) string {
	if disableIstioVirtualHost {
		if isvc.Spec.Router != nil {
			return constants.DefaultRouterServiceName(isvc.Name)
		}
		return constants.PredictorServiceName(isvc.Name)
	}
	return isvc.Name
}

func (s *ServerlessStrategy) getDomainList(ctx context.Context) *[]string {
	res := new([]string)
	ns := constants.DefaultNSKnativeServing
	if namespace := os.Getenv(system.NamespaceEnvKey); namespace != "" {
		ns = namespace
	}

	configMap, err := s.clientset.CoreV1().ConfigMaps(ns).Get(ctx,
		config.DomainConfigName, metav1.GetOptions{})
	if err != nil {
		return res
	}
	for domain := range configMap.Data {
		*res = append(*res, domain)
	}
	return res
}

func (s *ServerlessStrategy) routeSemanticEquals(desired, existing *istioclientv1beta1.VirtualService) bool {
	return cmp.Equal(desired.Spec.DeepCopy(), existing.Spec.DeepCopy(), protocmp.Transform()) &&
		equality.Semantic.DeepEqual(desired.ObjectMeta.Labels, existing.ObjectMeta.Labels) &&
		equality.Semantic.DeepEqual(desired.ObjectMeta.Annotations, existing.ObjectMeta.Annotations)
}
