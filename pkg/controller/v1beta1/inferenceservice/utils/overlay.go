package utils

import (
	"strings"
)

const (
	OverlayMountPathPrefix = "/opt/ml/model-overlays"
	OverlayEnvVarPrefix    = "OVERLAY_"
	OverlayEnvVarSuffix    = "_MODEL_PATH"
)

// OverlayEnvVarName returns the env var through which the runner
// addresses one overlay model: OVERLAY_<UPPERCASED_NAME>_MODEL_PATH.
func OverlayEnvVarName(modelName string) string {
	return OverlayEnvVarPrefix + sanitizeOverlayName(modelName) + OverlayEnvVarSuffix
}

// OverlayMountPath returns the in-pod mount path for one overlay model.
func OverlayMountPath(modelName string) string {
	return OverlayMountPathPrefix + "/" + modelName
}

func sanitizeOverlayName(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}
