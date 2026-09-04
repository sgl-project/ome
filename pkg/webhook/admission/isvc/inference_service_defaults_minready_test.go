package isvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

func TestDefaultOMENativeMinReadySeconds(t *testing.T) {
	omeNative := map[string]string{constants.DeploymentMode: string(constants.OMENative)}

	t.Run("unconfigured stamps nothing", func(t *testing.T) {
		ext := &v1beta1.ComponentExtensionSpec{Annotations: omeNative}
		defaultOMENativeMinReadySeconds(ext, nil, nil)
		assert.Nil(t, ext.Lifecycle, "no lifecycle block may be manufactured without a configured default")
	})

	t.Run("configured default fills an unset OMENative component", func(t *testing.T) {
		seconds := int32(30)
		ext := &v1beta1.ComponentExtensionSpec{Annotations: omeNative}
		defaultOMENativeMinReadySeconds(ext, nil, &seconds)
		require.NotNil(t, ext.Lifecycle)
		require.NotNil(t, ext.Lifecycle.MinReadySeconds)
		assert.Equal(t, int32(30), *ext.Lifecycle.MinReadySeconds)
		assert.NotSame(t, &seconds, ext.Lifecycle.MinReadySeconds, "stamped value must be a copy, not the shared config pointer")
	})

	t.Run("authored value wins over the configured default", func(t *testing.T) {
		seconds := int32(30)
		authored := int32(5)
		ext := &v1beta1.ComponentExtensionSpec{
			Annotations: omeNative,
			Lifecycle:   &v1beta1.LifecycleSpec{MinReadySeconds: &authored},
		}
		defaultOMENativeMinReadySeconds(ext, nil, &seconds)
		assert.Equal(t, int32(5), *ext.Lifecycle.MinReadySeconds)
	})

	t.Run("authored zero is preserved", func(t *testing.T) {
		seconds := int32(30)
		zero := int32(0)
		ext := &v1beta1.ComponentExtensionSpec{
			Annotations: omeNative,
			Lifecycle:   &v1beta1.LifecycleSpec{MinReadySeconds: &zero},
		}
		defaultOMENativeMinReadySeconds(ext, nil, &seconds)
		assert.Equal(t, int32(0), *ext.Lifecycle.MinReadySeconds)
	})

	t.Run("non-OMENative component is left untouched", func(t *testing.T) {
		seconds := int32(30)
		ext := &v1beta1.ComponentExtensionSpec{
			Annotations: map[string]string{constants.DeploymentMode: string(constants.RawDeployment)},
		}
		defaultOMENativeMinReadySeconds(ext, nil, &seconds)
		assert.Nil(t, ext.Lifecycle)
	})

	t.Run("spec.deploymentMode OMENative resolves without a component annotation", func(t *testing.T) {
		seconds := int32(30)
		mode := constants.OMENative
		ext := &v1beta1.ComponentExtensionSpec{}
		defaultOMENativeMinReadySeconds(ext, &mode, &seconds)
		require.NotNil(t, ext.Lifecycle)
		assert.Equal(t, int32(30), *ext.Lifecycle.MinReadySeconds)
	})

	t.Run("DefaultInferenceService stamps every OMENative component from deploy config", func(t *testing.T) {
		seconds := int32(45)
		isvc := &v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{constants.DeploymentMode: string(constants.OMENative)}},
			Spec: v1beta1.InferenceServiceSpec{
				Engine:  &v1beta1.EngineSpec{},
				Decoder: &v1beta1.DecoderSpec{},
				Router:  &v1beta1.RouterSpec{},
			},
		}
		cfg := &controllerconfig.DeployConfig{DefaultDeploymentMode: string(constants.RawDeployment), MinReadySeconds: &seconds}
		require.NoError(t, DefaultInferenceService(context.Background(), createFakeClient(t), isvc, cfg))
		for name, lc := range map[string]*v1beta1.LifecycleSpec{
			"engine":  isvc.Spec.Engine.Lifecycle,
			"decoder": isvc.Spec.Decoder.Lifecycle,
			"router":  isvc.Spec.Router.Lifecycle,
		} {
			require.NotNil(t, lc, "%s lifecycle", name)
			require.NotNil(t, lc.MinReadySeconds, "%s minReadySeconds", name)
			assert.Equal(t, int32(45), *lc.MinReadySeconds, name)
		}
	})

	t.Run("DefaultInferenceService leaves minReadySeconds unset without deploy config", func(t *testing.T) {
		isvc := &v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{constants.DeploymentMode: string(constants.OMENative)}},
			Spec:       v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{}},
		}
		require.NoError(t, DefaultInferenceService(context.Background(), createFakeClient(t), isvc, nil))
		require.NotNil(t, isvc.Spec.Engine.Lifecycle, "OMENative lifecycle defaults still apply")
		assert.Nil(t, isvc.Spec.Engine.Lifecycle.MinReadySeconds, "no in-code minReadySeconds default")
	})
}
