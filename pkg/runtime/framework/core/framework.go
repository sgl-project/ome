package core

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	runtimeobj "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime/framework"
	fwkplugins "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime/framework/plugins"
	"context"
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
