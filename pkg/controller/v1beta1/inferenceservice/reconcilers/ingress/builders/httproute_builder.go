package builders

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
	"github.com/sgl-project/ome/pkg/utils"
)

const (
	EngineComponent   = "engine"
	RouterComponent   = "router"
	DecoderComponent  = "decoder"
	TopLevelComponent = "toplevel"
)

var DefaultTimeout = toGatewayAPIDuration(60)

// HTTPRouteBuilder builds Gateway API HTTPRoute resources
type HTTPRouteBuilder struct {
	ingressConfig *controllerconfig.IngressConfig
	isvcConfig    *controllerconfig.InferenceServicesConfig
	domainService interfaces.DomainService
	pathService   interfaces.PathService
}

// NewHTTPRouteBuilder creates a new HTTPRoute builder
func NewHTTPRouteBuilder(ingressConfig *controllerconfig.IngressConfig, isvcConfig *controllerconfig.InferenceServicesConfig,
	domainService interfaces.DomainService, pathService interfaces.PathService) interfaces.HTTPRouteBuilder {
	return &HTTPRouteBuilder{
		ingressConfig: ingressConfig,
		isvcConfig:    isvcConfig,
		domainService: domainService,
		pathService:   pathService,
	}
}

func (b *HTTPRouteBuilder) GetResourceType() string {
	return "HTTPRoute"
}

func (b *HTTPRouteBuilder) Build(ctx context.Context, isvc *v1beta1.InferenceService) (client.Object, error) {
	return b.BuildHTTPRoute(ctx, isvc, EngineComponent)
}

func (b *HTTPRouteBuilder) BuildHTTPRoute(ctx context.Context, isvc *v1beta1.InferenceService, componentType string) (client.Object, error) {
	switch componentType {
	case EngineComponent:
		return b.buildEngineHTTPRoute(isvc)
	case RouterComponent:
		return b.buildRouterHTTPRoute(isvc)
	case DecoderComponent:
		return b.buildDecoderHTTPRoute(isvc)
	case TopLevelComponent:
		return b.buildTopLevelHTTPRoute(isvc)
	default:
		return nil, fmt.Errorf("unsupported component type: %s", componentType)
	}
}

// toGatewayAPIDuration converts seconds to gatewayapiv1.Duration
func toGatewayAPIDuration(seconds int64) *gatewayapiv1.Duration {
	duration := gatewayapiv1.Duration(fmt.Sprintf("%ds", seconds))
	return &duration
}

func (b *HTTPRouteBuilder) createHTTPRouteMatch(prefix string) gatewayapiv1.HTTPRouteMatch {
	return gatewayapiv1.HTTPRouteMatch{
		Path: &gatewayapiv1.HTTPPathMatch{
			Type:  ptr.To(gatewayapiv1.PathMatchRegularExpression),
			Value: ptr.To(prefix),
		},
	}
}

func (b *HTTPRouteBuilder) addIsvcHeaders(name string, namespace string) gatewayapiv1.HTTPRouteFilter {
	return gatewayapiv1.HTTPRouteFilter{
		Type: gatewayapiv1.HTTPRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &gatewayapiv1.HTTPHeaderFilter{
			Set: []gatewayapiv1.HTTPHeader{
				{
					Name:  constants.IsvcNameHeader,
					Value: name,
				},
				{
					Name:  constants.IsvcNamespaceHeader,
					Value: namespace,
				},
			},
		},
	}
}

func (b *HTTPRouteBuilder) createHTTPRouteRule(routeMatches []gatewayapiv1.HTTPRouteMatch, filters []gatewayapiv1.HTTPRouteFilter,
	serviceName, namespace string, port int32, timeout *gatewayapiv1.Duration,
) gatewayapiv1.HTTPRouteRule {
	var backendRefs []gatewayapiv1.HTTPBackendRef
	if serviceName != "" {
		backendRefs = []gatewayapiv1.HTTPBackendRef{
			{
				BackendRef: gatewayapiv1.BackendRef{
					BackendObjectReference: gatewayapiv1.BackendObjectReference{
						Kind:      ptr.To(gatewayapiv1.Kind(constants.ServiceKind)),
						Name:      gatewayapiv1.ObjectName(serviceName),
						Namespace: (*gatewayapiv1.Namespace)(&namespace),
						Port:      (*gatewayapiv1.PortNumber)(&port),
					},
				},
			},
		}
	}
	return gatewayapiv1.HTTPRouteRule{
		Matches:     routeMatches,
		Filters:     filters,
		BackendRefs: backendRefs,
		Timeouts: &gatewayapiv1.HTTPRouteTimeouts{
			Request: timeout,
		},
	}
}

func (b *HTTPRouteBuilder) buildEngineHTTPRoute(isvc *v1beta1.InferenceService) (*gatewayapiv1.HTTPRoute, error) {
	if !isvc.Status.IsConditionReady(v1beta1.EngineReady) {
		isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
			Type:   v1beta1.IngressReady,
			Status: corev1.ConditionFalse,
			Reason: "Engine ingress not created",
		})
		return nil, nil
	}

	engineName := constants.PredictorServiceName(isvc.Name)
	filters := []gatewayapiv1.HTTPRouteFilter{b.addIsvcHeaders(isvc.Name, isvc.Namespace)}

	engineHost, err := b.domainService.GenerateDomainName(engineName, isvc.ObjectMeta, b.ingressConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to generate engine ingress host: %w", err)
	}

	allowedHosts := []gatewayapiv1.Hostname{gatewayapiv1.Hostname(engineHost)}
	routeMatch := []gatewayapiv1.HTTPRouteMatch{b.createHTTPRouteMatch(constants.FallbackPrefix())}

	timeout := DefaultTimeout
	if isvc.Spec.Predictor.TimeoutSeconds != nil {
		timeout = toGatewayAPIDuration(*isvc.Spec.Predictor.TimeoutSeconds)
	}

	httpRouteRules := []gatewayapiv1.HTTPRouteRule{
		b.createHTTPRouteRule(routeMatch, filters, engineName, isvc.Namespace, constants.CommonISVCPort, timeout),
	}

	return b.buildHTTPRouteResource(isvc, constants.PredictorServiceName(isvc.Name), allowedHosts, httpRouteRules), nil
}

func (b *HTTPRouteBuilder) buildRouterHTTPRoute(isvc *v1beta1.InferenceService) (*gatewayapiv1.HTTPRoute, error) {
	if !isvc.Status.IsConditionReady(v1beta1.RoutesReady) {
		isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
			Type:   v1beta1.IngressReady,
			Status: corev1.ConditionFalse,
			Reason: "Router ingress not created",
		})
		return nil, nil
	}

	routerName := constants.RouterServiceName(isvc.Name)
	filters := []gatewayapiv1.HTTPRouteFilter{b.addIsvcHeaders(isvc.Name, isvc.Namespace)}

	routerHost, err := b.domainService.GenerateDomainName(routerName, isvc.ObjectMeta, b.ingressConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to generate router ingress host: %w", err)
	}

	allowedHosts := []gatewayapiv1.Hostname{gatewayapiv1.Hostname(routerHost)}
	routeMatch := []gatewayapiv1.HTTPRouteMatch{b.createHTTPRouteMatch(constants.FallbackPrefix())}

	timeout := DefaultTimeout
	if isvc.Spec.Router.TimeoutSeconds != nil {
		timeout = toGatewayAPIDuration(*isvc.Spec.Router.TimeoutSeconds)
	}

	httpRouteRules := []gatewayapiv1.HTTPRouteRule{
		b.createHTTPRouteRule(routeMatch, filters, routerName, isvc.Namespace, constants.CommonISVCPort, timeout),
	}

	return b.buildHTTPRouteResource(isvc, constants.RouterServiceName(isvc.Name), allowedHosts, httpRouteRules), nil
}

func (b *HTTPRouteBuilder) buildDecoderHTTPRoute(isvc *v1beta1.InferenceService) (*gatewayapiv1.HTTPRoute, error) {
	if !isvc.Status.IsConditionReady(v1beta1.DecoderReady) {
		isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
			Type:   v1beta1.IngressReady,
			Status: corev1.ConditionFalse,
			Reason: "Decoder ingress not created",
		})
		return nil, nil
	}

	decoderName := constants.DecoderServiceName(isvc.Name)
	filters := []gatewayapiv1.HTTPRouteFilter{b.addIsvcHeaders(isvc.Name, isvc.Namespace)}

	decoderHost, err := b.domainService.GenerateDomainName(decoderName, isvc.ObjectMeta, b.ingressConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to generate decoder ingress host: %w", err)
	}

	allowedHosts := []gatewayapiv1.Hostname{gatewayapiv1.Hostname(decoderHost)}
	routeMatch := []gatewayapiv1.HTTPRouteMatch{b.createHTTPRouteMatch(constants.FallbackPrefix())}

	timeout := DefaultTimeout
	if isvc.Spec.Decoder.TimeoutSeconds != nil {
		timeout = toGatewayAPIDuration(*isvc.Spec.Decoder.TimeoutSeconds)
	}

	httpRouteRules := []gatewayapiv1.HTTPRouteRule{
		b.createHTTPRouteRule(routeMatch, filters, decoderName, isvc.Namespace, constants.CommonISVCPort, timeout),
	}

	return b.buildHTTPRouteResource(isvc, constants.DecoderServiceName(isvc.Name), allowedHosts, httpRouteRules), nil
}

func (b *HTTPRouteBuilder) buildTopLevelHTTPRoute(isvc *v1beta1.InferenceService) (*gatewayapiv1.HTTPRoute, error) {
	if !isvc.Status.IsConditionReady(v1beta1.PredictorReady) {
		isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
			Type:   v1beta1.IngressReady,
			Status: corev1.ConditionFalse,
			Reason: "Engine ingress not created",
		})
		return nil, nil
	}

	var httpRouteRules []gatewayapiv1.HTTPRouteRule
	engineName := constants.PredictorServiceName(isvc.Name)
	routerName := constants.RouterServiceName(isvc.Name)
	decoderName := constants.DecoderServiceName(isvc.Name)

	topLevelHost, err := b.domainService.GenerateDomainName(isvc.Name, isvc.ObjectMeta, b.ingressConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to generate top level ingress host: %w", err)
	}

	allowedHosts := []gatewayapiv1.Hostname{gatewayapiv1.Hostname(topLevelHost)}

	// Add additional hosts
	domainList := []string{b.ingressConfig.IngressDomain}
	additionalHosts := b.domainService.GetAdditionalHostsWithAnnotations(&domainList, topLevelHost, b.ingressConfig, isvc.Annotations)
	if additionalHosts != nil {
		hostMap := make(map[gatewayapiv1.Hostname]bool, len(allowedHosts))
		for _, host := range allowedHosts {
			hostMap[host] = true
		}
		for _, additionalHost := range *additionalHosts {
			gwHost := gatewayapiv1.Hostname(additionalHost)
			if _, found := hostMap[gwHost]; !found {
				allowedHosts = append(allowedHosts, gwHost)
			}
		}
	}

	filters := []gatewayapiv1.HTTPRouteFilter{b.addIsvcHeaders(isvc.Name, isvc.Namespace)}

	// Build component-specific routes
	if isvc.Spec.Decoder != nil {
		if !isvc.Status.IsConditionReady(v1beta1.DecoderReady) {
			isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
				Type:   v1beta1.IngressReady,
				Status: corev1.ConditionFalse,
				Reason: "Decoder ingress not created",
			})
			return nil, nil
		}
		timeout := DefaultTimeout
		if isvc.Spec.Decoder.TimeoutSeconds != nil {
			timeout = toGatewayAPIDuration(*isvc.Spec.Decoder.TimeoutSeconds)
		}
		explainRouteMatch := []gatewayapiv1.HTTPRouteMatch{b.createHTTPRouteMatch(constants.DecoderPrefix())}
		httpRouteRules = append(httpRouteRules, b.createHTTPRouteRule(explainRouteMatch, filters, decoderName, isvc.Namespace, constants.CommonISVCPort, timeout))
	}

	if isvc.Spec.Router != nil {
		if !isvc.Status.IsConditionReady(v1beta1.RoutesReady) {
			isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
				Type:   v1beta1.IngressReady,
				Status: corev1.ConditionFalse,
				Reason: "Router ingress not created",
			})
			return nil, nil
		}
		timeout := DefaultTimeout
		if isvc.Spec.Router.TimeoutSeconds != nil {
			timeout = toGatewayAPIDuration(*isvc.Spec.Router.TimeoutSeconds)
		}
		routeMatch := []gatewayapiv1.HTTPRouteMatch{b.createHTTPRouteMatch(constants.FallbackPrefix())}
		httpRouteRules = append(httpRouteRules, b.createHTTPRouteRule(routeMatch, filters, routerName, isvc.Namespace, constants.CommonISVCPort, timeout))
	} else {
		timeout := DefaultTimeout
		if isvc.Spec.Predictor.TimeoutSeconds != nil {
			timeout = toGatewayAPIDuration(*isvc.Spec.Predictor.TimeoutSeconds)
		}
		routeMatch := []gatewayapiv1.HTTPRouteMatch{b.createHTTPRouteMatch(constants.FallbackPrefix())}
		httpRouteRules = append(httpRouteRules, b.createHTTPRouteRule(routeMatch, filters, engineName, isvc.Namespace, constants.CommonISVCPort, timeout))
	}

	// Add path-based routing if configured
	if b.ingressConfig.PathTemplate != "" {
		path, err := b.pathService.GenerateUrlPath(isvc.Name, isvc.Namespace, b.ingressConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to generate URL from pathTemplate: %w", err)
		}
		path = strings.TrimSuffix(path, "/")
		allowedHosts = append(allowedHosts, gatewayapiv1.Hostname(b.ingressConfig.IngressDomain))

		if isvc.Spec.Decoder != nil {
			timeout := DefaultTimeout
			if isvc.Spec.Decoder.TimeoutSeconds != nil {
				timeout = toGatewayAPIDuration(*isvc.Spec.Decoder.TimeoutSeconds)
			}
			decoderPathRouteMatch := []gatewayapiv1.HTTPRouteMatch{b.createHTTPRouteMatch(path + constants.PathBasedExplainPrefix())}
			httpRouteRules = append(httpRouteRules, b.createHTTPRouteRule(decoderPathRouteMatch, filters, decoderName, isvc.Namespace, constants.CommonISVCPort, timeout))
		}

		if isvc.Spec.Router != nil {
			timeout := DefaultTimeout
			if isvc.Spec.Router.TimeoutSeconds != nil {
				timeout = toGatewayAPIDuration(*isvc.Spec.Router.TimeoutSeconds)
			}
			pathRouteMatch := []gatewayapiv1.HTTPRouteMatch{b.createHTTPRouteMatch(path + "/")}
			httpRouteRules = append(httpRouteRules, b.createHTTPRouteRule(pathRouteMatch, filters, routerName, isvc.Namespace, constants.CommonISVCPort, timeout))
		} else {
			timeout := DefaultTimeout
			if isvc.Spec.Predictor.TimeoutSeconds != nil {
				timeout = toGatewayAPIDuration(*isvc.Spec.Predictor.TimeoutSeconds)
			}
			pathRouteMatch := []gatewayapiv1.HTTPRouteMatch{b.createHTTPRouteMatch(path + "/")}
			httpRouteRules = append(httpRouteRules, b.createHTTPRouteRule(pathRouteMatch, filters, engineName, isvc.Namespace, constants.CommonISVCPort, timeout))
		}
	}

	return b.buildHTTPRouteResource(isvc, isvc.Name, allowedHosts, httpRouteRules), nil
}

func (b *HTTPRouteBuilder) buildHTTPRouteResource(isvc *v1beta1.InferenceService, name string, allowedHosts []gatewayapiv1.Hostname, httpRouteRules []gatewayapiv1.HTTPRouteRule) *gatewayapiv1.HTTPRoute {
	annotations := utils.Filter(isvc.Annotations, func(key string) bool {
		return !utils.Includes(constants.ServiceAnnotationDisallowedList, key)
	})
	labels := utils.Filter(isvc.Labels, func(key string) bool {
		return !utils.Includes(constants.RevisionTemplateLabelDisallowedList, key)
	})

	gatewaySlice := strings.Split(b.ingressConfig.OmeIngressGateway, "/")

	return &gatewayapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   isvc.Namespace,
			Annotations: annotations,
			Labels:      labels,
		},
		Spec: gatewayapiv1.HTTPRouteSpec{
			Hostnames: allowedHosts,
			Rules:     httpRouteRules,
			CommonRouteSpec: gatewayapiv1.CommonRouteSpec{
				ParentRefs: []gatewayapiv1.ParentReference{
					{
						Group:     (*gatewayapiv1.Group)(&gatewayapiv1.GroupVersion.Group),
						Kind:      (*gatewayapiv1.Kind)(ptr.To(constants.GatewayKind)),
						Namespace: (*gatewayapiv1.Namespace)(&gatewaySlice[0]),
						Name:      gatewayapiv1.ObjectName(gatewaySlice[1]),
					},
				},
			},
		},
	}
}
