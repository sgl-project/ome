package status

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"sigs.k8s.io/ome/pkg/cli/factory"
)

func NewCmd(f factory.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status INFERENCESERVICE",
		Short: "Show the full readiness story of an InferenceService",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, _, err := f.Namespace()
			if err != nil {
				return err
			}
			r, err := gather(cmd.Context(), f, ns, args[0])
			if err != nil {
				return err
			}
			return render(r, streams.Out)
		},
	}
	return cmd
}
