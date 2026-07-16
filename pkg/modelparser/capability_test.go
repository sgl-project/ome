package modelparser

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/modelconfig"
)

// TestCapabilityToOME pins the 1:1 mapping from modelconfig's
// internal Capability enum to v1beta1.ModelCapability. Both are
// string types with deliberately matching wire values, so this
// test catches drift if either side adds a value without updating
// the map.
func TestCapabilityToOME(t *testing.T) {
	cases := []struct {
		mc  modelconfig.Capability
		ome v1beta1.ModelCapability
	}{
		{modelconfig.CapabilityUnknown, v1beta1.ModelCapabilityUnknown},
		{modelconfig.CapabilityTextToText, v1beta1.ModelCapabilityTextToText},
		{modelconfig.CapabilityImageTextToText, v1beta1.ModelCapabilityImageTextToText},
		{modelconfig.CapabilityTextToImage, v1beta1.ModelCapabilityTextToImage},
		{modelconfig.CapabilityImageTextToImage, v1beta1.ModelCapabilityImageTextToImage},
		{modelconfig.CapabilityTextToVideo, v1beta1.ModelCapabilityTextToVideo},
		{modelconfig.CapabilityImageTextToVideo, v1beta1.ModelCapabilityImageTextToVideo},
		{modelconfig.CapabilityTextToAudio, v1beta1.ModelCapabilityTextToAudio},
		{modelconfig.CapabilityImageTextToAudio, v1beta1.ModelCapabilityImageTextToAudio},
		{modelconfig.CapabilityVideoTextToAudio, v1beta1.ModelCapabilityVideoTextToAudio},
		{modelconfig.CapabilityAudioToText, v1beta1.ModelCapabilityAudioToText},
		{modelconfig.CapabilityAudioToAudio, v1beta1.ModelCapabilityAudioToAudio},
		{modelconfig.CapabilityAudioTextToText, v1beta1.ModelCapabilityAudioTextToText},
		{modelconfig.CapabilityEmbedding, v1beta1.ModelCapabilityEmbedding},
	}
	for _, tc := range cases {
		t.Run(string(tc.mc), func(t *testing.T) {
			got, ok := capabilityToOME[tc.mc]
			assert.True(t, ok, "missing mapping for %s", tc.mc)
			assert.Equal(t, tc.ome, got)
			// Wire values must match too — the whole point of the
			// design is that string(modelconfig.X) == string(v1beta1.X).
			assert.Equal(t, string(tc.ome), string(tc.mc),
				"wire value drift: modelconfig=%q v1beta1=%q",
				string(tc.mc), string(tc.ome))
		})
	}
}
