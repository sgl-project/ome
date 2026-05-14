package basemodel

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/runtimeselector"
)

// BaseModelValidator validates namespace-scoped BaseModel runtime compatibility.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
// +kubebuilder:object:generate=false
// +k8s:openapi-gen=false
type BaseModelValidator struct {
	RuntimeSelector runtimeselector.Selector
}

// ClusterBaseModelValidator validates cluster-scoped ClusterBaseModel runtime compatibility.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
// +kubebuilder:object:generate=false
// +k8s:openapi-gen=false
type ClusterBaseModelValidator struct {
	RuntimeSelector runtimeselector.Selector
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-basemodel,mutating=false,failurePolicy=fail,groups=ome.io,resources=basemodels,versions=v1beta1,name=basemodel.ome-webhook-server.validator
var _ webhook.CustomValidator = &BaseModelValidator{}

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-clusterbasemodel,mutating=false,failurePolicy=fail,groups=ome.io,resources=clusterbasemodels,versions=v1beta1,name=clusterbasemodel.ome-webhook-server.validator
var _ webhook.CustomValidator = &ClusterBaseModelValidator{}

// ValidateCreate implements webhook.CustomValidator.
func (v *BaseModelValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	model, err := convertToBaseModel(obj)
	if err != nil {
		return nil, err
	}
	return v.validateModelRuntimeSupport(ctx, model.Name, model.Namespace, &model.Spec)
}

// ValidateUpdate implements webhook.CustomValidator.
func (v *BaseModelValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	model, err := convertToBaseModel(newObj)
	if err != nil {
		return nil, err
	}
	return v.validateModelRuntimeSupport(ctx, model.Name, model.Namespace, &model.Spec)
}

// ValidateDelete implements webhook.CustomValidator.
func (v *BaseModelValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateCreate implements webhook.CustomValidator.
func (v *ClusterBaseModelValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	model, err := convertToClusterBaseModel(obj)
	if err != nil {
		return nil, err
	}
	return v.validateModelRuntimeSupport(ctx, model.Name, "", &model.Spec)
}

// ValidateUpdate implements webhook.CustomValidator.
func (v *ClusterBaseModelValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	model, err := convertToClusterBaseModel(newObj)
	if err != nil {
		return nil, err
	}
	return v.validateModelRuntimeSupport(ctx, model.Name, "", &model.Spec)
}

// ValidateDelete implements webhook.CustomValidator.
func (v *ClusterBaseModelValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *BaseModelValidator) validateModelRuntimeSupport(ctx context.Context, modelName, namespace string, model *v1beta1.BaseModelSpec) (admission.Warnings, error) {
	return validateModelRuntimeSupport(ctx, v.RuntimeSelector, modelName, namespace, model)
}

func (v *ClusterBaseModelValidator) validateModelRuntimeSupport(ctx context.Context, modelName, namespace string, model *v1beta1.BaseModelSpec) (admission.Warnings, error) {
	return validateModelRuntimeSupport(ctx, v.RuntimeSelector, modelName, namespace, model)
}

func validateModelRuntimeSupport(ctx context.Context, selector runtimeselector.Selector, modelName, namespace string, model *v1beta1.BaseModelSpec) (admission.Warnings, error) {
	if selector == nil {
		return nil, fmt.Errorf("runtime selector is not configured")
	}
	if model == nil {
		return nil, fmt.Errorf("model specification is nil")
	}
	if model.Disabled != nil && *model.Disabled {
		return nil, nil
	}
	if model.ModelFormat.Name == "" {
		// Model metadata can be populated asynchronously by the model agent. There is not
		// enough information to validate runtime support until the model format is known.
		return nil, nil
	}

	if _, err := selector.SelectRuntimeForModel(ctx, model, namespace); err != nil {
		return nil, fmt.Errorf("no supporting runtime found for model %s: %w", modelName, err)
	}

	return nil, nil
}

func convertToBaseModel(obj runtime.Object) (*v1beta1.BaseModel, error) {
	model, ok := obj.(*v1beta1.BaseModel)
	if !ok {
		return nil, fmt.Errorf("expected BaseModel, got %T", obj)
	}
	return model, nil
}

func convertToClusterBaseModel(obj runtime.Object) (*v1beta1.ClusterBaseModel, error) {
	model, ok := obj.(*v1beta1.ClusterBaseModel)
	if !ok {
		return nil, fmt.Errorf("expected ClusterBaseModel, got %T", obj)
	}
	return model, nil
}
