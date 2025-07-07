package builders

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/controllerconfig"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
	"github.com/sgl-project/ome/pkg/utils"
)

// IngressBuilder builds Kubernetes Ingress resources
type IngressBuilder struct {
	scheme        *runtime.Scheme
	ingressConfig *controllerconfig.IngressConfig
	isvcConfig    *controllerconfig.InferenceServicesConfig
	domainService interfaces.DomainService
	pathService   interfaces.PathService
}

// NewIngressBuilder creates a new Kubernetes Ingress builder
func NewIngressBuilder(scheme *runtime.Scheme, ingressConfig *controllerconfig.IngressConfig, isvcConfig *controllerconfig.InferenceServicesConfig,
	domainService interfaces.DomainService, pathService interfaces.PathService) interfaces.IngressBuilder {
	return &IngressBuilder{
		scheme:        scheme,
		ingressConfig: ingressConfig,
		isvcConfig:    isvcConfig,
		domainService: domainService,
		pathService:   pathService,
	}
}

func (b *IngressBuilder) GetResourceType() string {
	return "Ingress"
}

func (b *IngressBuilder) Build(ctx context.Context, isvc *v1beta1.InferenceService) (client.Object, error) {
	return b.BuildIngress(ctx, isvc)
}

func (b *IngressBuilder) BuildIngress(ctx context.Context, isvc *v1beta1.InferenceService) (client.Object, error) {
	var rules []netv1.IngressRule

	switch {
	case isvc.Spec.Router != nil:
		if !isvc.Status.IsConditionReady(v1beta1.RouterReady) {
			isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
				Type:   v1beta1.IngressReady,
				Status: corev1.ConditionFalse,
				Reason: "Router ingress not created",
			})
			return nil, nil
		}
		routerRules, err := b.buildRouterRules(isvc)
		if err != nil {
			return nil, err
		}
		rules = append(rules, routerRules...)

	case isvc.Spec.Decoder != nil:
		if !isvc.Status.IsConditionReady(v1beta1.DecoderReady) {
			isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
				Type:   v1beta1.IngressReady,
				Status: corev1.ConditionFalse,
				Reason: "Decoder ingress not created",
			})
			return nil, nil
		}
		decoderRules, err := b.buildDecoderRules(isvc)
		if err != nil {
			return nil, err
		}
		rules = append(rules, decoderRules...)

	default:
		if !isvc.Status.IsConditionReady(v1beta1.EngineReady) {
			isvc.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{
				Type:   v1beta1.IngressReady,
				Status: corev1.ConditionFalse,
				Reason: "Engine ingress not created",
			})
			return nil, nil
		}
		engineRules, err := b.buildEngineOnlyRules(isvc)
		if err != nil {
			return nil, err
		}
		rules = append(rules, engineRules...)
	}

	ingress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        isvc.ObjectMeta.Name,
			Namespace:   isvc.ObjectMeta.Namespace,
			Annotations: isvc.Annotations,
		},
		Spec: netv1.IngressSpec{
			IngressClassName: b.ingressConfig.IngressClassName,
			Rules:            rules,
		},
	}

	if err := controllerutil.SetControllerReference(isvc, ingress, b.scheme); err != nil {
		return nil, err
	}

	return ingress, nil
}

func (b *IngressBuilder) buildRouterRules(isvc *v1beta1.InferenceService) ([]netv1.IngressRule, error) {
	var rules []netv1.IngressRule

	routerName := constants.RouterServiceName(isvc.Name)
	decoderName := constants.DecoderServiceName(isvc.Name)
	engineName := constants.EngineServiceName(isvc.Name)

	// 1. Default/top-level host routes to router (since router exists)
	host, err := b.generateIngressHost(string(constants.Router), true, routerName, isvc)
	if err != nil {
		return nil, fmt.Errorf("failed creating top level router ingress host: %w", err)
	}
	rules = append(rules, b.generateRule(host, routerName, "/", constants.CommonISVCPort))

	// 2. Component-specific rule for router
	routerHost, err := b.generateIngressHost(string(constants.Router), false, routerName, isvc)
	if err != nil {
		return nil, fmt.Errorf("failed creating router ingress host: %w", err)
	}
	rules = append(rules, b.generateRule(routerHost, routerName, "/", constants.CommonISVCPort))

	// 3. Component-specific rule for engine
	engineHost, err := b.generateIngressHost(string(constants.Engine), false, engineName, isvc)
	if err != nil {
		return nil, fmt.Errorf("failed creating engine ingress host: %w", err)
	}
	rules = append(rules, b.generateRule(engineHost, engineName, "/", constants.CommonISVCPort))

	// 4. Component-specific rule for decoder (if decoder exists)
	if isvc.Spec.Decoder != nil {
		decoderHost, err := b.generateIngressHost(string(constants.Decoder), false, decoderName, isvc)
		if err != nil {
			return nil, fmt.Errorf("failed creating decoder ingress host: %w", err)
		}
		rules = append(rules, b.generateRule(decoderHost, decoderName, "/", constants.CommonISVCPort))
	}

	return rules, nil
}

func (b *IngressBuilder) buildDecoderRules(isvc *v1beta1.InferenceService) ([]netv1.IngressRule, error) {
	var rules []netv1.IngressRule

	decoderName := constants.DecoderServiceName(isvc.Name)
	engineName := constants.EngineServiceName(isvc.Name)

	// 1. Default/top-level host routes to engine (since no router exists)
	host, err := b.generateIngressHost(string(constants.Decoder), true, decoderName, isvc)
	if err != nil {
		return nil, fmt.Errorf("failed creating top level decoder ingress host: %w", err)
	}
	rules = append(rules, b.generateRule(host, engineName, "/", constants.CommonISVCPort))

	// 2. Component-specific rule for engine
	engineHost, err := b.generateIngressHost(string(constants.Engine), false, engineName, isvc)
	if err != nil {
		return nil, fmt.Errorf("failed creating engine ingress host: %w", err)
	}
	rules = append(rules, b.generateRule(engineHost, engineName, "/", constants.CommonISVCPort))

	// 3. Component-specific rule for decoder
	decoderHost, err := b.generateIngressHost(string(constants.Decoder), false, decoderName, isvc)
	if err != nil {
		return nil, fmt.Errorf("failed creating decoder ingress host: %w", err)
	}
	rules = append(rules, b.generateRule(decoderHost, decoderName, "/", constants.CommonISVCPort))

	return rules, nil
}

func (b *IngressBuilder) buildEngineOnlyRules(isvc *v1beta1.InferenceService) ([]netv1.IngressRule, error) {
	var rules []netv1.IngressRule

	engineName := constants.EngineServiceName(isvc.Name)

	host, err := b.generateIngressHost(string(constants.Engine), true, engineName, isvc)
	if err != nil {
		return nil, fmt.Errorf("failed creating top level engine ingress host: %w", err)
	}

	rules = append(rules, b.generateRule(host, engineName, "/", constants.CommonISVCPort))

	return rules, nil
}

func (b *IngressBuilder) generateRule(ingressHost string, componentName string, path string, port int32) netv1.IngressRule {
	pathType := netv1.PathTypePrefix
	rule := netv1.IngressRule{
		Host: ingressHost,
		IngressRuleValue: netv1.IngressRuleValue{
			HTTP: &netv1.HTTPIngressRuleValue{
				Paths: []netv1.HTTPIngressPath{
					{
						Path:     path,
						PathType: &pathType,
						Backend: netv1.IngressBackend{
							Service: &netv1.IngressServiceBackend{
								Name: componentName,
								Port: netv1.ServiceBackendPort{
									Number: port,
								},
							},
						},
					},
				},
			},
		},
	}
	return rule
}

func (b *IngressBuilder) generateMetadata(isvc *v1beta1.InferenceService,
	componentType constants.InferenceServiceComponent, name string,
) metav1.ObjectMeta {
	// get annotations from isvc
	annotations := utils.Filter(isvc.Annotations, func(key string) bool {
		return !utils.Includes(constants.ServiceAnnotationDisallowedList, key)
	})
	objectMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: isvc.Namespace,
		Labels: utils.Union(isvc.Labels, map[string]string{
			constants.InferenceServicePodLabelKey: isvc.Name,
			constants.OMEComponentLabel:           string(componentType),
		}),
		Annotations: annotations,
	}
	return objectMeta
}

// generateIngressHost return the config domain in configmap.IngressDomain
func (b *IngressBuilder) generateIngressHost(componentType string, topLevelFlag bool, name string, isvc *v1beta1.InferenceService) (string, error) {
	metadata := b.generateMetadata(isvc, constants.InferenceServiceComponent(componentType), name)
	if !topLevelFlag {
		return b.domainService.GenerateDomainName(metadata.Name, isvc.ObjectMeta, b.ingressConfig)
	} else {
		return b.domainService.GenerateDomainName(isvc.Name, isvc.ObjectMeta, b.ingressConfig)
	}
}
