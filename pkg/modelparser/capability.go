package modelparser

import (
	"go.uber.org/zap"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/modelconfig"
)

// capabilityToOME maps modelconfig's package-internal Capability
// enum onto the OME API's v1beta1.ModelCapability. Both are string
// types with deliberately matching wire values; this map exists
// so the dependency direction is one-way (modelparser ->
// modelconfig).
var capabilityToOME = map[modelconfig.Capability]v1beta1.ModelCapability{
	modelconfig.CapabilityUnknown:          v1beta1.ModelCapabilityUnknown,
	modelconfig.CapabilityTextToText:       v1beta1.ModelCapabilityTextToText,
	modelconfig.CapabilityImageTextToText:  v1beta1.ModelCapabilityImageTextToText,
	modelconfig.CapabilityTextToImage:      v1beta1.ModelCapabilityTextToImage,
	modelconfig.CapabilityImageTextToImage: v1beta1.ModelCapabilityImageTextToImage,
	modelconfig.CapabilityTextToVideo:      v1beta1.ModelCapabilityTextToVideo,
	modelconfig.CapabilityImageTextToVideo: v1beta1.ModelCapabilityImageTextToVideo,
	modelconfig.CapabilityTextToAudio:      v1beta1.ModelCapabilityTextToAudio,
	modelconfig.CapabilityImageTextToAudio: v1beta1.ModelCapabilityImageTextToAudio,
	modelconfig.CapabilityVideoTextToAudio: v1beta1.ModelCapabilityVideoTextToAudio,
	modelconfig.CapabilityAudioToText:      v1beta1.ModelCapabilityAudioToText,
	modelconfig.CapabilityAudioToAudio:     v1beta1.ModelCapabilityAudioToAudio,
	modelconfig.CapabilityEmbedding:        v1beta1.ModelCapabilityEmbedding,
}

// capabilitiesAsStrings extracts the model's capabilities (via
// modelconfig's classifier) and converts them to the []string form
// that ModelMetadata.ModelCapabilities holds. Logs a warning when
// the classifier returns Unknown so we can iterate on rule coverage.
func capabilitiesAsStrings(hf modelconfig.HuggingFaceModel, logger *zap.SugaredLogger) []string {
	caps := hf.GetCapabilities()
	out := make([]string, 0, len(caps))
	for _, c := range caps {
		if c == modelconfig.CapabilityUnknown && logger != nil {
			logger.Warnf("model classified as Unknown: model_type=%s architecture=%s",
				hf.GetModelType(), hf.GetArchitecture())
		}
		if mapped, ok := capabilityToOME[c]; ok {
			out = append(out, string(mapped))
		}
	}
	return out
}
