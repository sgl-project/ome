package components

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/pdb"
)

func preflightComponentPDB(
	ctx context.Context,
	b *BaseComponentFields,
	request pdb.Request,
) error {
	return pdb.NewPDBReconcilerWithReader(b.Client, b.APIReader, b.Scheme).Preflight(ctx, request)
}

func resolveComponentPDBRequest(
	b *BaseComponentFields,
	isvc *v1beta1.InferenceService,
	mode constants.DeploymentModeType,
	componentType v1beta1.ComponentType,
	objectMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
) (pdb.Request, error) {
	budget, err := resolveComponentPDBBudgetForMode(b, mode, componentExt)
	if err != nil {
		return pdb.Request{}, err
	}
	request := pdb.Request{Owner: isvc, ObjectMeta: objectMeta, Budget: budget}
	rawSelector := map[string]string{
		constants.RawDeploymentAppLabel: constants.GetRawServiceLabel(objectMeta.Name),
	}
	omeSelector := omeNativeComponentSelector(isvc, componentType)
	switch mode {
	case constants.RawDeployment:
		request.Selector = rawSelector
		request.CutoverFromSelector = omeSelector
	case constants.OMENative:
		request.Selector = omeSelector
		request.CutoverFromSelector = rawSelector
	}
	return request, nil
}

// resolveComponentPDBBudgetForMode selects and validates the effective policy
// for deployment modes that generate PodDisruptionBudgets.
func resolveComponentPDBBudgetForMode(
	b *BaseComponentFields,
	mode constants.DeploymentModeType,
	componentExt *v1beta1.ComponentExtensionSpec,
) (*pdb.Budget, error) {
	if mode != constants.RawDeployment && mode != constants.OMENative {
		return nil, nil
	}

	var fallback *controllerconfig.PodDisruptionBudgetPolicy
	if b != nil && b.InferenceServiceConfig != nil {
		if mode == constants.RawDeployment {
			fallback = b.InferenceServiceConfig.PodDisruptionBudget.RawDeployment
		} else {
			fallback = b.InferenceServiceConfig.PodDisruptionBudget.OMENative
		}
	}
	return pdb.ResolveBudget(componentExt, fallback)
}
