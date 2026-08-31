package runtimeselector

import (
	"fmt"
	"strings"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func modelRequiresCacheProvider(model *v1beta1.BaseModelSpec) bool {
	return model != nil &&
		model.Distribution != nil &&
		*model.Distribution == v1beta1.DistributionSharded
}

func supportedFormatSupportsModelCacheProvider(format v1beta1.SupportedModelFormat, provider string) bool {
	provider = strings.TrimSpace(provider)
	if provider == "" || len(format.ModelCacheProviders) == 0 {
		return false
	}
	for _, supported := range format.ModelCacheProviders {
		if string(supported) == provider {
			return true
		}
	}
	return false
}

func modelCacheProviderMismatchReason(provider string, format v1beta1.SupportedModelFormat) string {
	provider = strings.TrimSpace(provider)
	if len(format.ModelCacheProviders) == 0 {
		if provider == "" {
			return "no model cache provider is configured for sharded model loading"
		}
		return fmt.Sprintf("runtime format does not support model cache provider %q", provider)
	}
	if provider == "" {
		return "no model cache provider is configured for sharded model loading"
	}

	supported := make([]string, 0, len(format.ModelCacheProviders))
	for _, cacheProvider := range format.ModelCacheProviders {
		supported = append(supported, string(cacheProvider))
	}
	return fmt.Sprintf("runtime format supports model cache providers [%s], not %q", strings.Join(supported, ", "), provider)
}
