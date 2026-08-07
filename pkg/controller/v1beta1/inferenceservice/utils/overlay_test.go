package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOverlayEnvVarName(t *testing.T) {
	tests := []struct {
		modelName string
		want      string
	}{
		{"foo", "OVERLAY_FOO_MODEL_PATH"},
		{"foo-pvc", "OVERLAY_FOO_PVC_MODEL_PATH"},
		{"llama-70b-pd-test", "OVERLAY_LLAMA_70B_PD_TEST_MODEL_PATH"},
		{"already_underscored", "OVERLAY_ALREADY_UNDERSCORED_MODEL_PATH"},
		{"Mixed-Case-Name", "OVERLAY_MIXED_CASE_NAME_MODEL_PATH"},
	}
	for _, tc := range tests {
		t.Run(tc.modelName, func(t *testing.T) {
			assert.Equal(t, tc.want, OverlayEnvVarName(tc.modelName))
		})
	}
}

// Two overlays whose names sanitize to the same env var must be
// rejected by webhook. The sanitization function itself doesn't
// enforce uniqueness; this test pins the collision behaviour so the
// webhook check has a stable specification to validate against.
func TestSanitizeOverlayName_HyphenUnderscoreCollision(t *testing.T) {
	assert.Equal(t, sanitizeOverlayName("foo-bar"), sanitizeOverlayName("foo_bar"),
		"hyphens and underscores collapse to the same sanitized form — webhook must reject this combination")
}

func TestOverlayMountPath(t *testing.T) {
	assert.Equal(t, "/opt/ml/model-overlays/foo-pvc", OverlayMountPath("foo-pvc"))
	assert.Equal(t, "/opt/ml/model-overlays/llama-70b-pd-test", OverlayMountPath("llama-70b-pd-test"))
}
