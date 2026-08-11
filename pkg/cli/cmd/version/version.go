// Package version implements `kubectl ome version`.
package version

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"sigs.k8s.io/ome/pkg/cli/factory"
	"sigs.k8s.io/ome/pkg/version"
)

type Options struct {
	genericiooptions.IOStreams
	OMENamespace string
}

func NewCmd(f factory.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &Options{IOStreams: streams, OMENamespace: "ome"}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print kubectl-ome and OME operator versions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return o.Run(cmd.Context(), f)
		},
	}
	cmd.Flags().StringVar(&o.OMENamespace, "ome-namespace", o.OMENamespace,
		"Namespace where the OME control plane is installed")
	return cmd
}

// Run always reports the client version; the operator lookup is best-effort
// and degrades to "unknown" (never a non-zero exit).
func (o *Options) Run(ctx context.Context, f factory.Factory) error {
	fmt.Fprintf(o.Out, "Client Version: %s (commit %s)\n", version.GitVersion, version.GitCommit)
	fmt.Fprintf(o.Out, "Operator Version: %s\n", o.operatorVersion(ctx, f))
	return nil
}

func (o *Options) operatorVersion(ctx context.Context, f factory.Factory) string {
	kube, err := f.KubeClient()
	if err != nil {
		return fmt.Sprintf("unknown (%v)", err)
	}
	dep, err := kube.AppsV1().Deployments(o.OMENamespace).Get(ctx, "ome-controller-manager", metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("unknown (cannot read Deployment ome-controller-manager in namespace %q: %v)", o.OMENamespace, err)
	}
	containers := dep.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		return "unknown (manager Deployment has no containers)"
	}
	image := containers[0].Image
	if i := strings.LastIndex(image, ":"); i >= 0 && i < len(image)-1 {
		return image[i+1:]
	}
	return image
}
