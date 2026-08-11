package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/apierror"
	"sigs.k8s.io/ome/pkg/cli/factory"
	"sigs.k8s.io/ome/pkg/cli/printers"
	"sigs.k8s.io/ome/pkg/runtimeselector"
)

type explainOptions struct {
	genericiooptions.IOStreams
	Model string
	ISVC  string
}

func newExplainCmd(f factory.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &explainOptions{IOStreams: streams}
	cmd := &cobra.Command{
		Use:   "explain (--model NAME | --isvc NAME)",
		Short: "Explain which serving runtimes match a model and why",
		Long: `Runs the operator's own runtime-selection engine (pkg/runtimeselector)
against the live cluster and prints every namespace-scoped and cluster-scoped
serving runtime it considered, whether each is compatible with the model, and
why -- including runtimes that were rejected.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run(cmd.Context(), f)
		},
	}
	cmd.Flags().StringVar(&o.Model, "model", "", "Explain runtime selection for this BaseModel/ClusterBaseModel")
	cmd.Flags().StringVar(&o.ISVC, "isvc", "", "Explain runtime selection for this InferenceService's model")
	return cmd
}

func (o *explainOptions) Validate() error {
	if (o.Model == "") == (o.ISVC == "") {
		return fmt.Errorf("exactly one of --model or --isvc is required")
	}
	return nil
}

func (o *explainOptions) Run(ctx context.Context, f factory.Factory) error {
	ns, _, err := f.Namespace()
	if err != nil {
		return err
	}
	modelSpec, isvc, err := o.resolveTarget(ctx, f, ns)
	if err != nil {
		return err
	}
	if o.ISVC != "" && isvc.Spec.Runtime != nil {
		fmt.Fprintf(o.ErrOut, "Note: InferenceService %q pins spec.runtime=%q; the table below shows what automatic selection would choose, which may differ from what is currently deployed.\n", o.ISVC, isvc.Spec.Runtime.Name)
	}

	ctrl, err := f.RuntimeClient()
	if err != nil {
		return err
	}
	selector := runtimeselector.New(ctrl)

	// GetCompatibleRuntimes is the operator's actual auto-select ranking:
	// every entry it returns already passed compatibility, auto-select
	// eligibility and scoring, so these are unconditionally "Yes".
	matches, err := selector.GetCompatibleRuntimes(ctx, modelSpec, isvc, ns)
	if err != nil {
		return apierror.Friendly(err)
	}

	// GetCompatibleRuntimes silently drops everything that isn't a match --
	// disabled runtimes, format/size mismatches, and runtimes with no
	// autoSelect-enabled format never appear in its result at all. The OEP
	// requires rejected runtimes to show up too, with a reason, so list every
	// runtime the selector could have considered and re-validate whichever
	// ones didn't make the cut.
	candidates, err := listCandidateRuntimes(ctx, ctrl, ns)
	if err != nil {
		return apierror.Friendly(err)
	}
	if len(matches) == 0 && len(candidates) == 0 {
		fmt.Fprintf(o.ErrOut, "No serving runtimes found in namespace %q or at cluster scope.\n", ns)
		return nil
	}

	matched := make(map[runtimeKey]bool, len(matches))
	table := printers.Table{Headers: []string{"RUNTIME", "SCOPE", "COMPATIBLE", "PRIORITY", "WEIGHT", "REASON"}}
	for _, m := range matches {
		matched[runtimeKey{m.Name, m.IsCluster}] = true
		table.Rows = append(table.Rows, []string{
			m.Name, scopeLabel(m.IsCluster), "Yes",
			fmt.Sprintf("%d", m.MatchDetails.Priority),
			fmt.Sprintf("%d", m.MatchDetails.Weight),
			printers.OrDash(strings.Join(m.MatchDetails.Reasons, "; ")),
		})
	}
	for _, c := range candidates {
		if matched[runtimeKey(c)] {
			continue
		}
		reason := "supports the model but has no supportedModelFormats[].autoSelect=true entry, so automatic selection skips it (pin it explicitly via spec.runtime.name instead)"
		if err := selector.ValidateRuntime(ctx, c.name, modelSpec, isvc); err != nil {
			reason = rejectionReason(err)
		}
		table.Rows = append(table.Rows, []string{c.name, scopeLabel(c.isCluster), "No", "-", "-", reason})
	}

	return table.Write(o.Out)
}

// resolveTarget loads the model spec (and, for --isvc, the service) using the
// operator's resolution order: namespaced BaseModel first, then
// ClusterBaseModel.
func (o *explainOptions) resolveTarget(ctx context.Context, f factory.Factory, ns string) (*v1beta1.BaseModelSpec, *v1beta1.InferenceService, error) {
	ome, err := f.OMEClient()
	if err != nil {
		return nil, nil, err
	}
	modelName := o.Model
	// Synthetic isvc gives the selector namespace context on the --model path.
	isvc := &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Namespace: ns}}
	if o.ISVC != "" {
		got, err := ome.OmeV1beta1().InferenceServices(ns).Get(ctx, o.ISVC, metav1.GetOptions{})
		if err != nil {
			return nil, nil, apierror.Friendly(err)
		}
		if got.Spec.Model == nil {
			return nil, nil, fmt.Errorf("InferenceService %q has no spec.model; pass --model instead", o.ISVC)
		}
		isvc = got
		modelName = got.Spec.Model.Name
	}
	if bm, err := ome.OmeV1beta1().BaseModels(ns).Get(ctx, modelName, metav1.GetOptions{}); err == nil {
		return &bm.Spec, isvc, nil
	} else if !kerrors.IsNotFound(err) {
		return nil, nil, apierror.Friendly(err)
	}
	cbm, err := ome.OmeV1beta1().ClusterBaseModels().Get(ctx, modelName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, apierror.Friendly(err)
	}
	return &cbm.Spec, isvc, nil
}

// runtimeKey identifies a runtime by name and scope; namespace-scoped and
// cluster-scoped runtimes sharing a name are distinct candidates, matching
// how pkg/runtimeselector itself never deduplicates across scope.
type runtimeKey struct {
	name      string
	isCluster bool
}

type runtimeCandidate struct {
	name      string
	isCluster bool
}

// listCandidateRuntimes enumerates every runtime GetCompatibleRuntimes could
// have considered: namespace-scoped runtimes in ns, plus every cluster-scoped
// runtime. This mirrors pkg/runtimeselector's own fetch (namespace runtimes
// always precede cluster ones) so rejected runtimes can be re-validated for a
// reason instead of silently vanishing from the table.
func listCandidateRuntimes(ctx context.Context, c ctrlclient.Client, ns string) ([]runtimeCandidate, error) {
	var out []runtimeCandidate

	nsRuntimes := &v1beta1.ServingRuntimeList{}
	if err := c.List(ctx, nsRuntimes, ctrlclient.InNamespace(ns)); err != nil {
		return nil, err
	}
	for _, rt := range nsRuntimes.Items {
		out = append(out, runtimeCandidate{name: rt.Name})
	}

	clusterRuntimes := &v1beta1.ClusterServingRuntimeList{}
	if err := c.List(ctx, clusterRuntimes); err != nil {
		return nil, err
	}
	for _, rt := range clusterRuntimes.Items {
		out = append(out, runtimeCandidate{name: rt.Name, isCluster: true})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].isCluster != out[j].isCluster {
			return !out[i].isCluster // namespace-scoped first, matching GetCompatibleRuntimes' ordering
		}
		return out[i].name < out[j].name
	})
	return out, nil
}

// rejectionReason extracts a human-readable, non-redundant reason from the
// errors ValidateRuntime returns. Unrecognized error types fall back to
// err.Error() so the table always has something to show.
func rejectionReason(err error) string {
	switch e := err.(type) {
	case *runtimeselector.RuntimeCompatibilityError:
		return e.Reason
	case *runtimeselector.RuntimeDisabledError:
		return "runtime is disabled"
	default:
		return err.Error()
	}
}

func scopeLabel(isCluster bool) string {
	if isCluster {
		return "Cluster"
	}
	return "Namespaced"
}
