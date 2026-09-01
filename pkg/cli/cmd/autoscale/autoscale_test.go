package autoscale

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"sigs.k8s.io/ome/pkg/cli/factory"
)

func TestNewCmdRegistersStatusSubcommand(t *testing.T) {
	cmd := NewCmd(factory.Static{}, genericiooptions.IOStreams{
		In: &bytes.Buffer{}, Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{},
	})

	assert.Equal(t, "autoscale", cmd.Use)
	found, args, err := cmd.Find([]string{"status"})
	require.NoError(t, err)
	assert.Empty(t, args)
	assert.Equal(t, "autoscale status", found.CommandPath())
	assert.Equal(t, "status INFERENCESERVICE", found.Use)
}

func TestAutoscaleHelpExplainsReportedEvidence(t *testing.T) {
	var output bytes.Buffer
	cmd := NewCmd(factory.Static{}, genericiooptions.IOStreams{
		In: &bytes.Buffer{}, Out: &output, ErrOut: &output,
	})
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--help"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, `Inspect controller-reported autoscaling evidence

Usage:
  autoscale [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  status      Show controller-reported autoscaling status

Flags:
  -h, --help   help for autoscale

Use "autoscale [command] --help" for more information about a command.
`, output.String())
}

func TestStatusHelpDefinesTheReportedEvidenceBoundary(t *testing.T) {
	var output bytes.Buffer
	cmd := NewCmd(factory.Static{}, genericiooptions.IOStreams{
		In: &bytes.Buffer{}, Out: &output, ErrOut: &output,
	})
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"status", "--help"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, `Show autoscaling evidence already reported on the InferenceService parent.
It does not query HPA, KEDA ScaledObject, Deployment, or InferenceReplica
objects, so "Reported" describes controller-reported evidence, not freshness.

Usage:
  autoscale status INFERENCESERVICE [flags]

Flags:
  -h, --help            help for status
  -o, --output string   Output format: table, json or yaml (default "table")
`, output.String())
}
