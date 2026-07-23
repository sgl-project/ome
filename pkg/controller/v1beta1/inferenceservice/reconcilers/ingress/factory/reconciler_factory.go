package factory

import (
	"fmt"

	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/services"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/strategies"
)

// DefaultStrategyFactory implements StrategyFactory interface
type DefaultStrategyFactory struct {
	clientset     kubernetes.Interface
	domainService interfaces.DomainService
	pathService   interfaces.PathService
}

// NewStrategyFactory creates a new strategy factory
func NewStrategyFactory(clientset kubernetes.Interface) interfaces.StrategyFactory {
	return &DefaultStrategyFactory{
		clientset:     clientset,
		domainService: services.NewDomainService(),
		pathService:   services.NewPathService(),
	}
}

// CreateStrategyWithOptions creates the appropriate ingress strategy with options
func (f *DefaultStrategyFactory) CreateStrategyWithOptions(deploymentMode string, opts interfaces.ReconcilerOptions) (interfaces.IngressStrategy, error) {
	switch deploymentMode {
	case string(constants.RawDeployment), string(constants.MultiNode):
		if opts.IngressConfig != nil && opts.IngressConfig.EnableGatewayAPI {
			return strategies.NewGatewayAPIStrategy(opts, f.domainService, f.pathService), nil
		} else {
			return strategies.NewKubernetesIngressStrategy(opts, f.domainService, f.pathService), nil
		}
	default:
		return nil, fmt.Errorf("unsupported deployment mode: %s", deploymentMode)
	}
}
