package builders

import (
	"context"
	"strings"

	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"

	istiov1beta1 "istio.io/api/networking/v1beta1"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/network"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
	"github.com/sgl-project/ome/pkg/utils"
)

// VirtualServiceBuilder builds Istio VirtualService resources
type VirtualServiceBuilder struct {
	ingressConfig *controllerconfig.IngressConfig
	isvcConfig    *controllerconfig.InferenceServicesConfig
	domainService interfaces.DomainService
	pathService   interfaces.PathService
}

// NewVirtualServiceBuilder creates a new VirtualService builder
func NewVirtualServiceBuilder(ingressConfig *controllerconfig.IngressConfig, isvcConfig *controllerconfig.InferenceServicesConfig,
	domainService interfaces.DomainService, pathService interfaces.PathService) interfaces.VirtualServiceBuilder {
	return &VirtualServiceBuilder{
		ingressConfig: ingressConfig,
		isvcConfig:    isvcConfig,
		domainService: domainService,
		pathService:   pathService,
	}
}

func (b *VirtualServiceBuilder) GetResourceType() string {
	return "VirtualService"
}

func (b *VirtualServiceBuilder) Build(ctx context.Context, isvc *v1beta1.InferenceService) (client.Object, error) {
	return b.BuildVirtualService(ctx, isvc, nil)
}

func (b *VirtualServiceBuilder) BuildVirtualService(ctx context.Context, isvc *v1beta1.InferenceService, domainList *[]string) (client.Object, error) {
	// Determine backend service based on component architecture
	var backend string

	switch {
	case isvc.Spec.Router != nil:
		// Router takes priority - check router readiness
		if !isvc.Status.IsConditionReady(v1beta1.RoutesReady) {
			status := corev1.ConditionFalse
			if isvc.Status.IsConditionUnknown(v1beta1.RoutesReady) {
				status = corev1.ConditionUnknown
			}
			isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
				Type:   v1beta1.IngressReady,
				Status: status,
				Reason: "Router ingress not created",
			})
			return nil, nil
		}
		backend = constants.RouterServiceName(isvc.Name)

	case isvc.Spec.Decoder != nil:
		// Decoder without router - check engine readiness since VirtualService routes to engine
		if !isvc.Status.IsConditionReady(v1beta1.EngineReady) {
			status := corev1.ConditionFalse
			if isvc.Status.IsConditionUnknown(v1beta1.EngineReady) {
				status = corev1.ConditionUnknown
			}
			isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
				Type:   v1beta1.IngressReady,
				Status: status,
				Reason: "Engine ingress not created",
			})
			return nil, nil
		}
		// For serverless with decoder, still route to engine as the entrypoint
		backend = constants.EngineServiceName(isvc.Name)

	default:
		// Engine only - check engine readiness
		if !isvc.Status.IsConditionReady(v1beta1.EngineReady) {
			status := corev1.ConditionFalse
			if isvc.Status.IsConditionUnknown(v1beta1.EngineReady) {
				status = corev1.ConditionUnknown
			}
			isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
				Type:   v1beta1.IngressReady,
				Status: status,
				Reason: "Engine ingress not created",
			})
			return nil, nil
		}
		backend = constants.EngineServiceName(isvc.Name)
	}

	isInternal := b.determineIfInternal(isvc)
	serviceHost := b.getServiceHost(isvc)

	var additionalHosts *[]string
	hosts := []string{network.GetServiceHostname(isvc.Name, isvc.Namespace)}
	if !isInternal {
		additionalHosts = b.domainService.GetAdditionalHostsWithAnnotations(domainList, serviceHost, b.ingressConfig, isvc.Annotations)
	}

	httpRoutes := b.buildHTTPRoutes(isvc, serviceHost, additionalHosts, isInternal, backend)

	gateways := []string{
		b.ingressConfig.LocalGateway,
		constants.IstioMeshGateway,
	}
	if !isInternal {
		hosts = append(hosts, serviceHost)
		gateways = append(gateways, b.ingressConfig.IngressGateway)
	}

	// Add path-based routing if configured
	if b.ingressConfig.PathTemplate != "" {
		pathRoutes, pathHosts := b.buildPathBasedRoutes(isvc, backend)
		httpRoutes = append(httpRoutes, pathRoutes...)
		hosts = append(hosts, pathHosts...)
	}

	if !isInternal {
		hosts = b.addAdditionalHosts(hosts, additionalHosts)
	}

	annotations := utils.Filter(isvc.Annotations, func(key string) bool {
		return !utils.Includes(constants.ServiceAnnotationDisallowedList, key)
	})

	return &istioclientv1beta1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:        isvc.Name,
			Namespace:   isvc.Namespace,
			Annotations: annotations,
			Labels:      isvc.Labels,
		},
		Spec: istiov1beta1.VirtualService{
			Hosts:    hosts,
			Gateways: gateways,
			Http:     httpRoutes,
		},
	}, nil
}

func (b *VirtualServiceBuilder) determineIfInternal(isvc *v1beta1.InferenceService) bool {
	serviceHost := b.getServiceHost(isvc)
	if val, ok := isvc.Labels[constants.VisibilityLabel]; ok && val == constants.ClusterLocalVisibility {
		return true
	}
	serviceInternalHostName := network.GetServiceHostname(isvc.Name, isvc.Namespace)
	return serviceHost == serviceInternalHostName
}

func (b *VirtualServiceBuilder) getServiceHost(isvc *v1beta1.InferenceService) string {
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
				return strings.Replace(routerStatus.URL.Host, "-router-default", "", 1)
			} else {
				return strings.Replace(routerStatus.URL.Host, "-router", "", 1)
			}
		}
	}

	if engineStatus, ok := isvc.Status.Components[v1beta1.EngineComponent]; !ok {
		return ""
	} else if engineStatus.URL == nil {
		return ""
	} else {
		if strings.Contains(engineStatus.URL.Host, "-default") {
			return strings.Replace(engineStatus.URL.Host, "-engine-default", "", 1)
		} else {
			return strings.Replace(engineStatus.URL.Host, "-engine", "", 1)
		}
	}
}

func (b *VirtualServiceBuilder) buildHTTPRoutes(isvc *v1beta1.InferenceService, serviceHost string, additionalHosts *[]string, isInternal bool, backend string) []*istiov1beta1.HTTPRoute {
	var httpRoutes []*istiov1beta1.HTTPRoute
	expBackend := constants.DecoderServiceName(isvc.Name)

	// Build decoder route
	if isvc.Spec.Decoder != nil {
		if !isvc.Status.IsConditionReady(v1beta1.DecoderReady) {
			status := corev1.ConditionFalse
			if isvc.Status.IsConditionUnknown(v1beta1.DecoderReady) {
				status = corev1.ConditionUnknown
			}
			isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
				Type:   v1beta1.IngressReady,
				Status: status,
				Reason: "Decoder ingress not created",
			})
			return nil
		}
		decoderRouter := istiov1beta1.HTTPRoute{
			Match: b.createHTTPMatchRequest(constants.DecoderPrefix(), serviceHost,
				network.GetServiceHostname(isvc.Name, isvc.Namespace), additionalHosts, isInternal),
			Route: []*istiov1beta1.HTTPRouteDestination{
				b.createHTTPRouteDestination(b.ingressConfig.KnativeLocalGatewayService),
			},
			Headers: &istiov1beta1.Headers{
				Request: &istiov1beta1.Headers_HeaderOperations{
					Set: map[string]string{
						"Host":                        network.GetServiceHostname(expBackend, isvc.Namespace),
						constants.IsvcNameHeader:      isvc.Name,
						constants.IsvcNamespaceHeader: isvc.Namespace,
					},
				},
			},
		}
		httpRoutes = append(httpRoutes, &decoderRouter)
	}

	// Add predict route
	httpRoutes = append(httpRoutes, &istiov1beta1.HTTPRoute{
		Match: b.createHTTPMatchRequest("", serviceHost,
			network.GetServiceHostname(isvc.Name, isvc.Namespace), additionalHosts, isInternal),
		Route: []*istiov1beta1.HTTPRouteDestination{
			b.createHTTPRouteDestination(b.ingressConfig.KnativeLocalGatewayService),
		},
		Headers: &istiov1beta1.Headers{
			Request: &istiov1beta1.Headers_HeaderOperations{
				Set: map[string]string{
					"Host":                        network.GetServiceHostname(backend, isvc.Namespace),
					constants.IsvcNameHeader:      isvc.Name,
					constants.IsvcNamespaceHeader: isvc.Namespace,
				},
			},
		},
	})

	return httpRoutes
}

func (b *VirtualServiceBuilder) buildPathBasedRoutes(isvc *v1beta1.InferenceService, backend string) ([]*istiov1beta1.HTTPRoute, []string) {
	var httpRoutes []*istiov1beta1.HTTPRoute
	var hosts []string

	path, err := b.pathService.GenerateUrlPath(isvc.Name, isvc.Namespace, b.ingressConfig)
	if err != nil {
		return nil, nil
	}

	url := &apis.URL{}
	url.Path = strings.TrimSuffix(path, "/")
	url.Host = b.ingressConfig.IngressDomain

	expBackend := constants.DecoderServiceName(isvc.Name)

	// Add decoder path-based route
	if isvc.Spec.Decoder != nil {
		httpRoutes = append(httpRoutes, &istiov1beta1.HTTPRoute{
			Match: []*istiov1beta1.HTTPMatchRequest{
				{
					Uri: &istiov1beta1.StringMatch{
						MatchType: &istiov1beta1.StringMatch_Regex{
							Regex: url.Path + constants.PathBasedExplainPrefix(),
						},
					},
					Authority: &istiov1beta1.StringMatch{
						MatchType: &istiov1beta1.StringMatch_Regex{
							Regex: constants.HostRegExp(url.Host),
						},
					},
					Gateways: []string{b.ingressConfig.IngressGateway},
				},
			},
			Rewrite: &istiov1beta1.HTTPRewrite{
				UriRegexRewrite: &istiov1beta1.RegexRewrite{
					Match:   url.Path + constants.PathBasedExplainPrefix(),
					Rewrite: `\1`,
				},
			},
			Route: []*istiov1beta1.HTTPRouteDestination{
				b.createHTTPRouteDestination(b.ingressConfig.KnativeLocalGatewayService),
			},
			Headers: &istiov1beta1.Headers{
				Request: &istiov1beta1.Headers_HeaderOperations{
					Set: map[string]string{
						"Host":                        network.GetServiceHostname(expBackend, isvc.Namespace),
						constants.IsvcNameHeader:      isvc.Name,
						constants.IsvcNamespaceHeader: isvc.Namespace,
					},
				},
			},
		})
	}

	// Add predict path-based route
	httpRoutes = append(httpRoutes, &istiov1beta1.HTTPRoute{
		Match: []*istiov1beta1.HTTPMatchRequest{
			{
				Uri: &istiov1beta1.StringMatch{
					MatchType: &istiov1beta1.StringMatch_Prefix{
						Prefix: url.Path + "/",
					},
				},
				Authority: &istiov1beta1.StringMatch{
					MatchType: &istiov1beta1.StringMatch_Regex{
						Regex: constants.HostRegExp(url.Host),
					},
				},
				Gateways: []string{b.ingressConfig.IngressGateway},
			},
			{
				Uri: &istiov1beta1.StringMatch{
					MatchType: &istiov1beta1.StringMatch_Exact{
						Exact: url.Path,
					},
				},
				Authority: &istiov1beta1.StringMatch{
					MatchType: &istiov1beta1.StringMatch_Regex{
						Regex: constants.HostRegExp(url.Host),
					},
				},
				Gateways: []string{b.ingressConfig.IngressGateway},
			},
		},
		Rewrite: &istiov1beta1.HTTPRewrite{
			Uri: "/",
		},
		Route: []*istiov1beta1.HTTPRouteDestination{
			b.createHTTPRouteDestination(b.ingressConfig.KnativeLocalGatewayService),
		},
		Headers: &istiov1beta1.Headers{
			Request: &istiov1beta1.Headers_HeaderOperations{
				Set: map[string]string{
					"Host":                        network.GetServiceHostname(backend, isvc.Namespace),
					constants.IsvcNameHeader:      isvc.Name,
					constants.IsvcNamespaceHeader: isvc.Namespace,
				},
			},
		},
	})

	hosts = append(hosts, url.Host)
	return httpRoutes, hosts
}

func (b *VirtualServiceBuilder) addAdditionalHosts(hosts []string, additionalHosts *[]string) []string {
	hostMap := make(map[string]bool, len(hosts))
	for _, host := range hosts {
		hostMap[host] = true
	}
	if additionalHosts != nil && len(*additionalHosts) != 0 {
		for _, additionalHost := range *additionalHosts {
			if !hostMap[additionalHost] {
				hosts = append(hosts, additionalHost)
			}
		}
	}
	return hosts
}

func (b *VirtualServiceBuilder) createHTTPRouteDestination(gatewayService string) *istiov1beta1.HTTPRouteDestination {
	return &istiov1beta1.HTTPRouteDestination{
		Destination: &istiov1beta1.Destination{
			Host: gatewayService,
			Port: &istiov1beta1.PortSelector{
				Number: constants.CommonISVCPort,
			},
		},
		Weight: 100,
	}
}

func (b *VirtualServiceBuilder) createHTTPMatchRequest(prefix, targetHost, internalHost string, additionalHosts *[]string, isInternal bool) []*istiov1beta1.HTTPMatchRequest {
	var uri *istiov1beta1.StringMatch
	if prefix != "" {
		uri = &istiov1beta1.StringMatch{
			MatchType: &istiov1beta1.StringMatch_Regex{
				Regex: prefix,
			},
		}
	}
	matchRequests := []*istiov1beta1.HTTPMatchRequest{
		{
			Uri: uri,
			Authority: &istiov1beta1.StringMatch{
				MatchType: &istiov1beta1.StringMatch_Regex{
					Regex: constants.HostRegExp(internalHost),
				},
			},
			Gateways: []string{b.ingressConfig.LocalGateway, constants.IstioMeshGateway},
		},
	}
	if !isInternal {
		matchRequests = append(matchRequests,
			&istiov1beta1.HTTPMatchRequest{
				Uri: uri,
				Authority: &istiov1beta1.StringMatch{
					MatchType: &istiov1beta1.StringMatch_Regex{
						Regex: constants.HostRegExp(targetHost),
					},
				},
				Gateways: []string{b.ingressConfig.IngressGateway},
			})

		if additionalHosts != nil && len(*additionalHosts) != 0 {
			for _, host := range *additionalHosts {
				matchRequest := &istiov1beta1.HTTPMatchRequest{
					Uri: uri,
					Authority: &istiov1beta1.StringMatch{
						MatchType: &istiov1beta1.StringMatch_Regex{
							Regex: constants.HostRegExp(host),
						},
					},
					Gateways: []string{b.ingressConfig.IngressGateway},
				}
				if !b.containsHTTPMatchRequest(matchRequest, matchRequests) {
					matchRequests = append(matchRequests, matchRequest)
				}
			}
		}
	}
	return matchRequests
}

func (b *VirtualServiceBuilder) containsHTTPMatchRequest(matchRequest *istiov1beta1.HTTPMatchRequest, matchRequests []*istiov1beta1.HTTPMatchRequest) bool {
	for _, matchRequestEle := range matchRequests {
		if b.stringMatchEqual(matchRequest.GetAuthority(), matchRequestEle.GetAuthority()) &&
			b.gatewaysEqual(matchRequest, matchRequestEle) &&
			b.stringMatchEqual(matchRequest.GetUri(), matchRequestEle.GetUri()) {
			return true
		}
	}
	return false
}

func (b *VirtualServiceBuilder) stringMatchEqual(stringMatch, stringMatchDest *istiov1beta1.StringMatch) bool {
	if stringMatch == nil && stringMatchDest == nil {
		return true
	}
	if stringMatch == nil || stringMatchDest == nil {
		return false
	}

	// Compare match types and values
	switch sm := stringMatch.GetMatchType().(type) {
	case *istiov1beta1.StringMatch_Exact:
		if smd, ok := stringMatchDest.GetMatchType().(*istiov1beta1.StringMatch_Exact); ok {
			return sm.Exact == smd.Exact
		}
	case *istiov1beta1.StringMatch_Prefix:
		if smd, ok := stringMatchDest.GetMatchType().(*istiov1beta1.StringMatch_Prefix); ok {
			return sm.Prefix == smd.Prefix
		}
	case *istiov1beta1.StringMatch_Regex:
		if smd, ok := stringMatchDest.GetMatchType().(*istiov1beta1.StringMatch_Regex); ok {
			return sm.Regex == smd.Regex
		}
	}
	return false
}

func (b *VirtualServiceBuilder) gatewaysEqual(matchRequest, matchRequestDest *istiov1beta1.HTTPMatchRequest) bool {
	if len(matchRequest.GetGateways()) != len(matchRequestDest.GetGateways()) {
		return false
	}
	for i, gateway := range matchRequest.GetGateways() {
		if gateway != matchRequestDest.GetGateways()[i] {
			return false
		}
	}
	return true
}
