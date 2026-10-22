package isvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/validation"
)

// ratioGroupISVC is an OMENative engine+decoder pair under RecreatePod on
// both sides in one rollingUpdate group with maintainRatio; tolerance nil
// omits it. Every member rolls drain-first, the shape the batch step paces.
func ratioGroupISVC(engineReplicas, decoderReplicas int, tolerance *int32) *v1beta1.InferenceService {
	return readyPolicyISVC(func(isvc *v1beta1.InferenceService) {
		omeNative := map[string]string{constants.DeploymentMode: string(constants.OMENative)}
		runner := &v1beta1.RunnerSpec{Container: v1.Container{Image: "runtime-image:v1"}}
		recreate := &v1beta1.LifecycleSpec{UpdateStrategy: &v1beta1.UpdateStrategy{Type: v1beta1.UpdateStrategyRecreatePod}}
		isvc.Spec.Engine = &v1beta1.EngineSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeNative,
				MinReplicas: ptr.To(engineReplicas), MaxReplicas: engineReplicas, Lifecycle: recreate.DeepCopy()},
			Runner: runner.DeepCopy(),
		}
		isvc.Spec.Decoder = &v1beta1.DecoderSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: omeNative,
				MinReplicas: ptr.To(decoderReplicas), MaxReplicas: decoderReplicas, Lifecycle: recreate.DeepCopy()},
			Runner: runner.DeepCopy(),
		}
		isvc.Spec.Rollout = &v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{{
			Components:    []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
			RollingUpdate: &v1beta1.GroupRollingUpdate{},
			MaintainRatio: &v1beta1.MaintainRatio{Tolerance: tolerance},
		}}}
	})
}

// mixedRatioGroupISVC is ratioGroupISVC with a SurgeThenDrain engine beside
// the RecreatePod decoder: the engine steps on its surges and the decoder on
// its drains, so the batch step paces the pair.
func mixedRatioGroupISVC(engineReplicas, decoderReplicas int, tolerance *int32) *v1beta1.InferenceService {
	isvc := ratioGroupISVC(engineReplicas, decoderReplicas, tolerance)
	isvc.Spec.Engine.Lifecycle.UpdateStrategy.Type = v1beta1.UpdateStrategySurgeThenDrain
	return isvc
}

// inPlaceEngineRatioGroupISVC is ratioGroupISVC with an engine that patches
// its single pods in place beside the RecreatePod decoder: the engine does
// not step, so the ratio check, not the batch step, paces the decoder's
// drains.
func inPlaceEngineRatioGroupISVC(engineReplicas, decoderReplicas int, tolerance *int32) *v1beta1.InferenceService {
	isvc := ratioGroupISVC(engineReplicas, decoderReplicas, tolerance)
	isvc.Spec.Engine.Lifecycle.UpdateStrategy.Type = v1beta1.UpdateStrategyInPlaceIfPossible
	return isvc
}

// withSurgingRouters adds two routers under SurgeThenDrain to the group. A
// router does not decide whether the batch step paces the engine and the
// decoder: one that surges rolls on the ratio and surge gates.
func withSurgingRouters(isvc *v1beta1.InferenceService) *v1beta1.InferenceService {
	isvc.Spec.Router = &v1beta1.RouterSpec{
		ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
			Annotations: map[string]string{constants.DeploymentMode: string(constants.OMENative)},
			MinReplicas: ptr.To(2), MaxReplicas: 2,
			Lifecycle: &v1beta1.LifecycleSpec{UpdateStrategy: &v1beta1.UpdateStrategy{Type: v1beta1.UpdateStrategySurgeThenDrain}},
		},
		Runner: &v1beta1.RunnerSpec{Container: v1.Container{Image: "runtime-image:v1"}},
	}
	isvc.Spec.Rollout.Groups[0].Components = append(isvc.Spec.Rollout.Groups[0].Components, v1beta1.RouterComponent)
	return isvc
}

// withoutDecoderReplicas clears the decoder's replica fields, the shape the
// reconciler sizes at the configured minReplicas default, else one.
func withoutDecoderReplicas(isvc *v1beta1.InferenceService) *v1beta1.InferenceService {
	isvc.Spec.Decoder.MinReplicas = nil
	isvc.Spec.Decoder.MaxReplicas = 0
	return isvc
}

func TestValidateCreate_RatioDrainFeasibility(t *testing.T) {
	tests := []struct {
		name               string
		isvc               *v1beta1.InferenceService
		defaultTolerance   *int32
		defaultMinReplicas *int
		wantErr            bool
	}{
		{
			name: "6:2 RecreatePod on both sides at tolerance 20 is admitted: the batch step paces the group",
			isvc: ratioGroupISVC(6, 2, ptr.To[int32](20)),
		},
		{
			name: "6:2 with a SurgeThenDrain engine and a RecreatePod decoder at tolerance 20 is admitted: the batch step paces the mixed pair",
			isvc: mixedRatioGroupISVC(6, 2, ptr.To[int32](20)),
		},
		{
			name:    "6:2 with an in-place single-pod engine and a RecreatePod decoder at tolerance 20 is refused on create: the engine does not step",
			isvc:    inPlaceEngineRatioGroupISVC(6, 2, ptr.To[int32](20)),
			wantErr: true,
		},
		{
			name: "the same in-place pair at tolerance 50 is admitted",
			isvc: inPlaceEngineRatioGroupISVC(6, 2, ptr.To[int32](50)),
		},
		{
			name: "6:2 RecreatePod on both sides with two SurgeThenDrain routers in the group at tolerance 20 is admitted: the routers do not decide the batch step",
			isvc: withSurgingRouters(ratioGroupISVC(6, 2, ptr.To[int32](20))),
		},
		{
			name: "the mixed group with two SurgeThenDrain routers at tolerance 20 is admitted: the batch step paces the pair and the routers join",
			isvc: withSurgingRouters(mixedRatioGroupISVC(6, 2, ptr.To[int32](20))),
		},
		{
			name:    "the in-place pair with two SurgeThenDrain routers at tolerance 20 is refused: the engine does not step, so the ratio check keeps the decoder",
			isvc:    withSurgingRouters(inPlaceEngineRatioGroupISVC(6, 2, ptr.To[int32](20))),
			wantErr: true,
		},
		{
			name: "omitted tolerance with no operator default is admitted",
			isvc: inPlaceEngineRatioGroupISVC(6, 2, nil),
		},
		{
			name:             "omitted tolerance under an operator default of 20 refuses the in-place pair",
			isvc:             inPlaceEngineRatioGroupISVC(6, 2, nil),
			defaultTolerance: ptr.To[int32](20),
			wantErr:          true,
		},
		{
			name:             "omitted tolerance under an operator default of 20 admits the mixed pair",
			isvc:             mixedRatioGroupISVC(6, 2, nil),
			defaultTolerance: ptr.To[int32](20),
		},
		{
			name:             "omitted tolerance under an operator default of 20 admits the all-RecreatePod group",
			isvc:             ratioGroupISVC(6, 2, nil),
			defaultTolerance: ptr.To[int32](20),
		},
		{
			name:    "a RecreatePod decoder with no replica fields beside an in-place single-pod engine is sized at one and refused",
			isvc:    withoutDecoderReplicas(inPlaceEngineRatioGroupISVC(6, 2, ptr.To[int32](50))),
			wantErr: true,
		},
		{
			name: "a RecreatePod decoder with no replica fields beside a SurgeThenDrain engine is sized at one and admitted: the batch step takes the side of one in its first batch",
			isvc: withoutDecoderReplicas(mixedRatioGroupISVC(6, 2, ptr.To[int32](50))),
		},
		{
			name: "a RecreatePod decoder with no replica fields beside a RecreatePod engine is sized at one and admitted: the batch step takes the side of one in its first batch",
			isvc: withoutDecoderReplicas(ratioGroupISVC(6, 2, ptr.To[int32](50))),
		},
		{
			name:               "the in-place pair's decoder under a configured minReplicas default of 2 is sized at 2 and admitted at tolerance 50",
			isvc:               withoutDecoderReplicas(inPlaceEngineRatioGroupISVC(6, 2, ptr.To[int32](50))),
			defaultMinReplicas: ptr.To(2),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &InferenceServiceValidator{DefaultRatioTolerancePercent: tt.defaultTolerance, DefaultMinReplicas: tt.defaultMinReplicas}
			_, err := v.ValidateCreate(context.Background(), tt.isvc)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), validation.ReasonRatioDrainInfeasible)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestValidateUpdate_RatioDrainFeasibilityScaleDown(t *testing.T) {
	v := &InferenceServiceValidator{}
	old := inPlaceEngineRatioGroupISVC(4, 4, ptr.To[int32](20))
	_, err := v.ValidateCreate(context.Background(), old)
	require.NoError(t, err, "4:4 at tolerance 20 is admitted on create")

	scaled := inPlaceEngineRatioGroupISVC(4, 1, ptr.To[int32](20))
	_, err = v.ValidateUpdate(context.Background(), old, scaled)
	require.Error(t, err, "scaling the decoder to one under RecreatePod beside an in-place single-pod engine leaves it no admissible drain")
	assert.Contains(t, err.Error(), validation.ReasonRatioDrainInfeasible)

	unchanged := inPlaceEngineRatioGroupISVC(4, 4, ptr.To[int32](20))
	_, err = v.ValidateUpdate(context.Background(), old, unchanged)
	assert.NoError(t, err)

	for name, pair := range map[string]func(int, int, *int32) *v1beta1.InferenceService{
		"RecreatePod on both sides":                       ratioGroupISVC,
		"a surging engine beside the RecreatePod decoder": mixedRatioGroupISVC,
	} {
		batched := pair(4, 4, ptr.To[int32](20))
		_, err = v.ValidateUpdate(context.Background(), batched, pair(4, 1, ptr.To[int32](20)))
		assert.NoError(t, err, "under %s the batch step paces the group, so the scale to one is admitted", name)
	}
}
