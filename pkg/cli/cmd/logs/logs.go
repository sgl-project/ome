package logs

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"sigs.k8s.io/ome/pkg/cli/factory"
	"sigs.k8s.io/ome/pkg/cli/paging"
	"sigs.k8s.io/ome/pkg/constants"
)

type Options struct {
	genericiooptions.IOStreams
	Name      string
	Component string
	Container string
	Follow    bool
	Tail      int64
	Since     time.Duration
}

func NewCmd(f factory.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &Options{IOStreams: streams, Tail: -1}
	cmd := &cobra.Command{
		Use:   "logs INFERENCESERVICE",
		Short: "Stream logs from the pods behind an InferenceService",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run(cmd.Context(), f)
		},
	}
	cmd.Flags().StringVarP(&o.Component, "component", "c", "", "Only this component: engine, decoder or router")
	cmd.Flags().StringVar(&o.Container, "container", "", "Container name (default: the OME main container, falling back to the pod's first container)")
	cmd.Flags().BoolVarP(&o.Follow, "follow", "f", false, "Stream new log lines as they arrive")
	cmd.Flags().Int64Var(&o.Tail, "tail", o.Tail, "Lines of recent log to show per pod (-1 for all)")
	cmd.Flags().DurationVar(&o.Since, "since", 0, "Only logs newer than this duration (e.g. 10m)")
	return cmd
}

func (o *Options) Validate() error {
	switch o.Component {
	case "", "engine", "decoder", "router":
		return nil
	default:
		return fmt.Errorf("invalid component %q (valid: engine, decoder, router)", o.Component)
	}
}

func (o *Options) Run(ctx context.Context, f factory.Factory) error {
	ns, _, err := f.Namespace()
	if err != nil {
		return err
	}
	kube, err := f.KubeClient()
	if err != nil {
		return err
	}
	selector := fmt.Sprintf("%s=%s", constants.InferenceServiceLabel, o.Name)
	if o.Component != "" {
		selector += fmt.Sprintf(",%s=%s", constants.OMEComponentLabel, o.Component)
	}
	podObjs, err := paging.ListAllPaged(ctx, func(pageOpts metav1.ListOptions) ([]runtime.Object, string, error) {
		pageOpts.LabelSelector = selector
		l, err := kube.CoreV1().Pods(ns).List(ctx, pageOpts)
		if err != nil {
			return nil, "", err
		}
		items := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			items = append(items, &l.Items[i])
		}
		return items, l.Continue, nil
	})
	if err != nil {
		return err
	}
	pods := make([]corev1.Pod, 0, len(podObjs))
	for _, obj := range podObjs {
		pods = append(pods, *obj.(*corev1.Pod))
	}
	if len(pods) == 0 {
		return fmt.Errorf("no pods found for InferenceService %q in namespace %q (selector %s)", o.Name, ns, selector)
	}
	opts := &corev1.PodLogOptions{Follow: o.Follow}
	if o.Tail >= 0 {
		opts.TailLines = &o.Tail
	}
	if o.Since > 0 {
		secs := int64(o.Since.Seconds())
		opts.SinceSeconds = &secs
	}
	var streams []namedStream
	for _, p := range pods {
		po := *opts
		po.Container = o.containerFor(&p)
		req := kube.CoreV1().Pods(ns).GetLogs(p.Name, &po)
		reader, err := req.Stream(ctx)
		if err != nil {
			// Don't leak the API-server log connections already opened for
			// earlier pods in this loop: multiplex() never gets to run its
			// deferred Close on them since we're bailing out before the call.
			for _, s := range streams {
				_ = s.Reader.Close()
			}
			return fmt.Errorf("streaming logs for pod %s: %w", p.Name, err)
		}
		prefix := ""
		if len(pods) > 1 {
			prefix = fmt.Sprintf("[%s/%s] ", p.Labels[constants.OMEComponentLabel], p.Name)
		}
		streams = append(streams, namedStream{Prefix: prefix, Reader: reader})
	}
	return multiplex(streams, o.Out)
}

// containerFor picks --container if set, else the OME main container when the
// pod has one, else the first container.
func (o *Options) containerFor(p *corev1.Pod) string {
	if o.Container != "" {
		return o.Container
	}
	for _, c := range p.Spec.Containers {
		if c.Name == constants.MainContainerName {
			return c.Name
		}
	}
	return "" // empty lets the API default to the only/first container
}
