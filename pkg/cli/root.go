package cli

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"sigs.k8s.io/ome/pkg/cli/factory"
)

// NewRootCmd builds the kubectl-ome root command. Subcommands are registered
// here and nowhere else; adding a future command family means adding one
// AddCommand line.
func NewRootCmd(streams genericiooptions.IOStreams) *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)
	f := factory.New(configFlags)

	cmd := &cobra.Command{
		Use:   "ome",
		Short: "Inspect OME models, runtimes and inference services",
		Long: `kubectl-ome is the official OME CLI, invoked as a kubectl plugin:

  kubectl ome <command>

It provides model-centric visibility into OME resources: rich listings,
InferenceService readiness diagnosis, runtime-selection explanations and
component-aware log streaming.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	configFlags.AddFlags(cmd.PersistentFlags())

	// Command families. Keep alphabetical.
	// (get/status/runtime/logs are added by later PRs; version below.)
	cmd.AddCommand(newVersionPlaceholder(f, streams)) // replaced in Task 1.4

	return cmd
}

func newVersionPlaceholder(_ factory.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	return &cobra.Command{Use: "version", Short: "Print version", RunE: func(*cobra.Command, []string) error {
		_, err := streams.Out.Write([]byte("dev\n"))
		return err
	}}
}
