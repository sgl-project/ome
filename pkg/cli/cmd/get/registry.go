// Package get implements `kubectl ome get`: a table-driven view over every
// OME resource. Each resource is one registry entry; adding a resource means
// adding an entry, never a new command.
package get

import (
	"context"
	"fmt"
	"sort"
	"strings"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/factory"
	"sigs.k8s.io/ome/pkg/cli/printers"
)

type listFunc func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error)

type getFunc func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error)

type column struct {
	Name    string
	Wide    bool // shown only with -o wide
	Extract func(obj runtime.Object) string
}

type entry struct {
	Canonical  string
	Aliases    []string
	Namespaced bool
	Columns    []column
	List       listFunc
	GetOne     getFunc
}

func resolve(resource string) (*entry, error) {
	needle := strings.ToLower(resource)
	for _, e := range registry {
		if e.Canonical == needle {
			return e, nil
		}
		for _, a := range e.Aliases {
			if a == needle {
				return e, nil
			}
		}
	}
	names := make([]string, 0, len(registry))
	for _, e := range registry {
		names = append(names, e.Canonical)
	}
	sort.Strings(names)
	return nil, fmt.Errorf("unknown resource %q (valid resources: %s)", resource, strings.Join(names, ", "))
}

// nameCol adapts a typed extractor (e.g. func(*v1beta1.InferenceService) string)
// to the runtime.Object-keyed column.Extract signature, so each entry's
// column table can be written against its concrete API type instead of
// runtime.Object.
func nameCol[T runtime.Object](extract func(T) string) func(runtime.Object) string {
	return func(o runtime.Object) string { return extract(o.(T)) }
}

// safeCol is nameCol's nil-safe sibling: a wrong-typed object returns "?"
// instead of panicking through a failed type assertion. The Task 2.3
// long-tail entries (acceleratorclasses, benchmarkjobs, finetunedweights,
// inferencereplicas, workloadclusters) use this instead of nameCol because
// TestLongTailColumnsWrongType requires every one of their columns to
// survive an object of the wrong resource kind.
func safeCol[T runtime.Object](extract func(T) string) func(runtime.Object) string {
	return func(o runtime.Object) string {
		t, ok := o.(T)
		if !ok {
			return "?"
		}
		return extract(t)
	}
}

// derefOrDash dereferences a *string for column rendering, substituting "-"
// for a nil pointer (mirrors baseModelColumns' local deref helper, promoted
// to package scope since finetunedweights needs it for two separate fields).
func derefOrDash(p *string) string {
	if p == nil {
		return "-"
	}
	return printers.OrDash(*p)
}

var isvcEntry = &entry{
	Canonical:  "inferenceservices",
	Aliases:    []string{"inferenceservice", "isvc", "isvcs"},
	Namespaced: true,
	Columns: []column{
		{Name: "NAME", Extract: nameCol(func(i *v1beta1.InferenceService) string { return i.Name })},
		{Name: "MODEL", Extract: nameCol(func(i *v1beta1.InferenceService) string {
			if i.Spec.Model == nil {
				return "-"
			}
			return i.Spec.Model.Name
		})},
		{Name: "RUNTIME", Extract: nameCol(func(i *v1beta1.InferenceService) string {
			if i.Spec.Runtime == nil {
				return "-" // auto-selected; status view shows the resolved one
			}
			return i.Spec.Runtime.Name
		})},
		{Name: "READY", Extract: nameCol(func(i *v1beta1.InferenceService) string {
			c := i.Status.GetCondition(apis.ConditionReady)
			if c == nil {
				return "Unknown"
			}
			return string(c.Status)
		})},
		{Name: "URL", Extract: nameCol(func(i *v1beta1.InferenceService) string {
			if i.Status.URL == nil {
				return "-"
			}
			return i.Status.URL.String()
		})},
		{Name: "AGE", Extract: nameCol(func(i *v1beta1.InferenceService) string {
			return printers.Age(i.CreationTimestamp)
		})},
	},
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		l, err := c.OmeV1beta1().InferenceServices(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			out = append(out, &l.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		return c.OmeV1beta1().InferenceServices(ns).Get(ctx, name, metav1.GetOptions{})
	},
}

// baseModelColumns builds the shared column set for BaseModel/ClusterBaseModel
// objects. withScope adds a SCOPE column distinguishing namespaced vs
// cluster-scoped entries; used only by the merged `models` view since the
// single-kind `basemodels`/`clusterbasemodels` views have a fixed scope.
func baseModelColumns(withScope bool) []column {
	cols := []column{
		{Name: "NAME", Extract: func(o runtime.Object) string {
			switch m := o.(type) {
			case *v1beta1.BaseModel:
				return m.Name
			case *v1beta1.ClusterBaseModel:
				return m.Name
			}
			return "?"
		}},
	}
	if withScope {
		cols = append(cols, column{Name: "SCOPE", Extract: func(o runtime.Object) string {
			switch o.(type) {
			case *v1beta1.ClusterBaseModel:
				return "Cluster"
			case *v1beta1.BaseModel:
				return "Namespaced"
			}
			return "?"
		}})
	}
	spec := func(o runtime.Object) *v1beta1.BaseModelSpec {
		switch m := o.(type) {
		case *v1beta1.BaseModel:
			return &m.Spec
		case *v1beta1.ClusterBaseModel:
			return &m.Spec
		}
		return nil
	}
	status := func(o runtime.Object) *v1beta1.ModelStatusSpec {
		switch m := o.(type) {
		case *v1beta1.BaseModel:
			return &m.Status
		case *v1beta1.ClusterBaseModel:
			return &m.Status
		}
		return nil
	}
	deref := func(p *string) string {
		if p == nil {
			return "-"
		}
		return *p
	}
	cols = append(cols,
		column{Name: "ARCH", Extract: func(o runtime.Object) string {
			s := spec(o)
			if s == nil {
				return "?"
			}
			return deref(s.ModelArchitecture)
		}},
		column{Name: "PARAMS", Extract: func(o runtime.Object) string {
			s := spec(o)
			if s == nil {
				return "?"
			}
			return deref(s.ModelParameterSize)
		}},
		column{Name: "FORMAT", Extract: func(o runtime.Object) string {
			s := spec(o)
			if s == nil {
				return "?"
			}
			return printers.OrDash(s.ModelFormat.Name)
		}},
		column{Name: "STATE", Extract: func(o runtime.Object) string {
			s := status(o)
			if s == nil {
				return "?"
			}
			return printers.OrDash(string(s.State))
		}},
		column{Name: "AGE", Extract: func(o runtime.Object) string {
			acc, ok := o.(metav1.Object)
			if !ok {
				return "?"
			}
			return printers.Age(metav1.Time{Time: acc.GetCreationTimestamp().Time})
		}},
	)
	return cols
}

// modelsEntry is the merged view: BaseModels from the target namespace (all
// namespaces with -A) plus ClusterBaseModels always. A single-name lookup
// resolves BaseModel first, falling back to ClusterBaseModel on NotFound —
// the same precedence the operator itself applies when resolving an
// InferenceService's model reference (see pkg/modelparser).
var modelsEntry = &entry{
	Canonical:  "models",
	Aliases:    []string{"model"},
	Namespaced: true, // namespace scopes the BaseModel half of the view
	Columns:    baseModelColumns(true),
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		var out []runtime.Object
		bms, err := c.OmeV1beta1().BaseModels(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for i := range bms.Items {
			out = append(out, &bms.Items[i])
		}
		cbms, err := c.OmeV1beta1().ClusterBaseModels().List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for i := range cbms.Items {
			out = append(out, &cbms.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		if bm, err := c.OmeV1beta1().BaseModels(ns).Get(ctx, name, metav1.GetOptions{}); err == nil {
			return bm, nil
		} else if !kerrors.IsNotFound(err) {
			return nil, err
		}
		return c.OmeV1beta1().ClusterBaseModels().Get(ctx, name, metav1.GetOptions{})
	},
}

var baseModelsEntry = &entry{
	Canonical:  "basemodels",
	Aliases:    []string{"basemodel", "bm"},
	Namespaced: true,
	Columns:    baseModelColumns(false),
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		l, err := c.OmeV1beta1().BaseModels(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			out = append(out, &l.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		return c.OmeV1beta1().BaseModels(ns).Get(ctx, name, metav1.GetOptions{})
	},
}

var clusterBaseModelsEntry = &entry{
	Canonical:  "clusterbasemodels",
	Aliases:    []string{"clusterbasemodel", "cbm"},
	Namespaced: false,
	Columns:    baseModelColumns(false),
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		l, err := c.OmeV1beta1().ClusterBaseModels().List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			out = append(out, &l.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		return c.OmeV1beta1().ClusterBaseModels().Get(ctx, name, metav1.GetOptions{})
	},
}

// runtimeColumns builds the shared column set for ServingRuntime/
// ClusterServingRuntime objects. withScope adds a SCOPE column; used only by
// the merged `runtimes` view, mirroring baseModelColumns.
func runtimeColumns(withScope bool) []column {
	spec := func(o runtime.Object) *v1beta1.ServingRuntimeSpec {
		switch r := o.(type) {
		case *v1beta1.ServingRuntime:
			return &r.Spec
		case *v1beta1.ClusterServingRuntime:
			return &r.Spec
		}
		return nil
	}
	cols := []column{
		{Name: "NAME", Extract: func(o runtime.Object) string {
			acc, ok := o.(metav1.Object)
			if !ok {
				return "?"
			}
			return acc.GetName()
		}},
	}
	if withScope {
		cols = append(cols, column{Name: "SCOPE", Extract: func(o runtime.Object) string {
			switch o.(type) {
			case *v1beta1.ClusterServingRuntime:
				return "Cluster"
			case *v1beta1.ServingRuntime:
				return "Namespaced"
			}
			return "?"
		}})
	}
	cols = append(cols,
		column{Name: "DISABLED", Extract: func(o runtime.Object) string {
			s := spec(o)
			if s == nil {
				return "?"
			}
			d := s.Disabled
			return fmt.Sprintf("%t", d != nil && *d)
		}},
		column{Name: "FORMATS", Extract: func(o runtime.Object) string {
			s := spec(o)
			if s == nil {
				return "?"
			}
			fmts := s.SupportedModelFormats
			names := make([]string, 0, len(fmts))
			for _, sf := range fmts {
				switch {
				case sf.Name != "":
					names = append(names, sf.Name)
				case sf.ModelFormat != nil && sf.ModelFormat.Name != "":
					names = append(names, sf.ModelFormat.Name)
				}
			}
			if len(names) > 3 {
				return strings.Join(names[:3], ",") + fmt.Sprintf(",+%d", len(names)-3)
			}
			return printers.OrDash(strings.Join(names, ","))
		}},
		column{Name: "AGE", Extract: func(o runtime.Object) string {
			acc, ok := o.(metav1.Object)
			if !ok {
				return "?"
			}
			return printers.Age(metav1.Time{Time: acc.GetCreationTimestamp().Time})
		}},
	)
	return cols
}

// runtimesEntry is the merged view: ServingRuntimes from the target
// namespace (all namespaces with -A) plus ClusterServingRuntimes always. A
// single-name lookup resolves ServingRuntime first, falling back to
// ClusterServingRuntime on NotFound — mirroring modelsEntry's precedence.
var runtimesEntry = &entry{
	Canonical:  "runtimes",
	Aliases:    []string{"runtime"},
	Namespaced: true, // namespace scopes the ServingRuntime half of the view
	Columns:    runtimeColumns(true),
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		var out []runtime.Object
		srts, err := c.OmeV1beta1().ServingRuntimes(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for i := range srts.Items {
			out = append(out, &srts.Items[i])
		}
		csrts, err := c.OmeV1beta1().ClusterServingRuntimes().List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for i := range csrts.Items {
			out = append(out, &csrts.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		if srt, err := c.OmeV1beta1().ServingRuntimes(ns).Get(ctx, name, metav1.GetOptions{}); err == nil {
			return srt, nil
		} else if !kerrors.IsNotFound(err) {
			return nil, err
		}
		return c.OmeV1beta1().ClusterServingRuntimes().Get(ctx, name, metav1.GetOptions{})
	},
}

var servingRuntimesEntry = &entry{
	Canonical:  "servingruntimes",
	Aliases:    []string{"servingruntime", "srt"},
	Namespaced: true,
	Columns:    runtimeColumns(false),
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		l, err := c.OmeV1beta1().ServingRuntimes(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			out = append(out, &l.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		return c.OmeV1beta1().ServingRuntimes(ns).Get(ctx, name, metav1.GetOptions{})
	},
}

var clusterServingRuntimesEntry = &entry{
	Canonical:  "clusterservingruntimes",
	Aliases:    []string{"clusterservingruntime", "csrt"},
	Namespaced: false,
	Columns:    runtimeColumns(false),
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		l, err := c.OmeV1beta1().ClusterServingRuntimes().List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			out = append(out, &l.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		return c.OmeV1beta1().ClusterServingRuntimes().Get(ctx, name, metav1.GetOptions{})
	},
}

var acceleratorClassesEntry = &entry{
	Canonical:  "acceleratorclasses",
	Aliases:    []string{"acceleratorclass", "ac"},
	Namespaced: false,
	Columns: []column{
		{Name: "NAME", Extract: safeCol(func(a *v1beta1.AcceleratorClass) string { return a.Name })},
		{Name: "VENDOR", Extract: safeCol(func(a *v1beta1.AcceleratorClass) string { return printers.OrDash(a.Spec.Vendor) })},
		{Name: "FAMILY", Extract: safeCol(func(a *v1beta1.AcceleratorClass) string { return printers.OrDash(a.Spec.Family) })},
		{Name: "AGE", Extract: safeCol(func(a *v1beta1.AcceleratorClass) string { return printers.Age(a.CreationTimestamp) })},
	},
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		l, err := c.OmeV1beta1().AcceleratorClasses().List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			out = append(out, &l.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		return c.OmeV1beta1().AcceleratorClasses().Get(ctx, name, metav1.GetOptions{})
	},
}

var benchmarkJobsEntry = &entry{
	Canonical:  "benchmarkjobs",
	Aliases:    []string{"benchmarkjob", "bj"},
	Namespaced: true,
	Columns: []column{
		{Name: "NAME", Extract: safeCol(func(b *v1beta1.BenchmarkJob) string { return b.Name })},
		{Name: "STATE", Extract: safeCol(func(b *v1beta1.BenchmarkJob) string { return printers.OrDash(b.Status.State) })},
		{Name: "AGE", Extract: safeCol(func(b *v1beta1.BenchmarkJob) string { return printers.Age(b.CreationTimestamp) })},
	},
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		l, err := c.OmeV1beta1().BenchmarkJobs(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			out = append(out, &l.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		return c.OmeV1beta1().BenchmarkJobs(ns).Get(ctx, name, metav1.GetOptions{})
	},
}

// fineTunedWeightsEntry is cluster-scoped: FineTunedWeight carries
// +genclient:nonNamespaced and +kubebuilder:resource:scope="Cluster" (see
// pkg/apis/ome/v1beta1/model.go), so the generated FineTunedWeights() getter
// takes no namespace argument -- unlike BenchmarkJobs/InferenceReplicas.
var fineTunedWeightsEntry = &entry{
	Canonical:  "finetunedweights",
	Aliases:    []string{"finetunedweight", "ftw"},
	Namespaced: false,
	Columns: []column{
		{Name: "NAME", Extract: safeCol(func(w *v1beta1.FineTunedWeight) string { return w.Name })},
		{Name: "TYPE", Extract: safeCol(func(w *v1beta1.FineTunedWeight) string { return derefOrDash(w.Spec.ModelType) })},
		{Name: "BASEMODEL", Extract: safeCol(func(w *v1beta1.FineTunedWeight) string { return derefOrDash(w.Spec.BaseModelRef.Name) })},
		{Name: "AGE", Extract: safeCol(func(w *v1beta1.FineTunedWeight) string { return printers.Age(w.CreationTimestamp) })},
	},
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		l, err := c.OmeV1beta1().FineTunedWeights().List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			out = append(out, &l.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		return c.OmeV1beta1().FineTunedWeights().Get(ctx, name, metav1.GetOptions{})
	},
}

var inferenceReplicasEntry = &entry{
	Canonical:  "inferencereplicas",
	Aliases:    []string{"inferencereplica", "ir"},
	Namespaced: true,
	Columns: []column{
		{Name: "NAME", Extract: safeCol(func(r *v1beta1.InferenceReplica) string { return r.Name })},
		{Name: "COMPONENT", Extract: safeCol(func(r *v1beta1.InferenceReplica) string { return printers.OrDash(string(r.Spec.Component)) })},
		{Name: "PARENT", Extract: safeCol(func(r *v1beta1.InferenceReplica) string { return printers.OrDash(r.Spec.ParentRef.Name) })},
		{Name: "REPLICAS", Extract: safeCol(func(r *v1beta1.InferenceReplica) string { return fmt.Sprintf("%d", r.Status.Replicas) })},
		{Name: "AGE", Extract: safeCol(func(r *v1beta1.InferenceReplica) string { return printers.Age(r.CreationTimestamp) })},
	},
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		l, err := c.OmeV1beta1().InferenceReplicas(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			out = append(out, &l.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		return c.OmeV1beta1().InferenceReplicas(ns).Get(ctx, name, metav1.GetOptions{})
	},
}

var workloadClustersEntry = &entry{
	Canonical:  "workloadclusters",
	Aliases:    []string{"workloadcluster", "wc"},
	Namespaced: false,
	Columns: []column{
		{Name: "NAME", Extract: safeCol(func(w *v1beta1.WorkloadCluster) string { return w.Name })},
		{Name: "READY", Extract: safeCol(func(w *v1beta1.WorkloadCluster) string {
			c := meta.FindStatusCondition(w.Status.Conditions, v1beta1.WorkloadClusterReady)
			if c == nil {
				return "Unknown"
			}
			return string(c.Status)
		})},
		{Name: "AGE", Extract: safeCol(func(w *v1beta1.WorkloadCluster) string { return printers.Age(w.CreationTimestamp) })},
	},
	List: func(ctx context.Context, f factory.Factory, ns string, opts metav1.ListOptions) ([]runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		l, err := c.OmeV1beta1().WorkloadClusters().List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out := make([]runtime.Object, 0, len(l.Items))
		for i := range l.Items {
			out = append(out, &l.Items[i])
		}
		return out, nil
	},
	GetOne: func(ctx context.Context, f factory.Factory, ns, name string) (runtime.Object, error) {
		c, err := f.OMEClient()
		if err != nil {
			return nil, err
		}
		return c.OmeV1beta1().WorkloadClusters().Get(ctx, name, metav1.GetOptions{})
	},
}

// registry lists every resource `kubectl ome get` knows how to render. Adding
// a resource means adding an entry here, never a new command.
var registry = []*entry{
	isvcEntry,
	modelsEntry,
	baseModelsEntry, clusterBaseModelsEntry,
	runtimesEntry, servingRuntimesEntry, clusterServingRuntimesEntry,
	acceleratorClassesEntry, benchmarkJobsEntry,
	fineTunedWeightsEntry, inferenceReplicasEntry, workloadClustersEntry,
}
