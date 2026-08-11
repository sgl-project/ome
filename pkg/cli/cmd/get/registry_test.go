package get

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCanonicalAndAlias(t *testing.T) {
	for _, name := range []string{"inferenceservices", "inferenceservice", "isvc", "ISVC"} {
		e, err := resolve(name)
		require.NoError(t, err, name)
		assert.Equal(t, "inferenceservices", e.Canonical, name)
	}
}

func TestResolveUnknownListsChoices(t *testing.T) {
	_, err := resolve("banana")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown resource \"banana\"")
	assert.Contains(t, err.Error(), "inferenceservices") // error enumerates valid names
}

func TestRegistryHasAllResources(t *testing.T) {
	want := []string{
		"inferenceservices", "basemodels", "clusterbasemodels",
		"servingruntimes", "clusterservingruntimes", "acceleratorclasses",
		"benchmarkjobs", "finetunedweights", "inferencereplicas",
		"workloadclusters", "models", "runtimes",
	}
	var got []string
	for _, e := range registry {
		got = append(got, e.Canonical)
	}
	assert.ElementsMatch(t, want, got)
}
