package cli

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"sigs.k8s.io/ome/pkg/cli/cmd/get"
	"sigs.k8s.io/ome/pkg/cli/cmd/version"
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
	// (status/runtime/logs are added by later PRs; get/version below.)
	cmd.AddCommand(get.NewCmd(f, streams))
	cmd.AddCommand(version.NewCmd(f, streams))

	return cmd
}
