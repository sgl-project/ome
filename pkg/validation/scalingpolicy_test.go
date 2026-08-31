package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func proportionalPolicy(decoderRatio string) *v1beta1.ScalingPolicy {
	return &v1beta1.ScalingPolicy{
		Mode: v1beta1.ScalingProportional,
		Proportional: &v1beta1.ProportionalPolicy{
			Anchor: v1beta1.EngineComponent,
			Ratios: map[v1beta1.ComponentType]resource.Quantity{
				v1beta1.DecoderComponent: resource.MustParse(decoderRatio),
			},
		},
	}
}

func TestValidateScalingPolicy(t *testing.T) {
	t.Run("nil policy is ok", func(t *testing.T) {
		require.NoError(t, ValidateScalingPolicy(nil, "spec.scalingPolicy"))
	})

	t.Run("empty mode is ok", func(t *testing.T) {
		require.NoError(t, ValidateScalingPolicy(&v1beta1.ScalingPolicy{}, "spec.scalingPolicy"))
	})

	t.Run("Independent is ok", func(t *testing.T) {
		require.NoError(t, ValidateScalingPolicy(
			&v1beta1.ScalingPolicy{Mode: v1beta1.ScalingIndependent}, "spec.scalingPolicy"))
	})

	t.Run("Proportional is rejected as not implemented", func(t *testing.T) {
		err := ValidateScalingPolicy(proportionalPolicy("1"), "spec.scalingPolicy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spec.scalingPolicy.mode")
		assert.Contains(t, err.Error(), "Proportional")
		assert.Contains(t, err.Error(), "not implemented")
		assert.Contains(t, err.Error(), ReasonScalingModeNotImplemented)
	})

	t.Run("Proportional without proportional block is still rejected", func(t *testing.T) {
		err := ValidateScalingPolicy(
			&v1beta1.ScalingPolicy{Mode: v1beta1.ScalingProportional}, "spec.scalingPolicy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented")
	})

	t.Run("Pinned is rejected as not implemented", func(t *testing.T) {
		err := ValidateScalingPolicy(
			&v1beta1.ScalingPolicy{Mode: v1beta1.ScalingPinned}, "spec.scalingPolicy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Pinned")
		assert.Contains(t, err.Error(), "not implemented")
		assert.Contains(t, err.Error(), ReasonScalingModeNotImplemented)
	})

	t.Run("unknown mode is rejected", func(t *testing.T) {
		err := ValidateScalingPolicy(
			&v1beta1.ScalingPolicy{Mode: v1beta1.ScalingMode("Elastic")}, "spec.scalingPolicy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Elastic")
		assert.Contains(t, err.Error(), "is not one of")
	})

	t.Run("fieldPath is reflected in the error", func(t *testing.T) {
		err := ValidateScalingPolicy(
			&v1beta1.ScalingPolicy{Mode: v1beta1.ScalingPinned}, "spec.other.path")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "spec.other.path.mode")
	})
}

func TestValidateScalingPolicyUpdate(t *testing.T) {
	t.Run("nil to nil is ok", func(t *testing.T) {
		require.NoError(t, ValidateScalingPolicyUpdate(nil, nil, "spec.scalingPolicy"))
	})

	t.Run("nil to Independent is ok", func(t *testing.T) {
		require.NoError(t, ValidateScalingPolicyUpdate(
			nil, &v1beta1.ScalingPolicy{Mode: v1beta1.ScalingIndependent}, "spec.scalingPolicy"))
	})

	t.Run("newly set Proportional is rejected", func(t *testing.T) {
		err := ValidateScalingPolicyUpdate(nil, proportionalPolicy("1"), "spec.scalingPolicy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), ReasonScalingModeNotImplemented)
	})

	t.Run("newly set Pinned is rejected", func(t *testing.T) {
		err := ValidateScalingPolicyUpdate(
			nil, &v1beta1.ScalingPolicy{Mode: v1beta1.ScalingPinned}, "spec.scalingPolicy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), ReasonScalingModeNotImplemented)
	})

	t.Run("Independent changed to Proportional is rejected", func(t *testing.T) {
		err := ValidateScalingPolicyUpdate(
			&v1beta1.ScalingPolicy{Mode: v1beta1.ScalingIndependent},
			proportionalPolicy("1"), "spec.scalingPolicy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), ReasonScalingModeNotImplemented)
	})

	t.Run("unchanged stored Proportional is ratcheted through", func(t *testing.T) {
		require.NoError(t, ValidateScalingPolicyUpdate(
			proportionalPolicy("1"), proportionalPolicy("1"), "spec.scalingPolicy"))
	})

	t.Run("unchanged stored Pinned is ratcheted through", func(t *testing.T) {
		pinned := &v1beta1.ScalingPolicy{Mode: v1beta1.ScalingPinned}
		require.NoError(t, ValidateScalingPolicyUpdate(
			pinned, pinned.DeepCopy(), "spec.scalingPolicy"))
	})

	t.Run("semantically equal ratio quantities count as unchanged", func(t *testing.T) {
		require.NoError(t, ValidateScalingPolicyUpdate(
			proportionalPolicy("1"), proportionalPolicy("1000m"), "spec.scalingPolicy"))
	})

	t.Run("changed Proportional ratio is rejected", func(t *testing.T) {
		err := ValidateScalingPolicyUpdate(
			proportionalPolicy("1"), proportionalPolicy("2"), "spec.scalingPolicy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), ReasonScalingModeNotImplemented)
	})

	t.Run("Proportional changed to Pinned is rejected", func(t *testing.T) {
		err := ValidateScalingPolicyUpdate(
			proportionalPolicy("1"),
			&v1beta1.ScalingPolicy{Mode: v1beta1.ScalingPinned}, "spec.scalingPolicy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), ReasonScalingModeNotImplemented)
	})

	t.Run("Proportional downgraded to Independent is ok", func(t *testing.T) {
		require.NoError(t, ValidateScalingPolicyUpdate(
			proportionalPolicy("1"),
			&v1beta1.ScalingPolicy{Mode: v1beta1.ScalingIndependent}, "spec.scalingPolicy"))
	})

	t.Run("Proportional removed entirely is ok", func(t *testing.T) {
		require.NoError(t, ValidateScalingPolicyUpdate(
			proportionalPolicy("1"), nil, "spec.scalingPolicy"))
	})
}
