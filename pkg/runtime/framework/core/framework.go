package core

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	runtimeobj "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime/framework"
	fwkplugins "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime/framework/plugins"
	"context"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

func (f *Framework) RunEnforceMLPolicyPlugins(info *runtimeobj.Info, trainJob *v1beta1.TrainingJob) error {
	for _, plugin := range f.enforceMLPlugins {
		if err := plugin.EnforceMLPolicy(info, trainJob); err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) RunEnforcePodGroupPolicyPlugins(info *runtimeobj.Info, trainJob *v1beta1.TrainingJob) error {
	for _, plugin := range f.enforcePodGroupPolicyPlugins {
		if err := plugin.EnforcePodGroupPolicy(info, trainJob); err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) RunComponentBuilderPlugins(ctx context.Context, runtimeJobTemplate client.Object, info *runtimeobj.Info, trainJob *v1beta1.TrainingJob) ([]client.Object, error) {
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

func (f *Framework) RunTerminalConditionPlugins(ctx context.Context, trainJob *v1beta1.TrainingJob) (*metav1.Condition, error) {
	if len(f.terminalConditionPlugins) > 1 {
		return nil, errors.New("too many TerminalCondition plugins are registered")
	}
	if len(f.terminalConditionPlugins) != 0 {
		return f.terminalConditionPlugins[0].TerminalCondition(ctx, trainJob)
	}
	return nil, nil
}

func (f *Framework) WatchExtensionPlugins() []framework.WatchExtensionPlugin {
	return f.watchExtensionPlugins
}
