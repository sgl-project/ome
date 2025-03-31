package training_agent

import "testing"

func TestGetLayerNumberPrefixes(t *testing.T) {
	totalLayerNumber := 10
	nLastLayers := 5

	prefix := GetLayerNumberPrefixes(totalLayerNumber, nLastLayers)

	if len(prefix) != 5 {
		t.Errorf("error getting layer number prefix: %v", prefix)
	}
}
