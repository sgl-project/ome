package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestNamespaceDefaultsWhenUnset(t *testing.T) {
	flags := genericclioptions.NewConfigFlags(true)
	// Point at a kubeconfig that doesn't exist so only client defaulting applies.
	empty := ""
	flags.KubeConfig = &empty
	f := New(flags)
	ns, explicit, err := f.Namespace()
	require.NoError(t, err)
	assert.Equal(t, "default", ns)
	assert.False(t, explicit)
}

func TestNamespaceExplicitFlagWins(t *testing.T) {
	flags := genericclioptions.NewConfigFlags(true)
	team := "team-a"
	flags.Namespace = &team
	f := New(flags)
	ns, explicit, err := f.Namespace()
	require.NoError(t, err)
	assert.Equal(t, "team-a", ns)
	assert.True(t, explicit)
}
