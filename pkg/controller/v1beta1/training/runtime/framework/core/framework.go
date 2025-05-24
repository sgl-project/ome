package core

import (
	"context"
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework"
	fwkplugins "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins"
)

var errorTooManyTerminalConditionPlugin = errors.New("too many TerminalCondition plugins are registered")

type Framework struct {
	registry                     fwkplugins.Registry
	plugins                      map[string]framework.Plugin
	enforceMLPlugins             []framework.EnforceMLPolicyPlugin
	enforcePodGroupPolicyPlugins []framework.EnforcePodGroupPolicyPlugin
	customValidationPlugins      []framework.CustomValidationPlugin
	watchExtensionPlugins        []framework.WatchExtensionPlugin
	componentBuilderPlugins      []framework.ComponentBuilderPlugin
	terminalConditionPlugins     []framework.TerminalConditionPlugin
}

func New(ctx context.Context, c client.Client, r fwkplugins.Registry, indexer client.FieldIndexer) (*Framework, error) {
	f := &Framework{
		registry: r,
	}
	plugins := make(map[string]framework.Plugin, len(r))

	for name, factory := range r {
		plugin, err := factory(ctx, c, indexer)
		if err != nil {
			return nil, err
		}
		plugins[name] = plugin
		if p, ok := plugin.(framework.EnforceMLPolicyPlugin); ok {
			f.enforceMLPlugins = append(f.enforceMLPlugins, p)
		}
		if p, ok := plugin.(framework.EnforcePodGroupPolicyPlugin); ok {
			f.enforcePodGroupPolicyPlugins = append(f.enforcePodGroupPolicyPlugins, p)
		}
		if p, ok := plugin.(framework.CustomValidationPlugin); ok {
			f.customValidationPlugins = append(f.customValidationPlugins, p)
		}
		if p, ok := plugin.(framework.WatchExtensionPlugin); ok {
			f.watchExtensionPlugins = append(f.watchExtensionPlugins, p)
		}
		if p, ok := plugin.(framework.ComponentBuilderPlugin); ok {
			f.componentBuilderPlugins = append(f.componentBuilderPlugins, p)
		}
		if p, ok := plugin.(framework.TerminalConditionPlugin); ok {
			f.terminalConditionPlugins = append(f.terminalConditionPlugins, p)
		}
	}
	f.plugins = plugins
	return f, nil
}

func (f *Framework) RunEnforceMLPolicyPlugins(info *runtime.Info, trainJob *omev1beta1.TrainingJob) error {
	for _, plugin := range f.enforceMLPlugins {
		if err := plugin.EnforceMLPolicy(info, trainJob); err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) RunEnforcePodGroupPolicyPlugins(info *runtime.Info, trainJob *omev1beta1.TrainingJob) error {
	for _, plugin := range f.enforcePodGroupPolicyPlugins {
		if err := plugin.EnforcePodGroupPolicy(info, trainJob); err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) RunCustomValidationPlugins(oldObj, newObj *omev1beta1.TrainingJob) (admission.Warnings, field.ErrorList) {
	var aggregatedWarnings admission.Warnings
	var aggregatedErrors field.ErrorList
	for _, plugin := range f.customValidationPlugins {
		warnings, errs := plugin.Validate(oldObj, newObj)
		if len(warnings) != 0 {
			aggregatedWarnings = append(aggregatedWarnings, warnings...)
		}
		if errs != nil {
			aggregatedErrors = append(aggregatedErrors, errs...)
		}
	}
	return aggregatedWarnings, aggregatedErrors
}

func (f *Framework) RunComponentBuilderPlugins(ctx context.Context, runtimeJobTemplate client.Object, info *runtime.Info, trainJob *omev1beta1.TrainingJob) ([]client.Object, error) {
	var objs []client.Object
	for _, plugin := range f.componentBuilderPlugins {
		obj, err := plugin.Build(ctx, runtimeJobTemplate, info, trainJob)
		if err != nil {
			return nil, err
		}
		if obj != nil {
			objs = append(objs, obj)
		}
	}
	return objs, nil
}

func (f *Framework) RunTerminalConditionPlugins(ctx context.Context, trainJob *omev1beta1.TrainingJob) (*metav1.Condition, error) {
	// TODO (tenzen-y): Once we provide the Configuration API, we should validate which plugin should have terminalCondition execution points.
	if len(f.terminalConditionPlugins) > 1 {
		return nil, errorTooManyTerminalConditionPlugin
	}
	if len(f.terminalConditionPlugins) != 0 {
		return f.terminalConditionPlugins[0].TerminalCondition(ctx, trainJob)
	}
	return nil, nil
}

func (f *Framework) WatchExtensionPlugins() []framework.WatchExtensionPlugin {
	return f.watchExtensionPlugins
}
